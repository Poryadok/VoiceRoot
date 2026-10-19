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

func (s *Store) checkPreparation(ctx context.Context, tx *IdentityTx, p PreparedSelectors) error {
	if !p.valid() || p.snapshot.Revision != s.keys.snapshot.Revision || p.snapshot.ActiveVersion != s.keys.snapshot.ActiveVersion || len(p.snapshot.Versions) != len(s.keys.snapshot.Versions) {
		return ErrIncomplete
	}
	for version, fingerprint := range s.keys.snapshot.Versions {
		if actual, ok := p.snapshot.Versions[version]; !ok || actual != fingerprint {
			return ErrIncomplete
		}
	}
	return s.checkKeyset(ctx, tx)
}

func (s *Store) readFact(ctx context.Context, tx *IdentityTx, association uuid.UUID) (Fact, error) {
	var f Fact
	var digest, terminalJSON []byte
	err := tx.SQL().QueryRow(ctx, `SELECT association_id,association_revision,capture_revision,capture_origin,captured_at,receipt_id,receipt_digest,COALESCE(parent_handle,''),COALESCE(parent_lifecycle_revision,0),COALESCE(parent_state,''),profile_revision,deleted,frozen,account_inactive,terminal_fact FROM identity_associations WHERE association_id=$1`, association).Scan(&f.AssociationID, &f.Revision, &f.CaptureRevision, &f.Origin, &f.CapturedAt, &f.ReceiptID, &digest, &f.ParentHandle, &f.ParentRevision, &f.ParentState, &f.ProfileRevision, &f.Deleted, &f.Frozen, &f.AccountInactive, &terminalJSON)
	if err != nil {
		return Fact{}, err
	}
	if len(digest) != 32 {
		return Fact{}, ErrIncomplete
	}
	copy(f.ReceiptDigest[:], digest)
	if len(terminalJSON) > 0 {
		var record terminalRecord
		if json.Unmarshal(terminalJSON, &record) != nil || !record.valid(f.AssociationID, tx.generation, f.Revision) {
			return Fact{}, ErrIncomplete
		}
		f.Terminal = &record.Evidence
	}
	return f, nil
}

// The receipt commits only opaque owner facts and purpose-owned selectors.
// Raw profile/account IDs and Space request/cohort data are never persisted in it.
func captureReceipt(association, receipt uuid.UUID, cut time.Time, p PreparedSelectors) [32]byte {
	type alias struct {
		Version          uint32
		Account, Profile [32]byte
	}
	body := struct {
		Association, Receipt uuid.UUID
		Origin               Origin
		Cut                  time.Time
		Aliases              []alias
	}{Association: association, Receipt: receipt, Origin: OriginRetainedNow, Cut: cut}
	for _, a := range p.aliases {
		body.Aliases = append(body.Aliases, alias{a.version, a.account, a.profile})
	}
	encoded, _ := json.Marshal(body)
	return sha256.Sum256(append([]byte("voice.user.identity-association.receipt.v1\x00"), encoded...))
}

func verifyCaptureAliases(ctx context.Context, tx *IdentityTx, p PreparedSelectors, association uuid.UUID) error {
	for _, a := range p.aliases {
		var actual uuid.UUID
		var account []byte
		err := tx.SQL().QueryRow(ctx, `SELECT p.association_id,a.selector FROM association_selector_aliases p JOIN association_selector_aliases a ON a.association_id=p.association_id AND a.key_version=p.key_version AND a.kind='ACCOUNT' WHERE p.key_version=$1 AND p.kind='PROFILE' AND p.selector=$2`, a.version, a.profile[:]).Scan(&actual, &account)
		if err != nil {
			return ErrIncomplete
		}
		if actual != association || len(account) != 32 || string(account) != string(a.account[:]) {
			return ErrConflict
		}
	}
	return nil
}

// CaptureRetainedTx reads the exact retained row only after the caller acquired
// I. NULL Auth parent is truthful retained-NOW evidence, not birth/history proof.
func (s *Store) CaptureRetainedTx(ctx context.Context, tx *IdentityTx, p PreparedSelectors) (Fact, error) {
	if s == nil {
		return Fact{}, ErrIncomplete
	}
	if err := s.checkPreparation(ctx, tx, p); err != nil {
		return Fact{}, err
	}
	var active bool
	var baseline, next uint64
	if err := tx.SQL().QueryRow(ctx, `SELECT guards_active,baseline_cut,history_head+1 FROM identity_meta WHERE singleton`).Scan(&active, &baseline, &next); err != nil {
		return Fact{}, err
	}
	if !active || baseline == 0 {
		return Fact{}, ErrIncomplete
	}
	terminal, err := knownAccountTerminal(ctx, tx, p)
	if err != nil {
		return Fact{}, err
	}
	if terminal {
		return Fact{}, ErrIncomplete
	}
	var account uuid.UUID
	var association *uuid.UUID
	var rowBaseline, profileRevision uint64
	var deleted, frozen bool
	err = tx.SQL().QueryRow(ctx, `SELECT account_id,identity_association_id,identity_baseline_cut,sdk_eligibility_revision,deleted_at IS NOT NULL,frozen_at IS NOT NULL FROM profiles WHERE id=$1 FOR UPDATE`, p.profileID).Scan(&account, &association, &rowBaseline, &profileRevision, &deleted, &frozen)
	if errors.Is(err, pgx.ErrNoRows) {
		return Fact{}, ErrUnknownProfile
	}
	if err != nil {
		return Fact{}, err
	}
	if account != p.accountID {
		return Fact{}, ErrConflict
	}
	if profileRevision == 0 {
		return Fact{}, ErrIncomplete
	}
	if association != nil {
		if err = verifyCaptureAliases(ctx, tx, p, *association); err != nil {
			return Fact{}, err
		}
		fact, err := s.readFact(ctx, tx, *association)
		if err != nil {
			return Fact{}, err
		}
		if fact.ParentState == ParentTerminal {
			return Fact{}, ErrIncomplete
		}
		return fact, nil
	}
	if rowBaseline != baseline {
		return Fact{}, ErrIncomplete
	}
	var inactive bool
	var capturedAt time.Time
	if err = tx.SQL().QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM user_account_lifecycle WHERE account_id=$1 AND state='ACCOUNT_INACTIVE'),clock_timestamp()`, account).Scan(&inactive, &capturedAt); err != nil {
		return Fact{}, err
	}
	// A lone used-profile reservation is evidence of nonreuse, never a current
	// association or terminal certificate. It cannot be adopted as a fresh row.
	for _, a := range p.aliases {
		var used bool
		if err = tx.SQL().QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM identity_profile_reservations WHERE key_version=$1 AND profile_selector=$2)`, a.version, a.profile[:]).Scan(&used); err != nil {
			return Fact{}, err
		}
		if used {
			return Fact{}, ErrConflict
		}
	}
	id, receipt := uuid.New(), uuid.New()
	digest := captureReceipt(id, receipt, capturedAt, p)
	_, err = tx.SQL().Exec(ctx, `INSERT INTO identity_associations(association_id,association_revision,capture_revision,capture_origin,captured_at,receipt_id,receipt_digest,deleted,frozen,account_inactive,profile_revision) VALUES($1,$2,$2,'RETAINED_NOW',$3,$4,$5,$6,$7,$8,$9)`, id, next, capturedAt, receipt, digest[:], deleted, frozen, inactive, profileRevision)
	if err != nil {
		return Fact{}, err
	}
	for _, a := range p.aliases {
		if _, err = tx.SQL().Exec(ctx, `INSERT INTO identity_profile_reservations(key_version,profile_selector) VALUES($1,$2)`, a.version, a.profile[:]); err != nil {
			return Fact{}, err
		}
		if _, err = tx.SQL().Exec(ctx, `INSERT INTO association_selector_aliases(association_id,key_version,kind,selector) VALUES($1,$2,'PROFILE',$3),($1,$2,'ACCOUNT',$4)`, id, a.version, a.profile[:], a.account[:]); err != nil {
			return Fact{}, err
		}
	}
	if _, err = tx.SQL().Exec(ctx, `SELECT identity_append_event($1,$2,'CAPTURE',$3,$4,$5)`, id, next, deleted, frozen, inactive); err != nil {
		return Fact{}, err
	}
	if err = prepareIdentitySQL(ctx, tx, p.profileID, p.accountID, id, "RETAINED_NOW"); err != nil {
		return Fact{}, err
	}
	tag, err := tx.SQL().Exec(ctx, `UPDATE profiles SET identity_association_id=$1 WHERE id=$2 AND account_id=$3 AND identity_association_id IS NULL`, id, p.profileID, p.accountID)
	if err != nil {
		return Fact{}, err
	}
	if tag.RowsAffected() != 1 {
		return Fact{}, ErrConflict
	}
	return s.readFact(ctx, tx, id)
}
