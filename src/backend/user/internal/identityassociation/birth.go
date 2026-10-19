package identityassociation

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// parentFact deliberately omits AccountID. The exact account binding is carried
// only by versioned User-owned selectors; the proof's raw input stays in memory.
type parentFact struct {
	Handle                          string
	Revision                        uint64
	State                           ParentState
	Eligibility                     ParentEligibility
	Disposition                     ParentDisposition
	Cycle                           uuid.UUID
	EraseAt, ObservedAt, ValidUntil time.Time
	Cut                             ParentCut
}

func minimizedParent(p ParentObservation) parentFact {
	return parentFact{p.OwnerHandle, p.LifecycleRevision, p.State, p.Eligibility, p.Disposition, p.CycleID, p.EraseAt, p.ObservedAt, p.ValidUntil, p.Cut}
}

func knownAccountTerminal(ctx context.Context, tx *IdentityTx, p PreparedSelectors) (bool, error) {
	var knownHandle string
	for _, a := range p.aliases {
		var handle string
		var state ParentState
		err := tx.SQL().QueryRow(ctx, `SELECT alias.owner_handle,fence.state FROM identity_account_parent_aliases alias JOIN identity_parent_fences fence USING(owner_handle) WHERE alias.key_version=$1 AND alias.account_selector=$2`, a.version, a.account[:]).Scan(&handle, &state)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return false, err
		}
		if knownHandle != "" && knownHandle != handle {
			return false, ErrConflict
		}
		knownHandle = handle
		if state == ParentTerminal {
			return true, nil
		}
	}
	return false, nil
}

// applyParentFence must be followed by its caller's association/terminal event
// in this same transaction. It performs no remote operation.
func applyParentFence(ctx context.Context, tx *IdentityTx, p PreparedSelectors, proof VerifiedParentProof, now time.Time) error {
	if !proof.validAt(now) || proof.observation.AccountID != p.accountID {
		return ErrIncomplete
	}
	incoming := proof.observation
	var previous parentFact
	var encoded []byte
	err := tx.SQL().QueryRow(ctx, `SELECT owner_fact FROM identity_parent_fences WHERE owner_handle=$1`, incoming.OwnerHandle).Scan(&encoded)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if err == nil {
		if json.Unmarshal(encoded, &previous) != nil {
			return ErrIncomplete
		}
		cut := incoming.Cut
		if previous.Cut.Owner != cut.Owner || previous.Cut.Stream != cut.Stream || previous.Cut.HistoryGeneration != cut.HistoryGeneration {
			return ErrIncomplete
		}
		if incoming.LifecycleRevision < previous.Revision || cut.ThroughRevision < previous.Cut.ThroughRevision || cut.KeysetRevision < previous.Cut.KeysetRevision {
			return ErrConflict
		}
		if cut.ThroughRevision == previous.Cut.ThroughRevision && cut.Digest != previous.Cut.Digest {
			return ErrConflict
		}
		if previous.State == ParentTerminal && incoming.State != ParentTerminal {
			return ErrConflict
		}
		if incoming.LifecycleRevision == previous.Revision && (incoming.State != previous.State || incoming.CycleID != previous.Cycle || !incoming.EraseAt.Equal(previous.EraseAt) || incoming.Eligibility != previous.Eligibility || incoming.Disposition != previous.Disposition) {
			return ErrConflict
		}
	}
	cutJSON, err := json.Marshal(incoming.Cut)
	if err != nil {
		return err
	}
	factJSON, err := json.Marshal(minimizedParent(incoming))
	if err != nil {
		return err
	}
	_, err = tx.SQL().Exec(ctx, `INSERT INTO identity_parent_fences(owner_handle,lifecycle_revision,state,owner_cut,owner_fact) VALUES($1,$2,$3,$4,$5) ON CONFLICT(owner_handle) DO UPDATE SET lifecycle_revision=excluded.lifecycle_revision,state=excluded.state,owner_cut=excluded.owner_cut,owner_fact=excluded.owner_fact`, incoming.OwnerHandle, incoming.LifecycleRevision, incoming.State, cutJSON, factJSON)
	if err != nil {
		return err
	}
	for _, a := range p.aliases {
		var handle string
		err = tx.SQL().QueryRow(ctx, `SELECT owner_handle FROM identity_account_parent_aliases WHERE key_version=$1 AND account_selector=$2`, a.version, a.account[:]).Scan(&handle)
		if err == nil {
			if handle != incoming.OwnerHandle {
				return ErrConflict
			}
			continue
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if _, err = tx.SQL().Exec(ctx, `INSERT INTO identity_account_parent_aliases VALUES($1,$2,$3)`, a.version, a.account[:], incoming.OwnerHandle); err != nil {
			return err
		}
	}
	return nil
}

// CreateBirthTx registers a fresh lifetime before the ordinary profile insert.
// The deferred database constraint requires that profile to commit atomically.
func (s *Store) CreateBirthTx(ctx context.Context, tx *IdentityTx, p PreparedSelectors, proof VerifiedParentProof) (Fact, error) {
	if s == nil {
		return Fact{}, ErrIncomplete
	}
	if err := s.checkPreparation(ctx, tx, p); err != nil {
		return Fact{}, err
	}
	var now time.Time
	var active bool
	var next uint64
	if err := tx.SQL().QueryRow(ctx, `SELECT clock_timestamp(),guards_active,history_head+1 FROM identity_meta WHERE singleton`).Scan(&now, &active, &next); err != nil {
		return Fact{}, err
	}
	if !active || !proof.validForBirth(now) || proof.observation.AccountID != p.accountID {
		return Fact{}, ErrIncomplete
	}
	var exists bool
	if err := tx.SQL().QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM profiles WHERE id=$1)`, p.profileID).Scan(&exists); err != nil {
		return Fact{}, err
	}
	if exists {
		return Fact{}, ErrConflict
	}
	for _, a := range p.aliases {
		if err := tx.SQL().QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM identity_profile_reservations WHERE key_version=$1 AND profile_selector=$2)`, a.version, a.profile[:]).Scan(&exists); err != nil {
			return Fact{}, err
		}
		if exists {
			return Fact{}, ErrConflict
		}
	}
	if err := applyParentFence(ctx, tx, p, proof, now); err != nil {
		return Fact{}, err
	}
	id, receipt := uuid.New(), uuid.New()
	// Include the actual BIRTH origin and immutable authenticated parent fact in
	// the receipt commitment, without persisting raw account/profile identifiers.
	type receiptAlias struct {
		Version          uint32
		Account, Profile [32]byte
	}
	aliases := make([]receiptAlias, 0, len(p.aliases))
	for _, a := range p.aliases {
		aliases = append(aliases, receiptAlias{a.version, a.account, a.profile})
	}
	receiptBody, _ := json.Marshal(struct {
		ID, Receipt uuid.UUID
		Origin      Origin
		Cut         time.Time
		Parent      parentFact
		Aliases     []receiptAlias
	}{id, receipt, OriginBirth, now, minimizedParent(proof.observation), aliases})
	digest := sha256.Sum256(append([]byte("voice.user.identity-association.receipt.v1\x00"), receiptBody...))
	cutJSON, _ := json.Marshal(proof.observation.Cut)
	_, err := tx.SQL().Exec(ctx, `INSERT INTO identity_associations(association_id,association_revision,capture_revision,capture_origin,captured_at,receipt_id,receipt_digest,parent_handle,parent_lifecycle_revision,parent_state,parent_cut,profile_revision) VALUES($1,$2,$2,'BIRTH',$3,$4,$5,$6,$7,$8,$9,1)`, id, next, now, receipt, digest[:], proof.observation.OwnerHandle, proof.observation.LifecycleRevision, proof.observation.State, cutJSON)
	if err != nil {
		return Fact{}, err
	}
	for _, a := range p.aliases {
		if _, err = tx.SQL().Exec(ctx, `INSERT INTO identity_profile_reservations VALUES($1,$2)`, a.version, a.profile[:]); err != nil {
			return Fact{}, err
		}
		if _, err = tx.SQL().Exec(ctx, `INSERT INTO association_selector_aliases VALUES($1,$2,'PROFILE',$3),($1,$2,'ACCOUNT',$4)`, id, a.version, a.profile[:], a.account[:]); err != nil {
			return Fact{}, err
		}
	}
	if _, err = tx.SQL().Exec(ctx, `SELECT identity_append_event($1,$2,'BIRTH',false,false,false)`, id, next); err != nil {
		return Fact{}, err
	}
	if err = prepareIdentitySQL(ctx, tx, p.profileID, p.accountID, id, "BIRTH"); err != nil {
		return Fact{}, err
	}
	return s.readFact(ctx, tx, id)
}
