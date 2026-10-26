package store

import (
	"bytes"
	"context"
	"encoding/hex"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func gifTrackedModeTx(ctx context.Context, tx pgx.Tx, id uuid.UUID) (bool, error) {
	var typed, catalog bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema='public' AND table_name='file_catalog_writes' AND column_name='owner_kind'),COALESCE((to_jsonb(f)->>'catalog_asset')::boolean,false) FROM files f WHERE id=$1`, id).Scan(&typed, &catalog)
	if err != nil {
		return false, err
	}
	if catalog && !typed {
		return false, ErrTrackedWrite
	}
	return typed, nil
}

// PrepareGIFTrackedWrites declares both measured outputs before either PUT.
// Existing attempt ownership and the common File/blob checks fence declaration;
// the generic start call rechecks them immediately before each network attempt.
func (s *FilesStore) PrepareGIFTrackedWrites(ctx context.Context, job *GIFJob, full, thumb *Representation) error {
	if job == nil || !validGIFRepresentation(full, job.FullKey, "video/mp4") || !validGIFRepresentation(thumb, job.ThumbnailKey, "image/webp") {
		return ErrTrackedWrite
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	for i, r := range []*Representation{full, thumb} {
		digest, err := hex.DecodeString(r.SHA256)
		if err != nil {
			return ErrTrackedWrite
		}
		kind := "gif_full"
		if i == 1 {
			kind = "gif_thumbnail"
		}
		if err = DeclareTrackedWriteTx(ctx, tx, TrackedWriteIntent{Key: r.Key, FileID: job.Row.ID, Generation: job.Token, Kind: kind, ExpectedSHA256: digest, ExpectedSize: r.SizeBytes}); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func requireGIFTrackedResultsTx(ctx context.Context, tx pgx.Tx, job *GIFJob, full, thumb *Representation) error {
	typed, err := gifTrackedModeTx(ctx, tx, job.Row.ID)
	if err != nil {
		return err
	}
	if !typed {
		return nil
	} // Only ordinary pre-ledger schemas can take this path.
	var sourceSettled bool
	if err = tx.QueryRow(ctx, `SELECT NOT COALESCE((to_jsonb(f)->>'catalog_asset')::boolean,false) OR EXISTS(
SELECT 1 FROM file_catalog_writes w WHERE w.file_id=f.id AND w.owner_kind='catalog_source' AND w.object_key=f.r2_key
AND w.state='SETTLED' AND w.expected_size_bytes=f.size_bytes AND encode(w.expected_sha256,'hex')=f.sha256_hash)
FROM files f WHERE f.id=$1`, job.Row.ID).Scan(&sourceSettled); err != nil {
		return err
	}
	if !sourceSettled {
		return ErrTrackedWrite
	}
	var live bool
	if err = tx.QueryRow(ctx, `SELECT b.state='LIVE' AND b.deleted_at IS NULL FROM files f JOIN file_blobs b ON b.blob_id=f.blob_id WHERE f.id=$1 FOR UPDATE OF b`, job.Row.ID).Scan(&live); err != nil {
		return err
	}
	if !live {
		return ErrGIFPublicationLost
	}
	var retired bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM file_references WHERE file_id=$1) AND NOT EXISTS(SELECT 1 FROM file_references WHERE file_id=$1 AND released_at IS NULL)`, job.Row.ID).Scan(&retired); err != nil {
		return err
	}
	if retired {
		return ErrGIFPublicationLost
	}
	for i, r := range []*Representation{full, thumb} {
		w, state, err := readTrackedIntentTx(ctx, tx, r.Key, job.Token, false)
		if err != nil {
			return err
		}
		digest, err := hex.DecodeString(r.SHA256)
		if err != nil {
			return ErrTrackedWrite
		}
		kind := "gif_full"
		if i == 1 {
			kind = "gif_thumbnail"
		}
		if state != "SETTLED" || w.FileID != job.Row.ID || w.Kind != kind || w.ExpectedSize != r.SizeBytes || !bytes.Equal(w.ExpectedSHA256, digest) {
			return ErrTrackedWrite
		}
	}
	return nil
}
