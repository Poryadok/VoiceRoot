package store

import (
	"context"
	"encoding/hex"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"voice/backend/file/internal/catalogasset"
)

func (s *FilesStore) catalogWriteTx(ctx context.Context, imp CatalogImport, keys *catalogasset.ReplayKeys) (pgx.Tx, string, string, error) {
	tx, e := s.Pool.Begin(ctx)
	if e != nil {
		return nil, "", "", e
	}
	fail := func(err error) (pgx.Tx, string, string, error) {
		_ = tx.Rollback(context.Background())
		return nil, "", "", err
	}
	if e = catalogActorLock(ctx, tx, imp.Actor, keys); e != nil {
		return fail(e)
	}
	var state, key string
	if e = tx.QueryRow(ctx, `SELECT status,r2_key FROM files WHERE id=$1 AND catalog_asset FOR UPDATE`, imp.FileID).Scan(&state, &key); e != nil {
		return fail(e)
	}
	if key != imp.R2Key || imp.Generation == uuid.Nil {
		return fail(ErrCatalogTerminal)
	}
	return tx, state, key, nil
}

func catalogWriteSource(ctx context.Context, tx pgx.Tx, imp CatalogImport, c CatalogActorCapture, keys *catalogasset.ReplayKeys) error {
	if e := checkCatalogCapture(ctx, tx, c); e != nil {
		return e
	}
	if e := checkCatalogCaptureActor(ctx, tx, c, imp.Actor, keys); e != nil {
		return e
	}
	var match bool
	e := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM file_catalog_assets WHERE file_id=$1 AND source_id=$2 AND actor_account_id=$3 AND actor_profile_id=$4
AND account_gate_id=$5 AND profile_gate_id=$6 AND account_generation=$7 AND profile_generation=$8)`, imp.FileID, imp.SourceID, imp.Actor.AccountID, imp.Actor.ProfileID, c.AccountGate, c.ProfileGate, c.AccountGeneration, c.ProfileGeneration).Scan(&match)
	if e != nil {
		return e
	}
	if !match {
		return ErrCatalogActorClosed
	}
	return nil
}

// RecordCatalogFetched persists actual measured facts and a unique durable write
// declaration before any object I/O. The byte budget is never used as file size.
func (s *FilesStore) RecordCatalogFetched(ctx context.Context, imp CatalogImport, c CatalogActorCapture, keys *catalogasset.ReplayKeys, size int64, digest []byte) error {
	if size <= 0 || size > 50<<20 || len(digest) != 32 {
		return catalogasset.ErrAdmission
	}
	tx, state, key, e := s.catalogWriteTx(ctx, imp, keys)
	if e != nil {
		return e
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if state != "pending_upload" {
		return ErrCatalogTerminal
	}
	var phase string
	var budget int64
	e = tx.QueryRow(ctx, `SELECT phase,max_fetch_bytes FROM file_catalog_import_operations WHERE key_version=$1 AND operation_hmac=$2 AND generation=$3 AND file_id=$4 FOR UPDATE`, imp.KeyVersion, imp.OperationHMAC, imp.Generation, imp.FileID).Scan(&phase, &budget)
	if e != nil {
		return e
	}
	if phase != "PENDING" || size > budget {
		return ErrCatalogTerminal
	}
	if e = catalogWriteSource(ctx, tx, imp, c, keys); e != nil {
		return e
	}
	tag, e := tx.Exec(ctx, `UPDATE files SET size_bytes=$2,sha256_hash=$3,catalog_source_size_unknown=false,updated_at=clock_timestamp() WHERE id=$1 AND catalog_source_size_unknown AND size_bytes=0`, imp.FileID, size, hex.EncodeToString(digest))
	if e != nil {
		return e
	}
	if tag.RowsAffected() != 1 {
		return ErrCatalogTerminal
	}
	if _, e = tx.Exec(ctx, `UPDATE file_catalog_assets SET source_sha256=$2,source_url_sha256=NULL WHERE file_id=$1`, imp.FileID, digest); e != nil {
		return e
	}
	if e = DeclareTrackedWriteTx(ctx, tx, TrackedWriteIntent{Key: key, FileID: imp.FileID, Generation: imp.Generation, Kind: "catalog_source", ExpectedSHA256: digest, ExpectedSize: size}); e != nil {
		return e
	}
	if _, e = tx.Exec(ctx, `UPDATE file_catalog_import_operations SET phase='FETCHED' WHERE key_version=$1 AND operation_hmac=$2`, imp.KeyVersion, imp.OperationHMAC); e != nil {
		return e
	}
	return tx.Commit(ctx)
}

// FailCatalogFetch terminalizes only a pre-write, unknown-size import. It keeps
// the finite File-owned operation receipt but removes the unmeasured File; no
// invented size, fake reference or abandoned URL reservation is required.
func (s *FilesStore) FailCatalogFetch(ctx context.Context, imp CatalogImport, keys *catalogasset.ReplayKeys) error {
	tx, state, _, e := s.catalogWriteTx(ctx, imp, keys)
	if e != nil {
		return e
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if state != "pending_upload" {
		return ErrCatalogTerminal
	}
	var unknown bool
	var size int64
	var writes bool
	if e = tx.QueryRow(ctx, `SELECT catalog_source_size_unknown,size_bytes,EXISTS(SELECT 1 FROM file_catalog_writes WHERE file_id=$1) FROM files WHERE id=$1`, imp.FileID).Scan(&unknown, &size, &writes); e != nil {
		return e
	}
	if !unknown || size != 0 || writes {
		return ErrCatalogTerminal
	}
	tag, e := tx.Exec(ctx, `UPDATE file_catalog_import_operations SET phase='COMPLETE',outcome='failed',completed_at=statement_timestamp(),retain_until=statement_timestamp()+interval '30 days'
WHERE key_version=$1 AND operation_hmac=$2 AND generation=$3 AND file_id=$4 AND phase='PENDING'`, imp.KeyVersion, imp.OperationHMAC, imp.Generation, imp.FileID)
	// Use one statement timestamp for the exact P30D check constraint.
	if e != nil {
		return e
	}
	if tag.RowsAffected() != 1 {
		return ErrCatalogTerminal
	}
	source, e := keys.Source(imp.Actor, imp.SourceID)
	if e != nil {
		return e
	}
	asset, e := keys.Asset(imp.Actor, imp.SourceID, imp.Provider, imp.ProviderID, imp.FileID)
	if e != nil {
		return e
	}
	for _, fence := range []struct {
		purpose string
		digest  catalogasset.ReplayDigest
	}{{"source", source[0]}, {"asset", asset[0]}} {
		if _, e = tx.Exec(ctx, `INSERT INTO file_catalog_terminal_fences(purpose,key_version,identity_hmac,outcome,fence_id) VALUES($1,$2,$3,'failed',$4) ON CONFLICT(purpose,key_version,identity_hmac) DO NOTHING`, fence.purpose, fence.digest.KeyVersion, fence.digest.HMAC, uuid.New()); e != nil {
			return e
		}
	}
	if _, e = tx.Exec(ctx, `DELETE FROM file_catalog_assets WHERE file_id=$1`, imp.FileID); e != nil {
		return e
	}
	if _, e = tx.Exec(ctx, `DELETE FROM files WHERE id=$1`, imp.FileID); e != nil {
		return e
	}
	if _, e = tx.Exec(ctx, `UPDATE file_blobs SET state='GC_PENDING',gc_operation_id=COALESCE(gc_operation_id,$2),next_attempt_at=clock_timestamp() WHERE blob_id=$1 AND state='LIVE'`, imp.FileID, uuid.New()); e != nil {
		return e
	}
	return tx.Commit(ctx)
}

func (s *FilesStore) StartCatalogWrite(ctx context.Context, imp CatalogImport, c CatalogActorCapture, keys *catalogasset.ReplayKeys) error {
	tx, state, _, e := s.catalogWriteTx(ctx, imp, keys)
	if e != nil {
		return e
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if state != "pending_upload" {
		return ErrCatalogTerminal
	}
	if e = catalogWriteSource(ctx, tx, imp, c, keys); e != nil {
		return e
	}
	var live bool
	if e = tx.QueryRow(ctx, `SELECT state='LIVE' AND deleted_at IS NULL FROM file_blobs WHERE blob_id=$1 FOR SHARE`, imp.FileID).Scan(&live); e != nil {
		return e
	}
	if !live {
		return ErrCatalogTerminal
	}
	tag, e := tx.Exec(ctx, `UPDATE file_catalog_import_operations SET phase='WRITE_STARTED' WHERE key_version=$1 AND operation_hmac=$2 AND generation=$3 AND file_id=$4 AND phase='FETCHED'`, imp.KeyVersion, imp.OperationHMAC, imp.Generation, imp.FileID)
	if e != nil {
		return e
	}
	if tag.RowsAffected() != 1 {
		return ErrCatalogTerminal
	}
	if _, e = StartTrackedWriteTx(ctx, tx, imp.R2Key, imp.Generation); e != nil {
		return e
	}
	return tx.Commit(ctx)
}

// SettleCatalogWrite is called by the tracked source PUT executor only. A true
// acknowledgement requires one storage attempt with no hidden transport/SDK
// retries; it is not an administrator-provided crash/recovery fencing claim.
// Every error is UNSETTLED. No timer/session loss can promote it to settled.
func (s *FilesStore) SettleCatalogWrite(ctx context.Context, imp CatalogImport, c CatalogActorCapture, keys *catalogasset.ReplayKeys, acknowledged bool) error {
	// Physical outcome is independent of caller cancellation and actor/source
	// publication authority. A later authorization failure cannot erase this ACK.
	if e := s.SettleTrackedWrite(ctx, imp.R2Key, imp.Generation, acknowledged); e != nil {
		return e
	}
	tx, state, _, e := s.catalogWriteTx(ctx, imp, keys)
	if e != nil {
		return e
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var writeState string
	e = tx.QueryRow(ctx, `SELECT state FROM file_catalog_writes WHERE file_id=$1 AND generation=$2 FOR UPDATE`, imp.FileID, imp.Generation).Scan(&writeState)
	if e != nil {
		return e
	}
	if acknowledged && (writeState == "GC_PENDING" || writeState == "DONE") {
		return tx.Commit(ctx)
	}
	if (acknowledged && writeState != "SETTLED") || (!acknowledged && writeState != "UNSETTLED") {
		return ErrCatalogTerminal
	}
	before, e := getFileByID(ctx, tx, imp.FileID, "")
	if e != nil {
		return e
	}
	if !acknowledged {
		if state == "pending_upload" || state == "processing" {
			if e = catalogFailMeasuredFile(ctx, tx, before); e != nil {
				return e
			}
		}
		// Physical reconciliation remains represented by UNSETTLED even after
		// the user-visible failed operation has a finite completed receipt.
		return tx.Commit(ctx)
	}
	gateError := catalogWriteSource(ctx, tx, imp, c, keys)
	if gateError == nil && state == "processing" {
		return tx.Commit(ctx)
	}
	if gateError != nil || state != "pending_upload" {
		if gateError != nil && !errors.Is(gateError, ErrCatalogActorClosed) && !errors.Is(gateError, pgx.ErrNoRows) {
			return gateError
		}
		if state == "pending_upload" || state == "processing" {
			if e = catalogFailMeasuredFile(ctx, tx, before); e != nil {
				return e
			}
		}
		var referenced bool
		if e = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM file_references WHERE file_id=$1 AND released_at IS NULL)`, imp.FileID).Scan(&referenced); e != nil {
			return e
		}
		next := "SETTLED"
		if !referenced {
			next = "GC_PENDING"
		}
		if _, e = tx.Exec(ctx, `UPDATE file_catalog_writes SET state=$3 WHERE file_id=$1 AND generation=$2`, imp.FileID, imp.Generation, next); e != nil {
			return e
		}
		if !referenced {
			if _, e = tx.Exec(ctx, `UPDATE file_blobs SET state='GC_PENDING',gc_operation_id=COALESCE(gc_operation_id,$2),next_attempt_at=clock_timestamp() WHERE blob_id=$1 AND state='LIVE'`, imp.FileID, uuid.New()); e != nil {
				return e
			}
		}
		if e = tx.Commit(ctx); e != nil {
			return e
		}
		return ErrCatalogActorClosed
	}
	tag, e := tx.Exec(ctx, `UPDATE file_catalog_import_operations SET phase='PROCESSING' WHERE key_version=$1 AND operation_hmac=$2 AND generation=$3 AND file_id=$4 AND phase='WRITE_STARTED'`, imp.KeyVersion, imp.OperationHMAC, imp.Generation, imp.FileID)
	if e != nil {
		return e
	}
	if tag.RowsAffected() != 1 {
		return ErrCatalogTerminal
	}
	if _, e = tx.Exec(ctx, `UPDATE files SET status='processing',updated_at=clock_timestamp() WHERE id=$1`, imp.FileID); e != nil {
		return e
	}
	if _, e = tx.Exec(ctx, `INSERT INTO file_gif_jobs(file_id,processor_version) VALUES($1,$2)`, imp.FileID, GIFProcessorVersion); e != nil {
		return e
	}
	after := before
	after.Status = "processing"
	if e = recordPreviewChange(ctx, tx, before, after); e != nil {
		return e
	}
	return tx.Commit(ctx)
}

func catalogFailMeasuredFile(ctx context.Context, tx pgx.Tx, before FileRow) error {
	if _, e := tx.Exec(ctx, `UPDATE files SET status='failed',updated_at=clock_timestamp() WHERE id=$1`, before.ID); e != nil {
		return e
	}
	after := before
	after.Status = "failed"
	return recordPreviewChange(ctx, tx, before, after)
}

// AbortCatalogPrepared retires measured bytes which have not crossed the PUT
// boundary. File and declaration locks plus the terminal operation fence stop
// any delayed Start from turning a proven DECLARED absence into a future PUT.
func (s *FilesStore) AbortCatalogPrepared(ctx context.Context, imp CatalogImport, keys *catalogasset.ReplayKeys) error {
	tx, state, _, e := s.catalogWriteTx(ctx, imp, keys)
	if e != nil {
		return e
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if state != "pending_upload" && state != "failed" {
		return ErrCatalogTerminal
	}
	var write string
	if e = tx.QueryRow(ctx, `SELECT state FROM file_catalog_writes WHERE file_id=$1 AND generation=$2 FOR UPDATE`, imp.FileID, imp.Generation).Scan(&write); e != nil {
		return e
	}
	if write != "DECLARED" {
		return ErrCatalogTerminal
	}
	if state == "pending_upload" {
		before, e := getFileByID(ctx, tx, imp.FileID, "")
		if e != nil {
			return e
		}
		if e = catalogFailMeasuredFile(ctx, tx, before); e != nil {
			return e
		}
	}
	source, e := keys.Source(imp.Actor, imp.SourceID)
	if e != nil {
		return e
	}
	asset, e := keys.Asset(imp.Actor, imp.SourceID, imp.Provider, imp.ProviderID, imp.FileID)
	if e != nil {
		return e
	}
	for _, fence := range []struct {
		purpose string
		digest  catalogasset.ReplayDigest
	}{{"source", source[0]}, {"asset", asset[0]}} {
		if _, e = tx.Exec(ctx, `INSERT INTO file_catalog_terminal_fences(purpose,key_version,identity_hmac,outcome,fence_id) VALUES($1,$2,$3,'failed',$4) ON CONFLICT(purpose,key_version,identity_hmac) DO NOTHING`, fence.purpose, fence.digest.KeyVersion, fence.digest.HMAC, uuid.New()); e != nil {
			return e
		}
	}
	if _, e = tx.Exec(ctx, `DELETE FROM file_catalog_assets WHERE file_id=$1`, imp.FileID); e != nil {
		return e
	}
	var referenced bool
	if e = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM file_references WHERE file_id=$1 AND released_at IS NULL)`, imp.FileID).Scan(&referenced); e != nil {
		return e
	}
	next := "GC_PENDING"
	if referenced {
		next = "DONE"
	}
	if _, e = tx.Exec(ctx, `UPDATE file_catalog_writes SET state=$3 WHERE file_id=$1 AND generation=$2`, imp.FileID, imp.Generation, next); e != nil {
		return e
	}
	if !referenced {
		if _, e = tx.Exec(ctx, `UPDATE file_blobs SET state='GC_PENDING',gc_operation_id=COALESCE(gc_operation_id,$2),next_attempt_at=clock_timestamp() WHERE blob_id=$1 AND state='LIVE'`, imp.FileID, uuid.New()); e != nil {
			return e
		}
	}
	return tx.Commit(ctx)
}
