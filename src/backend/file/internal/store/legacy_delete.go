package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"voice/backend/file/internal/r2file"
)

var ErrFileRetained = errors.New("file has surviving references or aliases")

// DeleteLegacyFile prepares an irreversible durable intent before any storage
// call. A failed response may leave a pending intent for the ordinary GC worker.
// expiredAt selects expiry eligibility; nil requests uploader deletion.
func (s *FilesStore) DeleteLegacyFile(ctx context.Context, id uuid.UUID, expiredAt *time.Time, deleter r2file.ObjectDeleter) (bool, error) {
	blob, eligible, err := s.prepareLegacyDeletion(ctx, id, expiredAt)
	if err != nil || !eligible {
		return false, err
	}
	// Even a missing deleter leaves the accepted intent terminal and retryable.
	if deleter == nil {
		return false, fmt.Errorf("file reference GC object deleter not configured")
	}
	candidate, claimed, err := s.claimReferenceGC(ctx, blob)
	if err != nil {
		return false, err
	}
	if claimed {
		_, err = s.deleteClaimedReferenceGC(ctx, deleter, candidate)
		if err != nil {
			_, _ = s.Pool.Exec(ctx, `UPDATE file_blobs SET next_attempt_at=clock_timestamp() WHERE blob_id=$1 AND state='GC_PENDING' AND gc_operation_id=$2 AND gc_attempt=$3`, candidate.id, candidate.operation, candidate.attempt)
			return false, err
		}
	}
	var complete bool
	if err = s.Pool.QueryRow(ctx, `SELECT state='GC_COMPLETE' FROM file_blobs WHERE blob_id=$1`, blob).Scan(&complete); err != nil {
		return false, err
	}
	if !complete {
		return false, fmt.Errorf("file deletion remains pending")
	}
	return true, nil
}

func (s *FilesStore) prepareLegacyDeletion(ctx context.Context, id uuid.UUID, expiredAt *time.Time) (uuid.UUID, bool, error) {
	if s == nil || s.Pool == nil {
		return uuid.Nil, false, fmt.Errorf("file persistence not configured")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, false, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	// KEY SHARE from a concurrent reference FK insert must remain compatible.
	row, err := getFileByID(ctx, tx, id, " FOR NO KEY UPDATE")
	if err != nil {
		return uuid.Nil, false, err
	}
	if expiredAt != nil && (row.Status != "ready" || row.ExpiresAt == nil || row.ExpiresAt.After(*expiredAt)) {
		return uuid.Nil, false, nil
	}
	var blob uuid.UUID
	if err = tx.QueryRow(ctx, `SELECT blob_id FROM files WHERE id=$1`, id).Scan(&blob); err != nil {
		return uuid.Nil, false, err
	}
	var state string
	if err = tx.QueryRow(ctx, `SELECT state FROM file_blobs WHERE blob_id=$1 FOR UPDATE`, blob).Scan(&state); err != nil {
		return uuid.Nil, false, err
	}
	// Separate statement obtains a fresh READ COMMITTED snapshot after lock wait.
	var protected bool
	err = tx.QueryRow(ctx, `SELECT
 EXISTS(SELECT 1 FROM files f JOIN file_references r ON r.file_id=f.id WHERE f.blob_id=$1 AND r.released_at IS NULL)
 OR EXISTS(SELECT 1 FROM files f WHERE f.blob_id=$1 AND f.id<>$2 AND f.status NOT IN ('deleted','expired','failed'))`, blob, id).Scan(&protected)
	if err != nil {
		return uuid.Nil, false, err
	}
	if protected {
		return uuid.Nil, false, ErrFileRetained
	}
	// Preserve first terminal transition and music revision on every retry,
	// including a blob already completed by reference GC.
	if row.Status != "deleted" && row.Status != "expired" {
		terminal := "deleted"
		if expiredAt != nil {
			terminal = "expired"
		}
		if err = markUnavailableWithMusicEventTx(ctx, tx, row, terminal); err != nil {
			return uuid.Nil, false, err
		}
	}
	if state != "GC_COMPLETE" {
		_, err = tx.Exec(ctx, `UPDATE file_blobs SET state='GC_PENDING',gc_operation_id=COALESCE(gc_operation_id,$2),deleted_at=COALESCE(deleted_at,clock_timestamp()),next_attempt_at=COALESCE(next_attempt_at,clock_timestamp()) WHERE blob_id=$1`, blob, uuid.New())
		if err != nil {
			return uuid.Nil, false, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return uuid.Nil, false, err
	}
	return blob, true, nil
}
