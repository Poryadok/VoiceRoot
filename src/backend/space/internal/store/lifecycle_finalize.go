package store

import (
	"context"
	"crypto/sha256"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/spacecore"
)

// AccountHasher is supplied only by the Space workload's purpose-specific key
// provider. Neither staff identity nor a public RPC can select the HMAC input.
type AccountHasher interface {
	HashAccount(context.Context, uuid.UUID) ([]byte, string, error)
}

// FinalizeLifecyclePurge reloads all accepted remote evidence under the Space
// lock. Local deletion, the minimal tombstone and event visibility share one
// transaction; the caller's wall clock or aggregate cannot authorize deletion.
func (s *SpaceStore) FinalizeLifecyclePurge(ctx context.Context, spaceID uuid.UUID, hasher AccountHasher) (*spacecore.LifecycleAggregate, error) {
	if hasher == nil {
		return nil, ErrLifecycleEvidenceInvalid
	}
	return s.withLockedLifecycle(ctx, spaceID, func(tx pgx.Tx, aggregate *spacecore.LifecycleAggregate) (bool, error) {
		if aggregate.Phase() == spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_PURGED {
			return false, nil
		}
		if err := aggregate.RecordLocalPurgeCompleted(); err != nil {
			return false, err
		}
		snapshot := aggregate.Snapshot()
		operation, err := loadLifecycleOperation(ctx, tx, uuid.MustParse(snapshot.DeletionOperationID))
		if err != nil {
			return false, err
		}
		if err := operation.validate(); err != nil {
			return false, err
		}
		if operation.method != "DELETE" || operation.spaceID != spaceID || len(operation.authReceiptBytes) == 0 {
			return false, ErrLifecycleEvidenceInvalid
		}
		// The authenticated deletion actor is the owner recorded at admission.
		ownerHMAC, keyVersion, err := hasher.HashAccount(ctx, operation.accountID)
		if err != nil {
			return false, err
		}
		if len(ownerHMAC) != sha256.Size || keyVersion == "" {
			return false, ErrLifecycleEvidenceInvalid
		}
		var now time.Time
		if err := tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&now); err != nil {
			return false, err
		}
		if _, err := aggregate.CompletePurge(now.UTC()); err != nil {
			return false, err
		}
		snapshot = aggregate.Snapshot()
		if err := persistLifecycleSnapshot(ctx, tx, snapshot); err != nil {
			return false, err
		}
		_, err = tx.Exec(ctx, `INSERT INTO space_deletion_tombstones(space_id,owner_account_hmac,actor_account_hmac,key_version,reason,scheduled_at,purge_after,purge_decided_at,purged_at,retain_until)
   VALUES($1,$2,$2,$3,'OWNER_REQUESTED',$4::timestamptz AT TIME ZONE 'UTC',$5::timestamptz AT TIME ZONE 'UTC',$6::timestamptz AT TIME ZONE 'UTC',$7::timestamptz AT TIME ZONE 'UTC',($7::timestamptz AT TIME ZONE 'UTC')+interval '365 days')`, spaceID, ownerHMAC, keyVersion, snapshot.ScheduledAt, snapshot.PurgeAfter, snapshot.PurgeDecidedAt, snapshot.DeletedEvent.OccurredAt)
		if err != nil {
			return false, err
		}
		// Retain the minimum decision timestamp in the independent delivery row;
		// an undelivered event must survive aggregate/tombstone expiry.
		if _, err = tx.Exec(ctx, `UPDATE space_lifecycle_outbox SET purge_decided_at=$2 WHERE space_id=$1 AND event_type='space.deleted' AND state='READY'`, spaceID, snapshot.PurgeDecidedAt); err != nil {
			return false, err
		}
		// No-FK coordinator/evidence rows have their separately bounded retention.
		// Ordinary local content and ordinary audit are removed now.
		for _, query := range []string{
			`SELECT set_config('voice.audit_delete_mode','space_purge',true)`,
			`DELETE FROM ownership_outbox WHERE space_id=$1`,
			`DELETE FROM ownership_journal WHERE space_id=$1`,
			`DELETE FROM community_bootstrap_operations WHERE space_id=$1`,
			`DELETE FROM spaces WHERE id=$1`,
		} {
			var err error
			if query[0] == 'S' {
				_, err = tx.Exec(ctx, query)
			} else {
				_, err = tx.Exec(ctx, query, spaceID)
			}
			if err != nil {
				return false, err
			}
		}
		return false, nil // The terminal snapshot was saved before the guarded delete.
	})
}
