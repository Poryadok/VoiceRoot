package store

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var ErrGIFPublicationLost = errors.New("GIF publication authority lost")

// The caller holds the File; take the blob lock and recheck references before
// changing publication authority. Retirement never clears an
// unresolved write pin. A later actual ACK remains physical settlement only.
func retireGIFPublicationTx(ctx context.Context, tx pgx.Tx, before FileRow) (bool, error) {
	var blob uuid.UUID
	var state string
	var deleting, retired bool
	if err := tx.QueryRow(ctx, `SELECT b.blob_id,b.state,b.deleted_at IS NOT NULL
FROM files f JOIN file_blobs b ON b.blob_id=f.blob_id WHERE f.id=$1 FOR UPDATE OF b`, before.ID).Scan(&blob, &state, &deleting); err != nil {
		return false, err
	}
	// A lock wait may refresh b without refreshing subqueries from that
	// statement's snapshot. Observe committed reference winners only AFTER it.
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM file_references WHERE file_id=$1) AND NOT EXISTS(SELECT 1 FROM file_references WHERE file_id=$1 AND released_at IS NULL)`, before.ID).Scan(&retired); err != nil {
		return false, err
	}
	if before.Status == "processing" && state == "LIVE" && !deleting && !retired {
		return false, nil
	}
	if before.Status == "processing" {
		if _, err := tx.Exec(ctx, `UPDATE files SET status='failed',updated_at=clock_timestamp() WHERE id=$1`, before.ID); err != nil {
			return false, err
		}
		after := before
		after.Status = "failed"
		if err := recordPreviewChange(ctx, tx, before, after); err != nil {
			return false, err
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE file_gif_jobs SET state='failed',lease_until=NULL,updated_at=clock_timestamp() WHERE file_id=$1 AND state IN ('queued','running')`, before.ID); err != nil {
		return false, err
	}
	if err := queueGIFOriginalCleanupTx(ctx, tx, before); err != nil {
		return false, err
	}
	if _, err := tx.Exec(ctx, `UPDATE file_blobs b SET state='GC_PENDING',gc_operation_id=COALESCE(gc_operation_id,$2),next_attempt_at=clock_timestamp()
WHERE blob_id=$1 AND state<>'GC_COMPLETE'
 AND NOT EXISTS(SELECT 1 FROM files f JOIN file_references r ON r.file_id=f.id WHERE f.blob_id=b.blob_id AND r.released_at IS NULL)
 AND NOT EXISTS(SELECT 1 FROM files f WHERE f.blob_id=b.blob_id AND f.id<>$3 AND f.status NOT IN ('deleted','expired','failed'))`, blob, uuid.New(), before.ID); err != nil {
		return false, err
	}
	// Known outcomes need no second PUT. Unresolved outcomes remain unchanged;
	// if their original executor later records an ACK, recovery sees SETTLED.
	_, err := tx.Exec(ctx, `UPDATE file_catalog_writes SET state='GC_PENDING' WHERE file_id=$1 AND owner_kind IN ('gif_full','gif_thumbnail') AND state='SETTLED'`, before.ID)
	return true, err
}

// ReconcileRetiredGIFWritesOnce is the bounded consumer for a crash between
// known ACK and publication/GC. It also fences further starts after logical
// last-reference retirement while an old executor is still in flight. It never
// infers remote quiescence from time, lease loss, HEAD or an administrator flag.
func (s *FilesStore) ReconcileRetiredGIFWritesOnce(ctx context.Context) (bool, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var typed bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema='public' AND table_name='file_catalog_writes' AND column_name='owner_kind')`).Scan(&typed); err != nil {
		return false, err
	}
	if !typed {
		return false, nil
	}
	var id uuid.UUID
	err = tx.QueryRow(ctx, `SELECT f.id FROM files f WHERE f.upload_intent='gif'
 AND ((f.status IN ('failed','deleted','expired') AND EXISTS(SELECT 1 FROM file_catalog_writes w WHERE w.file_id=f.id AND w.owner_kind IN ('gif_full','gif_thumbnail') AND w.state='SETTLED'))
 OR (f.status='processing' AND EXISTS(SELECT 1 FROM file_references r WHERE r.file_id=f.id) AND NOT EXISTS(SELECT 1 FROM file_references r WHERE r.file_id=f.id AND r.released_at IS NULL)))
 ORDER BY f.id FOR NO KEY UPDATE OF f SKIP LOCKED LIMIT 1`).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	before, err := getFileByID(ctx, tx, id, "")
	if err != nil {
		return false, err
	}
	retired, err := retireGIFPublicationTx(ctx, tx, before)
	if err != nil {
		return false, err
	}
	return retired, tx.Commit(ctx)
}
