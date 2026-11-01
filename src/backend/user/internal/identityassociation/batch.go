package identityassociation

import (
	"bytes"
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"math"
	"sort"
	"time"
)

type CaptureStatus string

const (
	CaptureAssociated CaptureStatus = "ASSOCIATED"
	CaptureTerminal   CaptureStatus = "TERMINAL_PROFILE"
	CaptureIncomplete CaptureStatus = "INCOMPLETE"
)

type CaptureObservation struct {
	Status    CaptureStatus
	AccountID uuid.UUID
	Fact      Fact
}
type SourceCut struct {
	UserSourceGeneration  uuid.UUID
	UserAuthorityRevision uint64
	HistoryGeneration     uuid.UUID
	ThroughRevision       uint64
	KeysetRevision        uint64
	Digest                [32]byte
	RetainedFloor         uint64
}

// PhysicalSourceIdentity is a controller-held PostgreSQL binding. It is not
// proof of currentness or exclusive custody: physical clones may retain it.
type PhysicalSourceIdentity struct {
	SystemIdentifier string
	Database         string
	DatabaseOID      uint32
}
type CaptureBatchResult struct {
	Cut          SourceCut
	Observations map[uuid.UUID]CaptureObservation
}

type preparedCaptureBatch struct {
	ids      []uuid.UUID
	lookups  map[uuid.UUID]PreparedProfileLookup
	bindings map[uuid.UUID]PreparedSelectors
}
type aliasKey struct {
	version uint32
	value   [32]byte
}
type retainedCaptureRow struct {
	account                   uuid.UUID
	association               *uuid.UUID
	baseline, revision        uint64
	deleted, frozen, inactive bool
}

// CaptureProfiles returns only after its complete local observation commits.
// Preparation is outside I; all authoritative reads are separate commands after
// I, and SQL roundtrips do not grow with the number of requested profiles.
func (s *Store) CaptureProfiles(ctx context.Context, ids []uuid.UUID) (CaptureBatchResult, error) {
	return s.captureProfiles(ctx, ids, uuid.Nil, uuid.Nil, nil)
}

// CaptureProfilesForSource binds the grant's independent source generations to
// the actual User transaction before any identity fact can be written. The pin
// is a comparison, not proof of custody or currentness on its own.
func (s *Store) CaptureProfilesForSource(ctx context.Context, ids []uuid.UUID, history, source uuid.UUID, digest [32]byte) (CaptureBatchResult, error) {
	if history == uuid.Nil || source == uuid.Nil || history == source || s == nil || s.generation != history {
		return CaptureBatchResult{}, ErrIncomplete
	}
	actual, err := s.keys.CustodyDigest()
	if err != nil || digest != actual {
		return CaptureBatchResult{}, ErrIncomplete
	}
	return s.captureProfiles(ctx, ids, history, source, nil)
}

// CaptureProfilesForPhysicalSource verifies the controller-held source on both
// the lookup session and the authoritative transaction. It additionally needs
// independently enforced endpoint and restore custody.
func (s *Store) CaptureProfilesForPhysicalSource(ctx context.Context, ids []uuid.UUID, history, source uuid.UUID, digest [32]byte, physical PhysicalSourceIdentity) (CaptureBatchResult, error) {
	if history == uuid.Nil || source == uuid.Nil || history == source || s == nil || s.generation != history || physical.SystemIdentifier == "" || physical.Database == "" || physical.DatabaseOID == 0 {
		return CaptureBatchResult{}, ErrIncomplete
	}
	actual, err := s.keys.CustodyDigest()
	if err != nil || digest != actual {
		return CaptureBatchResult{}, ErrIncomplete
	}
	return s.captureProfiles(ctx, ids, history, source, &physical)
}

type physicalQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func checkPhysicalSource(ctx context.Context, query physicalQuerier, expected *PhysicalSourceIdentity) error {
	if expected == nil {
		return nil
	}
	var actual PhysicalSourceIdentity
	if query.QueryRow(ctx, `SELECT system_identifier::text,current_database(),(SELECT oid FROM pg_database WHERE datname=current_database()) FROM pg_control_system()`).Scan(&actual.SystemIdentifier, &actual.Database, &actual.DatabaseOID) != nil || actual != *expected {
		return ErrIncomplete
	}
	return nil
}

// ValidateForActivationTx checks an existing guarded identity source while its
// identity mutex is held. It neither provisions data nor proves currentness,
// writer deployment or exclusive administrative custody.
func (s *Store) ValidateForActivationTx(ctx context.Context, tx *IdentityTx, history, source uuid.UUID, digest [32]byte, physical PhysicalSourceIdentity) error {
	if ctx == nil || s == nil || s.keys == nil || tx == nil || tx.SQL() == nil || history == uuid.Nil || source == uuid.Nil || history == source || s.generation != history || tx.generation != history || physical.SystemIdentifier == "" || physical.Database == "" || physical.DatabaseOID == 0 {
		return ErrIncomplete
	}
	actualDigest, err := s.keys.CustodyDigest()
	if err != nil || actualDigest != digest || checkPhysicalSource(ctx, tx.SQL(), &physical) != nil || s.checkKeyset(ctx, tx) != nil {
		return ErrIncomplete
	}
	var actualSource uuid.UUID
	var ready bool
	if tx.SQL().QueryRow(ctx, `SELECT user_source_generation,guards_active AND baseline_cut>0 AND unresolved_erase_issued=unresolved_erase_covered FROM identity_meta WHERE singleton`).Scan(&actualSource, &ready) != nil || actualSource != source || !ready {
		return ErrIncomplete
	}
	return nil
}

// ActivateQualifiedGuardsTx is the transaction-local cutover primitive. Its
// caller must first fence every writer and establish independently attested
// source, writer deployment, Auth and restore custody. It does not commit.
func (s *Store) ActivateQualifiedGuardsTx(ctx context.Context, tx *IdentityTx, history, source uuid.UUID, digest [32]byte, physical PhysicalSourceIdentity) error {
	if ctx == nil || s == nil || s.keys == nil || tx == nil || tx.SQL() == nil || history == uuid.Nil || source == uuid.Nil || history == source || s.generation != history || tx.generation != history || physical.SystemIdentifier == "" || physical.Database == "" || physical.DatabaseOID == 0 {
		return ErrIncomplete
	}
	actualDigest, err := s.keys.CustodyDigest()
	if err != nil || actualDigest != digest || checkPhysicalSource(ctx, tx.SQL(), &physical) != nil || s.checkKeyset(ctx, tx) != nil {
		return ErrIncomplete
	}
	var actualSource uuid.UUID
	var active, ready bool
	var next uint64
	if tx.SQL().QueryRow(ctx, `SELECT user_source_generation,guards_active,NOT guards_active AND baseline_cut=0 AND unresolved_erase_issued=unresolved_erase_covered,history_head+1 FROM identity_meta WHERE singleton`).Scan(&actualSource, &active, &ready, &next) != nil || actualSource != source || active || !ready || next == 0 {
		return ErrIncomplete
	}
	if prepareIdentitySQL(ctx, tx, nil, nil, nil, "ACTIVATE") != nil {
		return ErrIncomplete
	}
	if _, err = tx.SQL().Exec(ctx, `UPDATE profiles SET identity_baseline_cut=$1 WHERE identity_association_id IS NULL`, next); err != nil {
		return ErrIncomplete
	}
	if _, err = tx.SQL().Exec(ctx, `UPDATE identity_meta SET guards_active=true,baseline_cut=$1 WHERE singleton`, next); err != nil {
		return ErrIncomplete
	}
	if _, err = tx.SQL().Exec(ctx, `SELECT identity_append_event(NULL,NULL,'GUARDS_ACTIVATED',NULL,NULL,NULL)`); err != nil {
		return ErrIncomplete
	}
	return s.ValidateForActivationTx(ctx, tx, history, source, digest, physical)
}

func (s *Store) captureProfiles(ctx context.Context, ids []uuid.UUID, history, source uuid.UUID, physical *PhysicalSourceIdentity) (CaptureBatchResult, error) {
	if s == nil {
		return CaptureBatchResult{}, ErrIncomplete
	}
	bounded, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	prepared, err := s.prepareCaptureBatch(bounded, ids, physical)
	if err != nil {
		return CaptureBatchResult{}, err
	}
	tx, err := s.Begin(bounded)
	if err != nil {
		return CaptureBatchResult{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if checkPhysicalSource(bounded, tx.SQL(), physical) != nil {
		return CaptureBatchResult{}, ErrIncomplete
	}
	if history != uuid.Nil {
		var actual uuid.UUID
		if err = tx.SQL().QueryRow(bounded, `SELECT user_source_generation FROM identity_meta WHERE singleton`).Scan(&actual); err != nil || actual != source {
			return CaptureBatchResult{}, ErrIncomplete
		}
	}
	out, err := s.captureBatchTx(bounded, tx, prepared)
	if err != nil {
		return CaptureBatchResult{}, err
	}
	if err = tx.Commit(bounded); err != nil {
		return CaptureBatchResult{}, err
	}
	return out, nil
}

func (s *Store) prepareCaptureBatch(ctx context.Context, ids []uuid.UUID, physical *PhysicalSourceIdentity) (preparedCaptureBatch, error) {
	p := preparedCaptureBatch{lookups: map[uuid.UUID]PreparedProfileLookup{}, bindings: map[uuid.UUID]PreparedSelectors{}}
	for _, id := range ids {
		if id == uuid.Nil {
			return p, ErrIncomplete
		}
		if _, exists := p.lookups[id]; exists {
			continue
		}
		lookup, err := s.keys.PrepareProfileLookup(id)
		if err != nil {
			return p, err
		}
		p.lookups[id] = lookup
		p.ids = append(p.ids, id)
	}
	sort.Slice(p.ids, func(i, j int) bool { return bytes.Compare(p.ids[i][:], p.ids[j][:]) < 0 })
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return p, err
	}
	defer conn.Release()
	if checkPhysicalSource(ctx, conn, physical) != nil {
		return p, ErrIncomplete
	}
	rows, err := conn.Query(ctx, `SELECT id,account_id FROM profiles WHERE id=ANY($1)`, p.ids)
	if err != nil {
		return p, err
	}
	defer rows.Close()
	for rows.Next() {
		var profile, account uuid.UUID
		if err = rows.Scan(&profile, &account); err != nil {
			return p, err
		}
		binding, err := s.keys.PrepareSelectors(profile, account)
		if err != nil {
			return p, err
		}
		p.bindings[profile] = binding
	}
	return p, rows.Err()
}

func (s *Store) captureBatchTx(ctx context.Context, tx *IdentityTx, p preparedCaptureBatch) (CaptureBatchResult, error) {
	result := CaptureBatchResult{Observations: map[uuid.UUID]CaptureObservation{}}
	if err := s.checkKeyset(ctx, tx); err != nil {
		return result, err
	}
	var active bool
	var baseline, head uint64
	var capturedAt time.Time
	if err := tx.SQL().QueryRow(ctx, `SELECT guards_active,baseline_cut,history_head,clock_timestamp() FROM identity_meta WHERE singleton`).Scan(&active, &baseline, &head, &capturedAt); err != nil {
		return result, err
	}
	if !active || baseline == 0 {
		return result, ErrIncomplete
	}
	retained := map[uuid.UUID]retainedCaptureRow{}
	rows, err := tx.SQL().Query(ctx, `SELECT p.id,p.account_id,p.identity_association_id,p.identity_baseline_cut,p.sdk_eligibility_revision,p.deleted_at IS NOT NULL,p.frozen_at IS NOT NULL,EXISTS(SELECT 1 FROM user_account_lifecycle l WHERE l.account_id=p.account_id AND l.state='ACCOUNT_INACTIVE') FROM profiles p WHERE p.id=ANY($1) ORDER BY p.id FOR UPDATE OF p`, p.ids)
	if err != nil {
		return result, err
	}
	for rows.Next() {
		var id uuid.UUID
		var row retainedCaptureRow
		if err = rows.Scan(&id, &row.account, &row.association, &row.baseline, &row.revision, &row.deleted, &row.frozen, &row.inactive); err != nil {
			rows.Close()
			return result, err
		}
		binding, ok := p.bindings[id]
		if !ok || binding.accountID != row.account {
			rows.Close()
			return result, ErrConflict
		}
		retained[id] = row
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return result, err
	}
	var versions []int64
	var selectors [][]byte
	profileByAlias := map[aliasKey]uuid.UUID{}
	for _, id := range p.ids {
		result.Observations[id] = CaptureObservation{Status: CaptureIncomplete}
		for _, a := range p.lookups[id].aliases {
			key := aliasKey{a.version, a.profile}
			if previous, exists := profileByAlias[key]; exists && previous != id {
				return result, ErrConflict
			}
			profileByAlias[key] = id
			versions = append(versions, int64(a.version))
			selectors = append(selectors, a.profile[:])
		}
	}
	found := map[aliasKey]uuid.UUID{}
	reserved := map[aliasKey]bool{}
	associationSet := map[uuid.UUID]bool{}
	rows, err = tx.SQL().Query(ctx, `SELECT a.key_version,a.selector,a.association_id FROM association_selector_aliases a JOIN unnest($1::bigint[],$2::bytea[]) requested(version,selector) ON a.key_version=requested.version AND a.selector=requested.selector WHERE a.kind='PROFILE'`, versions, selectors)
	if err != nil {
		return result, err
	}
	for rows.Next() {
		var version uint32
		var selector []byte
		var association uuid.UUID
		if err = rows.Scan(&version, &selector, &association); err != nil {
			rows.Close()
			return result, err
		}
		if len(selector) != 32 {
			rows.Close()
			return result, ErrIncomplete
		}
		var key aliasKey
		key.version = version
		copy(key.value[:], selector)
		found[key] = association
		associationSet[association] = true
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return result, err
	}
	rows, err = tx.SQL().Query(ctx, `SELECT r.key_version,r.profile_selector FROM identity_profile_reservations r JOIN unnest($1::bigint[],$2::bytea[]) requested(version,selector) ON r.key_version=requested.version AND r.profile_selector=requested.selector`, versions, selectors)
	if err != nil {
		return result, err
	}
	for rows.Next() {
		var version uint32
		var selector []byte
		if err = rows.Scan(&version, &selector); err != nil {
			rows.Close()
			return result, err
		}
		if len(selector) != 32 {
			rows.Close()
			return result, ErrIncomplete
		}
		var key aliasKey
		key.version = version
		copy(key.value[:], selector)
		reserved[key] = true
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return result, err
	}
	associationIDs := make([]uuid.UUID, 0, len(associationSet))
	for id := range associationSet {
		associationIDs = append(associationIDs, id)
	}
	accountsByAssociation := map[uuid.UUID]map[uint32][32]byte{}
	rows, err = tx.SQL().Query(ctx, `SELECT association_id,key_version,selector FROM association_selector_aliases WHERE association_id=ANY($1) AND kind='ACCOUNT'`, associationIDs)
	if err != nil {
		return result, err
	}
	for rows.Next() {
		var id uuid.UUID
		var version uint32
		var selector []byte
		if err = rows.Scan(&id, &version, &selector); err != nil {
			rows.Close()
			return result, err
		}
		if len(selector) != 32 {
			rows.Close()
			return result, ErrIncomplete
		}
		var value [32]byte
		copy(value[:], selector)
		if accountsByAssociation[id] == nil {
			accountsByAssociation[id] = map[uint32][32]byte{}
		}
		accountsByAssociation[id][version] = value
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return result, err
	}
	var accountVersions []int64
	var accountSelectors [][]byte
	for _, id := range p.ids {
		if binding, ok := p.bindings[id]; ok {
			for _, a := range binding.aliases {
				accountVersions = append(accountVersions, int64(a.version))
				accountSelectors = append(accountSelectors, a.account[:])
			}
		}
	}
	terminalAccounts := map[aliasKey]bool{}
	rows, err = tx.SQL().Query(ctx, `SELECT a.key_version,a.account_selector FROM identity_account_parent_aliases a JOIN identity_parent_fences f USING(owner_handle) JOIN unnest($1::bigint[],$2::bytea[]) requested(version,selector) ON a.key_version=requested.version AND a.account_selector=requested.selector WHERE f.state='TERMINAL'`, accountVersions, accountSelectors)
	if err != nil {
		return result, err
	}
	for rows.Next() {
		var version uint32
		var selector []byte
		if err = rows.Scan(&version, &selector); err != nil {
			rows.Close()
			return result, err
		}
		if len(selector) != 32 {
			rows.Close()
			return result, ErrIncomplete
		}
		var key aliasKey
		key.version = version
		copy(key.value[:], selector)
		terminalAccounts[key] = true
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return result, err
	}
	facts := map[uuid.UUID]Fact{}
	rows, err = tx.SQL().Query(ctx, `SELECT `+factColumns+` FROM identity_associations WHERE association_id=ANY($1)`, associationIDs)
	if err != nil {
		return result, err
	}
	for rows.Next() {
		fact, scanErr := scanFact(rows, tx.generation)
		if scanErr != nil {
			rows.Close()
			return result, scanErr
		}
		facts[fact.AssociationID] = fact
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return result, err
	}
	var newIDs []uuid.UUID
	for _, id := range p.ids {
		lookup := p.lookups[id]
		var association uuid.UUID
		complete := true
		anyAlias := false
		for _, a := range lookup.aliases {
			matched, exists := found[aliasKey{a.version, a.profile}]
			if !exists {
				complete = false
				continue
			}
			anyAlias = true
			if association != uuid.Nil && association != matched {
				complete = false
			}
			association = matched
		}
		row, present := retained[id]
		if !present {
			if complete && association != uuid.Nil {
				if fact, ok := facts[association]; ok && fact.Terminal != nil {
					result.Observations[id] = CaptureObservation{Status: CaptureTerminal, Fact: fact}
				}
			}
			continue
		}
		binding := p.bindings[id]
		parentTerminal := false
		for _, a := range binding.aliases {
			parentTerminal = parentTerminal || terminalAccounts[aliasKey{a.version, a.account}]
		}
		if row.association != nil {
			if !complete || association != *row.association {
				continue
			}
			fact, ok := facts[association]
			if !ok {
				continue
			}
			coherent := true
			for _, a := range binding.aliases {
				actual, exists := accountsByAssociation[association][a.version]
				if !exists || actual != a.account {
					coherent = false
				}
			}
			if !coherent || fact.Deleted != row.deleted || fact.Frozen != row.frozen || fact.AccountInactive != row.inactive {
				continue
			}
			fact.ProfileRevision = row.revision
			if fact.Terminal != nil {
				result.Observations[id] = CaptureObservation{Status: CaptureTerminal, AccountID: row.account, Fact: fact}
			} else if !parentTerminal && fact.ParentState != ParentTerminal {
				result.Observations[id] = CaptureObservation{Status: CaptureAssociated, AccountID: row.account, Fact: fact}
			}
			continue
		}
		if parentTerminal || anyAlias || row.baseline != baseline || row.revision == 0 {
			continue
		}
		used := false
		for _, a := range lookup.aliases {
			used = used || reserved[aliasKey{a.version, a.profile}]
		}
		if used {
			continue
		}
		newIDs = append(newIDs, id)
	}
	if head > math.MaxInt64-uint64(len(newIDs)) {
		return result, ErrIncomplete
	}
	if len(newIDs) > 0 {
		var associationRows, reservationRows, aliasRows, preparationRows, batchRows [][]any
		for i, id := range newIDs {
			row := retained[id]
			binding := p.bindings[id]
			association, receipt := uuid.New(), uuid.New()
			revision := head + uint64(i) + 1
			digest := captureReceipt(association, receipt, capturedAt, binding)
			associationRows = append(associationRows, []any{association, int64(revision), int64(revision), string(OriginRetainedNow), capturedAt, receipt, digest[:], row.deleted, row.frozen, row.inactive, int64(row.revision)})
			for _, a := range binding.aliases {
				reservationRows = append(reservationRows, []any{int64(a.version), a.profile[:]})
				aliasRows = append(aliasRows, []any{association, int64(a.version), "PROFILE", a.profile[:]}, []any{association, int64(a.version), "ACCOUNT", a.account[:]})
			}
			preparationRows = append(preparationRows, []any{tx.txid, id, row.account, association, "RETAINED_NOW"})
			batchRows = append(batchRows, []any{association, int64(revision)})
			fact := Fact{AssociationID: association, Revision: revision, CaptureRevision: revision, Origin: OriginRetainedNow, CapturedAt: capturedAt, ReceiptID: receipt, ReceiptDigest: digest, ProfileRevision: row.revision, Deleted: row.deleted, Frozen: row.frozen, AccountInactive: row.inactive}
			result.Observations[id] = CaptureObservation{Status: CaptureAssociated, AccountID: row.account, Fact: fact}
		}
		if _, err = tx.SQL().CopyFrom(ctx, pgx.Identifier{"public", "identity_associations"}, []string{"association_id", "association_revision", "capture_revision", "capture_origin", "captured_at", "receipt_id", "receipt_digest", "deleted", "frozen", "account_inactive", "profile_revision"}, pgx.CopyFromRows(associationRows)); err != nil {
			return result, err
		}
		if _, err = tx.SQL().CopyFrom(ctx, pgx.Identifier{"public", "identity_profile_reservations"}, []string{"key_version", "profile_selector"}, pgx.CopyFromRows(reservationRows)); err != nil {
			return result, err
		}
		if _, err = tx.SQL().CopyFrom(ctx, pgx.Identifier{"public", "association_selector_aliases"}, []string{"association_id", "key_version", "kind", "selector"}, pgx.CopyFromRows(aliasRows)); err != nil {
			return result, err
		}
		if _, err = tx.SQL().Exec(ctx, `CREATE TEMP TABLE IF NOT EXISTS user_identity_preparation(transaction_id bigint NOT NULL,profile_id uuid,account_id uuid,association_id uuid,kind text NOT NULL) ON COMMIT DROP; CREATE TEMP TABLE user_identity_capture_batch(association_id uuid PRIMARY KEY,expected_revision bigint NOT NULL) ON COMMIT DROP`); err != nil {
			return result, err
		}
		if _, err = tx.SQL().CopyFrom(ctx, pgx.Identifier{"pg_temp", "user_identity_preparation"}, []string{"transaction_id", "profile_id", "account_id", "association_id", "kind"}, pgx.CopyFromRows(preparationRows)); err != nil {
			return result, err
		}
		if _, err = tx.SQL().CopyFrom(ctx, pgx.Identifier{"pg_temp", "user_identity_capture_batch"}, []string{"association_id", "expected_revision"}, pgx.CopyFromRows(batchRows)); err != nil {
			return result, err
		}
		if _, err = tx.SQL().Exec(ctx, `SELECT identity_append_capture_batch()`); err != nil {
			return result, err
		}
		updated, updateErr := tx.SQL().Query(ctx, `UPDATE profiles p SET identity_association_id=prepared.association_id FROM pg_temp.user_identity_preparation prepared JOIN pg_temp.user_identity_capture_batch batch USING(association_id) WHERE prepared.transaction_id=txid_current() AND prepared.kind='RETAINED_NOW' AND p.id=prepared.profile_id AND p.account_id=prepared.account_id AND p.identity_association_id IS NULL RETURNING p.id,p.sdk_eligibility_revision`)
		if updateErr != nil {
			return result, updateErr
		}
		updatedCount := 0
		for updated.Next() {
			var id uuid.UUID
			var revision uint64
			if err = updated.Scan(&id, &revision); err != nil {
				updated.Close()
				return result, err
			}
			// The reference UPDATE also runs the existing SDK row-revision
			// trigger. Return its committed current revision, while preserving
			// the immutable original association capture receipt and cut.
			observation := result.Observations[id]
			observation.Fact.ProfileRevision = revision
			result.Observations[id] = observation
			updatedCount++
		}
		err = updated.Err()
		updated.Close()
		if err != nil {
			return result, err
		}
		if updatedCount != len(newIDs) {
			return result, ErrConflict
		}
	}
	var digest []byte
	err = tx.SQL().QueryRow(ctx, `SELECT m.user_source_generation,a.revision,m.history_generation,m.history_head,m.keyset_revision,m.head_digest,m.retained_floor FROM identity_meta m CROSS JOIN user_authority_revision a WHERE m.singleton AND a.singleton`).Scan(&result.Cut.UserSourceGeneration, &result.Cut.UserAuthorityRevision, &result.Cut.HistoryGeneration, &result.Cut.ThroughRevision, &result.Cut.KeysetRevision, &digest, &result.Cut.RetainedFloor)
	if err != nil {
		return result, err
	}
	if len(digest) != 32 {
		return result, ErrIncomplete
	}
	copy(result.Cut.Digest[:], digest)
	return result, nil
}
