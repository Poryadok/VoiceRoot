package store

import (
	"context"
	"crypto/sha256"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var ErrPDFPublicationLost = errors.New("PDF publication authority lost")

// ProcessPDFThumbnailOnce separates preparation, one tracked PUT and current
// publication. No SQL lock is held over PUT. STARTED/UNSETTLED survives process
// and SQL-session loss and prevents collection without an actual storage ACK.
func (s *FilesStore) ProcessPDFThumbnailOnce(ctx context.Context,
	prepare func(context.Context, FileRow) ([]byte, bool, error),
	put func(context.Context, TrackedWriteIntent, []byte) error,
) (bool, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var id uuid.UUID
	err = tx.QueryRow(ctx, `SELECT f.id FROM files f JOIN file_blobs b ON b.blob_id=f.blob_id
WHERE f.file_type='document' AND f.mime_type='application/pdf' AND NOT f.is_e2e
 AND b.state='LIVE' AND b.deleted_at IS NULL
 AND ((f.status='processing' AND f.scan_result IN ('clean','skipped'))
   OR (f.status IN ('deleted','expired','failed') AND EXISTS(SELECT 1 FROM file_catalog_writes settled
       WHERE settled.file_id=f.id AND settled.object_key=f.thumbnail_r2_key AND settled.owner_kind='pdf_thumbnail' AND settled.state='SETTLED')))
 AND NOT EXISTS(SELECT 1 FROM file_catalog_writes w WHERE w.file_id=f.id AND w.state IN ('STARTED','UNSETTLED'))
ORDER BY f.updated_at,f.id FOR NO KEY UPDATE OF f SKIP LOCKED LIMIT 1`).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	before, err := getFileByID(ctx, tx, id, "")
	if err != nil {
		return true, err
	}
	// Resume publication of an acknowledged attempt after a lost SQL response;
	// never replace its key or submit another PUT.
	if before.ThumbnailR2Key != nil {
		var generation uuid.UUID
		err = tx.QueryRow(ctx, `SELECT generation FROM file_catalog_writes WHERE object_key=$1 AND file_id=$2 AND owner_kind='pdf_thumbnail' AND state='SETTLED'`, *before.ThumbnailR2Key, id).Scan(&generation)
		if err == nil {
			_ = tx.Rollback(ctx)
			return true, s.publishPDFThumbnail(ctx, id, *before.ThumbnailR2Key, generation)
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return true, err
		}
	}
	body, permanent, prepareErr := prepare(ctx, before)
	if prepareErr != nil {
		if !permanent {
			_ = tx.Rollback(context.Background())
			// A temporarily unreadable/slow PDF must not monopolize the oldest
			// queue slot, including when its preparation context timed out.
			retryCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer cancel()
			_, touchErr := s.Pool.Exec(retryCtx, `UPDATE files SET updated_at=clock_timestamp() WHERE id=$1 AND status='processing'`, id)
			return true, errors.Join(prepareErr, touchErr)
		}
		if _, err = tx.Exec(ctx, `UPDATE files SET status='failed',updated_at=clock_timestamp() WHERE id=$1`, id); err != nil {
			return true, err
		}
		after := before
		after.Status = "failed"
		if err = recordPreviewChange(ctx, tx, before, after); err != nil {
			return true, err
		}
		if err = queueRetiredPDFBlobGCTx(ctx, tx, id); err != nil {
			return true, err
		}
		return true, tx.Commit(ctx)
	}
	generation := uuid.New()
	key := "processed/" + id.String() + "/" + generation.String() + "/pdf-first-page.png"
	digest := sha256.Sum256(body)
	intent := TrackedWriteIntent{Key: key, FileID: id, Generation: generation, Kind: "pdf_thumbnail", ExpectedSHA256: digest[:], ExpectedSize: int64(len(body))}
	if _, err = tx.Exec(ctx, `UPDATE files SET thumbnail_r2_key=$2,updated_at=clock_timestamp() WHERE id=$1`, id, key); err != nil {
		return true, err
	}
	if err = DeclareTrackedWriteTx(ctx, tx, intent); err != nil {
		return true, err
	}
	if _, err = StartTrackedWriteTx(ctx, tx, key, generation); err != nil {
		return true, err
	}
	if err = tx.Commit(ctx); err != nil {
		return true, err
	}
	putErr := put(ctx, intent, body)
	settleErr := s.SettleTrackedWrite(ctx, key, generation, putErr == nil)
	if putErr != nil || settleErr != nil {
		return true, errors.Join(putErr, settleErr)
	}
	return true, s.publishPDFThumbnail(ctx, id, key, generation)
}

func (s *FilesStore) publishPDFThumbnail(ctx context.Context, id uuid.UUID, key string, generation uuid.UUID) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	before, err := getFileByID(ctx, tx, id, " FOR NO KEY UPDATE")
	if err != nil {
		return err
	}
	var blob uuid.UUID
	var state string
	var deleted bool
	if err = tx.QueryRow(ctx, `SELECT b.blob_id,b.state,b.deleted_at IS NOT NULL FROM files f JOIN file_blobs b ON b.blob_id=f.blob_id WHERE f.id=$1 FOR UPDATE OF b`, id).Scan(&blob, &state, &deleted); err != nil {
		return err
	}
	var settled, retired bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM file_catalog_writes WHERE object_key=$1 AND generation=$2 AND file_id=$3 AND owner_kind='pdf_thumbnail' AND state='SETTLED')`, key, generation, id).Scan(&settled); err != nil {
		return err
	}
	// Last-reference release may leave the blob LIVE while the pin blocks GC.
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM file_references WHERE file_id=$1)
 AND NOT EXISTS(SELECT 1 FROM file_references WHERE file_id=$1 AND released_at IS NULL)`, id).Scan(&retired); err != nil {
		return err
	}
	eligible := settled && state == "LIVE" && !deleted && !retired && !before.IsE2E && before.FileType == "document" && before.MimeType == "application/pdf" && (before.ScanResult == "clean" || before.ScanResult == "skipped") && before.ThumbnailR2Key != nil && *before.ThumbnailR2Key == key
	if eligible && before.Status == "ready" {
		return tx.Commit(ctx)
	}
	current := eligible && before.Status == "processing"
	if !current {
		// ACK settles physical custody only. Deletion/release prevents READY;
		// queue GC only when no reference or other usable alias retains the blob.
		if retired || state != "LIVE" || before.Status == "deleted" || before.Status == "expired" || before.Status == "failed" {
			if err = queueRetiredPDFBlobGCTx(ctx, tx, id); err != nil {
				return err
			}
		}
		if err = tx.Commit(ctx); err != nil {
			return err
		}
		return ErrPDFPublicationLost
	}
	if _, err = tx.Exec(ctx, `UPDATE files SET status='ready',updated_at=clock_timestamp() WHERE id=$1`, id); err != nil {
		return err
	}
	after := before
	after.Status = "ready"
	if err = recordPreviewChange(ctx, tx, before, after); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// The caller owns the File lock. Match the legacy File->blob lock order and
// preserve every live reference/usable alias; physical GC uses the shared pin.
func queueRetiredPDFBlobGCTx(ctx context.Context, tx pgx.Tx, id uuid.UUID) error {
	var blob uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT b.blob_id FROM files f JOIN file_blobs b ON b.blob_id=f.blob_id WHERE f.id=$1 FOR UPDATE OF b`, id).Scan(&blob); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `UPDATE file_blobs b SET state='GC_PENDING',gc_operation_id=COALESCE(gc_operation_id,$2),next_attempt_at=clock_timestamp()
WHERE blob_id=$1 AND state<>'GC_COMPLETE'
 AND NOT EXISTS(SELECT 1 FROM files f JOIN file_references r ON r.file_id=f.id WHERE f.blob_id=b.blob_id AND r.released_at IS NULL)
 AND NOT EXISTS(SELECT 1 FROM files f WHERE f.blob_id=b.blob_id AND f.id<>$3 AND f.status NOT IN ('deleted','expired','failed'))`, blob, uuid.New(), id)
	return err
}
