package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"voice/backend/file/internal/r2file"
)

type referenceGCCandidate struct {
	id               uuid.UUID
	original         string
	converted, thumb *string
	operation        uuid.UUID
	attempt          int
}

// RunReferenceGCOnce processes at most limit durable zero-reference blobs.
// It persists a claim before touching R2, then re-locks and revalidates the
// claim while deleting. A reference resurrection that wins that lock changes
// the blob back to LIVE and makes the physical delete ineligible.
func (s *FilesStore) RunReferenceGCOnce(ctx context.Context, deleter r2file.ObjectDeleter, limit int) (int64, error) {
	if s == nil || s.Pool == nil {
		return 0, fmt.Errorf("file persistence not configured")
	}
	if limit <= 0 {
		limit = 100
	}
	var completed int64
	for claimed := 0; claimed < limit; claimed++ {
		candidate, ok, err := s.claimReferenceGC(ctx)
		if err != nil {
			return completed, err
		}
		if !ok {
			break
		}
		deleted, err := s.deleteClaimedReferenceGC(ctx, deleter, candidate)
		if err != nil {
			_, _ = s.Pool.Exec(ctx, `UPDATE file_blobs SET next_attempt_at=clock_timestamp() WHERE blob_id=$1 AND state='GC_PENDING' AND gc_operation_id=$2 AND gc_attempt=$3`, candidate.id, candidate.operation, candidate.attempt)
			return completed, err
		}
		if deleted {
			completed++
		}
	}
	return completed, nil
}

func (s *FilesStore) claimReferenceGC(ctx context.Context) (referenceGCCandidate, bool, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return referenceGCCandidate{}, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var candidate referenceGCCandidate
	var operation *uuid.UUID
	err = tx.QueryRow(ctx, `
SELECT b.blob_id,b.original_r2_key,b.converted_r2_key,b.thumbnail_r2_key,b.gc_operation_id,b.gc_attempt
FROM file_blobs b
WHERE b.state='GC_PENDING'
  AND (b.next_attempt_at IS NULL OR b.next_attempt_at <= clock_timestamp())
  AND NOT EXISTS (
      SELECT 1 FROM files f JOIN file_references r ON r.file_id=f.id
      WHERE f.blob_id=b.blob_id AND r.released_at IS NULL
  )
ORDER BY b.blob_id
FOR UPDATE OF b SKIP LOCKED
LIMIT 1`).Scan(&candidate.id, &candidate.original, &candidate.converted, &candidate.thumb, &operation, &candidate.attempt)
	if errors.Is(err, pgx.ErrNoRows) {
		return referenceGCCandidate{}, false, nil
	}
	if err != nil {
		return referenceGCCandidate{}, false, err
	}
	if operation == nil {
		candidate.operation = uuid.New()
	} else {
		candidate.operation = *operation
	}
	var nextAttempt int
	err = tx.QueryRow(ctx, `
UPDATE file_blobs b
SET gc_operation_id=COALESCE(gc_operation_id,$2),
    gc_attempt=gc_attempt+1,
    deleted_at=COALESCE(deleted_at,clock_timestamp()),
    next_attempt_at=clock_timestamp()+$4*interval '1 second'
WHERE blob_id=$1 AND state='GC_PENDING' AND gc_attempt=$3
  AND NOT EXISTS (
      SELECT 1 FROM files f JOIN file_references r ON r.file_id=f.id
      WHERE f.blob_id=b.blob_id AND r.released_at IS NULL
  )
RETURNING gc_attempt`, candidate.id, candidate.operation, candidate.attempt, 300).Scan(&nextAttempt)
	if errors.Is(err, pgx.ErrNoRows) {
		return referenceGCCandidate{}, false, nil
	}
	if err != nil {
		return referenceGCCandidate{}, false, err
	}
	candidate.attempt = nextAttempt
	if err := tx.Commit(ctx); err != nil {
		return referenceGCCandidate{}, false, err
	}
	return candidate, true, nil
}

func (s *FilesStore) deleteClaimedReferenceGC(ctx context.Context, deleter r2file.ObjectDeleter, candidate referenceGCCandidate) (bool, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var locked uuid.UUID
	err = tx.QueryRow(ctx, `
SELECT b.blob_id
FROM file_blobs b
WHERE b.blob_id=$1 AND b.state='GC_PENDING' AND b.gc_operation_id=$2 AND b.gc_attempt=$3
  AND NOT EXISTS (
      SELECT 1 FROM files f JOIN file_references r ON r.file_id=f.id
      WHERE f.blob_id=b.blob_id AND r.released_at IS NULL
  )
FOR UPDATE OF b`, candidate.id, candidate.operation, candidate.attempt).Scan(&locked)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	keys := []string{candidate.original}
	if candidate.converted != nil {
		keys = append(keys, *candidate.converted)
	}
	if candidate.thumb != nil {
		keys = append(keys, *candidate.thumb)
	}
	if err := r2file.DeleteKeys(ctx, deleter, keys...); err != nil {
		return false, err
	}
	tag, err := tx.Exec(ctx, `UPDATE file_blobs SET state='GC_COMPLETE',deleted_at=clock_timestamp(),next_attempt_at=NULL WHERE blob_id=$1 AND state='GC_PENDING' AND gc_operation_id=$2 AND gc_attempt=$3`, candidate.id, candidate.operation, candidate.attempt)
	if err != nil {
		return false, err
	}
	if tag.RowsAffected() != 1 {
		return false, fmt.Errorf("gc claim lost before completion")
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}
