package store

import (
	"context"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// sealChatCreationPurge runs under the lifecycle Space transaction lock after
// all authenticated participant drain evidence is accepted. Its fixed deadline
// belongs to aggregate PURGED, independently of Chat's own local expiry clock.
func sealChatCreationPurge(ctx context.Context, tx pgx.Tx, spaceID, operationID uuid.UUID, generation uint64, purgedAt time.Time) error {
	if spaceID == uuid.Nil || operationID == uuid.Nil || generation == 0 || generation > math.MaxInt64 || purgedAt.IsZero() {
		return ErrCompositeCorrupt
	}
	command, err := tx.Exec(ctx, `INSERT INTO space_permanent_purge_fences(space_id,deletion_operation_id,purge_generation,purged_at) VALUES($1,$2,$3,$4)
 ON CONFLICT(space_id) DO UPDATE SET space_id=EXCLUDED.space_id WHERE space_permanent_purge_fences.deletion_operation_id=EXCLUDED.deletion_operation_id AND space_permanent_purge_fences.purge_generation=EXCLUDED.purge_generation AND space_permanent_purge_fences.purged_at=EXCLUDED.purged_at`, spaceID, operationID, generation, purgedAt)
	if err != nil {
		return err
	}
	if command.RowsAffected() != 1 {
		return ErrCompositeConflict
	}
	var mismatches bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM space_chat_creation_fences WHERE space_id=$1 AND deletion_operation_id IS NOT NULL AND (deletion_operation_id<>$2 OR purge_generation<>$3 OR purge_completed_at<>$4))`, spaceID, operationID, generation, purgedAt).Scan(&mismatches); err != nil {
		return err
	}
	if mismatches {
		return ErrCompositeConflict
	}
	_, err = tx.Exec(ctx, `UPDATE space_chat_creation_fences SET deletion_operation_id=$2,purge_generation=$3,purge_completed_at=$4,full_evidence_expires_at=$4::timestamptz+interval '30 days' WHERE space_id=$1 AND deletion_operation_id IS NULL`, spaceID, operationID, generation, purgedAt)
	return err
}
