package identityassociation

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// AttachParentTx enriches the same retained lifetime. It never changes the
// immutable capture origin, receipt or raw identity, and never calls Auth.
func (s *Store) AttachParentTx(ctx context.Context, tx *IdentityTx, p PreparedSelectors, expectedRevision uint64, proof VerifiedParentProof) (Fact, error) {
	if s == nil || expectedRevision == 0 {
		return Fact{}, ErrIncomplete
	}
	if err := s.checkPreparation(ctx, tx, p); err != nil {
		return Fact{}, err
	}
	var account uuid.UUID
	var association *uuid.UUID
	var now time.Time
	err := tx.SQL().QueryRow(ctx, `SELECT account_id,identity_association_id,clock_timestamp() FROM profiles WHERE id=$1 FOR UPDATE`, p.profileID).Scan(&account, &association, &now)
	if errors.Is(err, pgx.ErrNoRows) {
		return Fact{}, ErrUnknownProfile
	}
	if err != nil {
		return Fact{}, err
	}
	if account != p.accountID {
		return Fact{}, ErrConflict
	}
	if association == nil {
		return Fact{}, ErrIncomplete
	}
	if !proof.validAt(now) || proof.observation.AccountID != account {
		return Fact{}, ErrIncomplete
	}
	if err = verifyCaptureAliases(ctx, tx, p, *association); err != nil {
		return Fact{}, err
	}
	current, err := s.readFact(ctx, tx, *association)
	if err != nil {
		return Fact{}, err
	}
	if current.Revision != expectedRevision {
		return Fact{}, ErrConflict
	}
	parent := proof.observation
	if current.ParentHandle != "" && current.ParentHandle != parent.OwnerHandle {
		return Fact{}, ErrConflict
	}
	if err = applyParentFence(ctx, tx, p, proof, now); err != nil {
		return Fact{}, err
	}
	var next uint64
	if err = tx.SQL().QueryRow(ctx, `SELECT history_head+1 FROM identity_meta WHERE singleton`).Scan(&next); err != nil {
		return Fact{}, err
	}
	cutJSON, err := json.Marshal(parent.Cut)
	if err != nil {
		return Fact{}, err
	}
	// A terminal Auth observation is positive owner evidence, distinct from raw
	// archive/freeze. Its normalized cycle/deadline/cut is preserved verbatim.
	var terminalJSON []byte
	if parent.State == ParentTerminal && current.Terminal == nil {
		terminalJSON, err = json.Marshal(newTerminalRecord(tx, *association, next, now, parent))
		if err != nil {
			return Fact{}, err
		}
	}
	_, err = tx.SQL().Exec(ctx, `UPDATE identity_associations SET association_revision=$1,parent_handle=$2,parent_lifecycle_revision=$3,parent_state=$4,parent_cut=$5,terminal_fact=COALESCE(terminal_fact,$6::jsonb) WHERE association_id=$7 AND association_revision=$8`, next, parent.OwnerHandle, parent.LifecycleRevision, parent.State, cutJSON, terminalJSON, *association, expectedRevision)
	if err != nil {
		return Fact{}, err
	}
	if _, err = tx.SQL().Exec(ctx, `SELECT identity_append_event($1,$2,'PARENT_ATTACHED',$3,$4,$5)`, *association, next, current.Deleted, current.Frozen, current.AccountInactive); err != nil {
		return Fact{}, err
	}
	return s.readFact(ctx, tx, *association)
}
