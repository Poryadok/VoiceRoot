package store

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ProcessPDFThumbnailOnce uses processing rows as a durable queue. Register the
// deterministic key in files AND file_blobs before any PUT; a crash at any later
// point leaves the object discoverable by normal delete/retention GC.
func (s *FilesStore) ProcessPDFThumbnailOnce(ctx context.Context, process func(context.Context, FileRow) (permanent bool, err error)) (bool, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var id uuid.UUID
	err = tx.QueryRow(ctx, `SELECT id FROM files WHERE status='processing' AND file_type='document' AND mime_type='application/pdf' AND NOT is_e2e AND scan_result IN ('clean','skipped') ORDER BY updated_at,id FOR NO KEY UPDATE SKIP LOCKED LIMIT 1`).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	key := "processed/" + id.String() + "/pdf-first-page.png"
	if _, err = tx.Exec(ctx, `UPDATE files SET thumbnail_r2_key=$2,updated_at=clock_timestamp() WHERE id=$1`, id, key); err != nil {
		return true, err
	}
	if err = tx.Commit(ctx); err != nil {
		return true, err
	}

	// Re-lock and revalidate after the reservation commit. Delete may have won
	// in between. Keep this lock through storage writes and the preview commit.
	tx, err = s.Pool.Begin(ctx)
	if err != nil {
		return true, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	before, err := getFileByID(ctx, tx, id, " FOR NO KEY UPDATE")
	if err != nil {
		return true, err
	}
	if before.Status != "processing" || before.IsE2E || before.ScanResult != "clean" && before.ScanResult != "skipped" {
		return false, nil
	}
	permanent, processErr := process(ctx, before)
	if processErr != nil && !permanent {
		return true, processErr
	}
	nextStatus := "ready"
	if permanent {
		nextStatus = "failed"
	}
	if _, err = tx.Exec(ctx, `UPDATE files SET status=$2,updated_at=clock_timestamp() WHERE id=$1`, id, nextStatus); err != nil {
		return true, err
	}
	after, err := getFileByID(ctx, tx, id, "")
	if err != nil {
		return true, err
	}
	if err = recordPreviewChange(ctx, tx, before, after); err != nil {
		return true, err
	}
	// Never delete the registered key on an ambiguous commit. A retry observes
	// ready or replaces the same key, and terminal deletion retains GC authority.
	return true, tx.Commit(ctx)
}
