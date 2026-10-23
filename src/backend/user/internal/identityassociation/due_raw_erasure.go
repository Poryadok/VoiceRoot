package identityassociation

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// PrepareDueRawEraseTx prepares only the exact current row for the caller's
// existing due cleanup. It needs no User HMAC key material and performs no remote
// call. Without proven spent-ID coverage the DELETE issues an identity-free gap;
// this helper never returns a historical association or terminal receipt.
func PrepareDueRawEraseTx(ctx context.Context, tx *IdentityTx, profile uuid.UUID, proof VerifiedParentProof) error {
	if tx == nil || tx.SQL() == nil || profile == uuid.Nil || tx.generation == uuid.Nil {
		return ErrIncomplete
	}
	var account uuid.UUID
	var association *uuid.UUID
	var now time.Time
	err := tx.SQL().QueryRow(ctx, `SELECT account_id,identity_association_id FROM profiles WHERE id=$1 FOR UPDATE`, profile).Scan(&account, &association)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrUnknownProfile
	}
	if err != nil {
		return err
	}
	// A projection in SELECT ... FOR UPDATE can be evaluated before LockRows
	// waits. Sample DB time in a separate command after the row lock returns.
	if err = tx.SQL().QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&now); err != nil {
		return err
	}
	p := proof.observation
	if !proof.validAt(now) || p.AccountID != account || p.State != ParentTerminal || p.Eligibility != ParentIneligible || p.Disposition != ParentIrreversible {
		return ErrIncomplete
	}
	if association != nil {
		var handle *string
		var revision *uint64
		var encoded []byte
		if err = tx.SQL().QueryRow(ctx, `SELECT parent_handle,parent_lifecycle_revision,parent_cut FROM identity_associations WHERE association_id=$1`, *association).Scan(&handle, &revision, &encoded); err != nil {
			return err
		}
		if handle != nil {
			var cut ParentCut
			if revision == nil || *handle != p.OwnerHandle || *revision > p.LifecycleRevision || json.Unmarshal(encoded, &cut) != nil || !dueParentCutCompatible(cut, p.Cut) {
				return ErrConflict
			}
		}
	}
	var previous []byte
	err = tx.SQL().QueryRow(ctx, `SELECT owner_fact FROM identity_parent_fences WHERE owner_handle=$1`, p.OwnerHandle).Scan(&previous)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if err == nil {
		var fact parentFact
		if json.Unmarshal(previous, &fact) != nil || fact.Handle != p.OwnerHandle || p.LifecycleRevision < fact.Revision || !dueParentCutCompatible(fact.Cut, p.Cut) ||
			(p.LifecycleRevision == fact.Revision && (fact.State != p.State || fact.Cycle != p.CycleID || !fact.EraseAt.Equal(p.EraseAt) || fact.Eligibility != p.Eligibility || fact.Disposition != p.Disposition)) {
			return ErrConflict
		}
	}
	cut, err := json.Marshal(p.Cut)
	if err != nil {
		return err
	}
	_, err = tx.SQL().Exec(ctx, `CREATE TEMP TABLE IF NOT EXISTS user_identity_due_erasure(
	 transaction_id bigint NOT NULL,profile_id uuid NOT NULL,account_id uuid NOT NULL,association_id uuid,
	 owner_handle text NOT NULL,lifecycle_revision bigint NOT NULL,cycle_id uuid NOT NULL,
	 erase_at timestamptz NOT NULL,observed_at timestamptz NOT NULL,valid_until timestamptz NOT NULL,parent_cut jsonb NOT NULL,
	 PRIMARY KEY(transaction_id,profile_id)) ON COMMIT DROP`)
	if err != nil {
		return err
	}
	_, err = tx.SQL().Exec(ctx, `INSERT INTO pg_temp.user_identity_due_erasure VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, tx.txid, profile, account, association, p.OwnerHandle, p.LifecycleRevision, p.CycleID, p.EraseAt, p.ObservedAt, p.ValidUntil, cut)
	return err
}

func dueParentCutCompatible(previous, incoming ParentCut) bool {
	return previous.Owner == incoming.Owner && previous.Stream == incoming.Stream && previous.HistoryGeneration == incoming.HistoryGeneration &&
		incoming.ThroughRevision >= previous.ThroughRevision && incoming.KeysetRevision >= previous.KeysetRevision &&
		(incoming.ThroughRevision != previous.ThroughRevision || incoming.Digest == previous.Digest)
}
