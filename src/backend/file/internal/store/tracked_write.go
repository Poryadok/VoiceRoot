package store

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var ErrTrackedWrite = errors.New("tracked object write unavailable")

// TrackedWriteIntent contains measured immutable bytes for one unique owned
// attempt key. It is physical write custody, never actor/publication authority.
type TrackedWriteIntent struct {
	Key                string
	FileID, Generation uuid.UUID
	Kind               string
	ExpectedSHA256     []byte
	ExpectedSize       int64
}

func validTrackedIntent(w TrackedWriteIntent) bool {
	if w.Key == "" || w.Key != strings.TrimSpace(w.Key) || w.FileID == uuid.Nil || w.Generation == uuid.Nil || len(w.ExpectedSHA256) != 32 || w.ExpectedSize <= 0 {
		return false
	}
	switch w.Kind {
	case "catalog_source":
		return w.ExpectedSize <= 50<<20
	case "pdf_thumbnail", "gif_full", "gif_thumbnail":
		return w.ExpectedSize <= 5<<20
	default:
		return false
	}
}

func readTrackedIntentTx(ctx context.Context, tx pgx.Tx, key string, generation uuid.UUID, lock bool) (TrackedWriteIntent, string, error) {
	var w TrackedWriteIntent
	var state string
	suffix := ""
	if lock {
		suffix = " FOR UPDATE"
	}
	err := tx.QueryRow(ctx, `SELECT object_key,file_id,generation,COALESCE(to_jsonb(w)->>'owner_kind','catalog_source'),expected_sha256,expected_size_bytes,state FROM file_catalog_writes w WHERE object_key=$1 AND generation=$2`+suffix, key, generation).Scan(&w.Key, &w.FileID, &w.Generation, &w.Kind, &w.ExpectedSHA256, &w.ExpectedSize, &state)
	if err == nil && !validTrackedIntent(w) {
		err = ErrTrackedWrite
	}
	return w, state, err
}

func checkTrackedOwnerFileTx(ctx context.Context, tx pgx.Tx, w TrackedWriteIntent) error {
	if !validTrackedIntent(w) {
		return ErrTrackedWrite
	}
	var status, mime, intent, key string
	var sourceSHA *string
	var size int64
	var e2e, catalog bool
	var blob uuid.UUID
	err := tx.QueryRow(ctx, `SELECT status,mime_type,upload_intent,r2_key,sha256_hash,size_bytes,is_e2e,blob_id,COALESCE((to_jsonb(f)->>'catalog_asset')::boolean,false)
FROM files f WHERE id=$1 FOR NO KEY UPDATE`, w.FileID).Scan(&status, &mime, &intent, &key, &sourceSHA, &size, &e2e, &blob, &catalog)
	if err != nil {
		return err
	}
	if e2e || sourceSHA == nil || size <= 0 {
		return ErrTrackedWrite
	}
	sha, err := hex.DecodeString(*sourceSHA)
	if err != nil || len(sha) != 32 {
		return ErrTrackedWrite
	}
	mime = strings.ToLower(strings.TrimSpace(strings.Split(mime, ";")[0]))
	switch w.Kind {
	case "catalog_source":
		if !catalog || status != "pending_upload" || intent != "gif" || key != w.Key || size != w.ExpectedSize || !bytes.Equal(sha, w.ExpectedSHA256) {
			return ErrTrackedWrite
		}
	case "pdf_thumbnail":
		if status != "processing" || mime != "application/pdf" {
			return ErrTrackedWrite
		}
	case "gif_full", "gif_thumbnail":
		if status != "processing" || intent != "gif" {
			return ErrTrackedWrite
		}
		var current bool
		if err = tx.QueryRow(ctx, `SELECT state='running' AND attempt_token=$2 AND lease_until>clock_timestamp() FROM file_gif_jobs WHERE file_id=$1 FOR SHARE`, w.FileID, w.Generation).Scan(&current); err != nil {
			return err
		}
		if !current {
			return ErrTrackedWrite
		}
	default:
		return ErrTrackedWrite
	}
	var live bool
	if err = tx.QueryRow(ctx, `SELECT state='LIVE' AND deleted_at IS NULL FROM file_blobs WHERE blob_id=$1 FOR SHARE`, blob).Scan(&live); err != nil {
		return err
	}
	if !live {
		return ErrTrackedWrite
	}
	return nil
}

// DeclareTrackedWriteTx is called under the owner's current-generation/File
// transaction after actual output length/digest exist and before any PUT. PDF
// owner kinds require additive migration 11; no catalog actor methods are used.
func DeclareTrackedWriteTx(ctx context.Context, tx pgx.Tx, w TrackedWriteIntent) error {
	if tx == nil {
		return ErrTrackedWrite
	}
	if err := checkTrackedOwnerFileTx(ctx, tx, w); err != nil {
		return err
	}
	var typed bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema='public' AND table_name='file_catalog_writes' AND column_name='owner_kind')`).Scan(&typed); err != nil {
		return err
	}
	var err error
	if typed {
		_, err = tx.Exec(ctx, `INSERT INTO file_catalog_writes(object_key,file_id,generation,owner_kind,state,expected_sha256,expected_size_bytes) VALUES($1,$2,$3,$4,'DECLARED',$5,$6) ON CONFLICT(object_key) DO NOTHING`, w.Key, w.FileID, w.Generation, w.Kind, w.ExpectedSHA256, w.ExpectedSize)
	} else {
		if w.Kind != "catalog_source" {
			return ErrTrackedWrite
		}
		_, err = tx.Exec(ctx, `INSERT INTO file_catalog_writes(object_key,file_id,generation,state,expected_sha256,expected_size_bytes) VALUES($1,$2,$3,'DECLARED',$4,$5) ON CONFLICT(object_key) DO NOTHING`, w.Key, w.FileID, w.Generation, w.ExpectedSHA256, w.ExpectedSize)
	}
	if err != nil {
		return err
	}
	existing, state, err := readTrackedIntentTx(ctx, tx, w.Key, w.Generation, true)
	if err != nil {
		return err
	}
	if state != "DECLARED" || existing.FileID != w.FileID || existing.Kind != w.Kind || existing.ExpectedSize != w.ExpectedSize || !bytes.Equal(existing.ExpectedSHA256, w.ExpectedSHA256) {
		return ErrTrackedWrite
	}
	return nil
}

// StartTrackedWriteTx must share the owner's generation-check transaction.
// Committed STARTED is a nonexpiring physical pin. It permits exactly one PUT,
// never an SDK retry or a second Start under the same unique key/generation.
func StartTrackedWriteTx(ctx context.Context, tx pgx.Tx, key string, generation uuid.UUID) (TrackedWriteIntent, error) {
	if tx == nil || key == "" || generation == uuid.Nil {
		return TrackedWriteIntent{}, ErrTrackedWrite
	}
	w, _, err := readTrackedIntentTx(ctx, tx, key, generation, false)
	if err != nil {
		return w, err
	}
	if err = checkTrackedOwnerFileTx(ctx, tx, w); err != nil {
		return w, err
	}
	current, state, err := readTrackedIntentTx(ctx, tx, key, generation, true)
	if err != nil {
		return w, err
	}
	if current.FileID != w.FileID || state != "DECLARED" {
		return w, ErrTrackedWrite
	}
	tag, err := tx.Exec(ctx, `UPDATE file_catalog_writes SET state='STARTED' WHERE object_key=$1 AND generation=$2 AND state='DECLARED'`, key, generation)
	if err != nil {
		return w, err
	}
	if tag.RowsAffected() != 1 {
		return w, ErrTrackedWrite
	}
	return current, nil
}

func (s *FilesStore) StartTrackedWrite(ctx context.Context, key string, generation uuid.UUID) (TrackedWriteIntent, error) {
	if s == nil || s.Pool == nil {
		return TrackedWriteIntent{}, ErrTrackedWrite
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return TrackedWriteIntent{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	w, err := StartTrackedWriteTx(ctx, tx, key, generation)
	if err != nil {
		return w, err
	}
	return w, tx.Commit(ctx)
}

// SettleTrackedWrite records only the actual single-attempt I/O outcome using
// a fresh connection. File deletion, actor purge or publication generation loss
// cannot suppress an acknowledged outcome. No admin/recovery boolean is exposed
// over an RPC; only the tracked executor calls this with its storage result.
func (s *FilesStore) SettleTrackedWrite(ctx context.Context, key string, generation uuid.UUID, acknowledged bool) error {
	if s == nil || s.Pool == nil || key == "" || generation == uuid.Nil {
		return ErrTrackedWrite
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	_, state, err := readTrackedIntentTx(ctx, tx, key, generation, true)
	if err != nil {
		return err
	}
	if acknowledged && (state == "SETTLED" || state == "GC_PENDING" || state == "DONE") {
		return tx.Commit(ctx)
	}
	if !acknowledged && state == "UNSETTLED" {
		return tx.Commit(ctx)
	}
	if state != "STARTED" {
		return ErrTrackedWrite
	}
	next := "UNSETTLED"
	if acknowledged {
		next = "SETTLED"
	}
	if _, err = tx.Exec(ctx, `UPDATE file_catalog_writes SET state=$3 WHERE object_key=$1 AND generation=$2 AND state='STARTED'`, key, generation, next); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func trackedWriteLedgerExists(ctx context.Context, tx pgx.Tx) (bool, error) {
	var exists bool
	err := tx.QueryRow(ctx, `SELECT to_regclass('public.file_catalog_writes') IS NOT NULL`).Scan(&exists)
	return exists, err
}

// CanCollectTrackedObjectTx is the common physical I/O predicate, independent of
// the caller's File/reference/attempt ownership predicate. The caller already
// locks the File before checking, matching every tracked write's start ordering.
// Missing old schemas are permitted only for ordinary pre-ledger Files.
func CanCollectTrackedObjectTx(ctx context.Context, tx pgx.Tx, file uuid.UUID, key string) (bool, error) {
	if tx == nil || file == uuid.Nil || key == "" {
		return false, ErrTrackedWrite
	}
	exists, err := trackedWriteLedgerExists(ctx, tx)
	if err != nil {
		return false, err
	}
	var blocked bool
	if !exists {
		err = tx.QueryRow(ctx, `SELECT COALESCE((to_jsonb(f)->>'catalog_asset')::boolean,false) FROM files f WHERE id=$1`, file).Scan(&blocked)
	} else {
		err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM file_catalog_writes WHERE (file_id=$1 OR object_key=$2) AND state IN ('STARTED','UNSETTLED'))`, file, key).Scan(&blocked)
	}
	return !blocked, err
}

// CanCollectTrackedBlobTx covers all aliases/keys sharing a physical blob. A
// session disappearing, timeout, HEAD observation or elapsed age never clears a
// STARTED/UNSETTLED pin. Reference and lifecycle authority remain separate.
func CanCollectTrackedBlobTx(ctx context.Context, tx pgx.Tx, blob uuid.UUID) (bool, error) {
	if tx == nil || blob == uuid.Nil {
		return false, ErrTrackedWrite
	}
	exists, err := trackedWriteLedgerExists(ctx, tx)
	if err != nil {
		return false, err
	}
	var blocked bool
	if !exists {
		err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM files f WHERE blob_id=$1 AND COALESCE((to_jsonb(f)->>'catalog_asset')::boolean,false))`, blob).Scan(&blocked)
	} else {
		err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM files f JOIN file_catalog_writes w ON w.file_id=f.id WHERE f.blob_id=$1 AND w.state IN ('STARTED','UNSETTLED'))`, blob).Scan(&blocked)
	}
	return !blocked, err
}
