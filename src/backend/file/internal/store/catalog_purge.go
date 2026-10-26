package store

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"voice/backend/file/internal/catalogasset"
)

// PurgeCatalogProfile closes the local publication gate before erasing private
// source/operation details. It does not release committed references, reopen an
// actor on restore, or assert that outstanding object I/O has been fenced.
// The caller must establish deletion using the protected actor authority;
// UNKNOWN or a failed authority request is not deletion evidence.
func (s *FilesStore) PurgeCatalogProfile(ctx context.Context, actor catalogasset.ActorPair, keys *catalogasset.ReplayKeys) error {
	digests, err := keys.Profile(actor)
	if err != nil {
		return err
	}
	// Commit closure separately: subsequent erasure failure still leaves all late
	// READY publishers denied. Never hold gate locks while waiting for File locks
	// (the READY trigger takes those locks in the opposite direction).
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err = catalogActorLock(ctx, tx, actor, keys); err != nil {
		return err
	}
	for _, d := range digests {
		_, err = tx.Exec(ctx, `INSERT INTO file_catalog_actor_gates(id,actor_kind,key_version,actor_hmac,generation,state)
VALUES($1,'profile',$2,$3,$4,'CLOSED')
ON CONFLICT(actor_kind,key_version,actor_hmac) DO UPDATE SET state='CLOSED',
generation=CASE WHEN file_catalog_actor_gates.state='LIVE' THEN EXCLUDED.generation ELSE file_catalog_actor_gates.generation END`, uuid.New(), d.KeyVersion, d.HMAC, uuid.New())
		if err != nil {
			return err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}

	tx, err = s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err = catalogActorLock(ctx, tx, actor, keys); err != nil {
		return err
	}
	rows, err := tx.Query(ctx, `SELECT file_id,source_id,provider,provider_id FROM file_catalog_assets
WHERE actor_account_id=$1 AND actor_profile_id=$2 ORDER BY file_id`, actor.AccountID, actor.ProfileID)
	if err != nil {
		return err
	}
	type asset struct {
		file, source         uuid.UUID
		provider, providerID string
	}
	var assets []asset
	for rows.Next() {
		var a asset
		if err = rows.Scan(&a.file, &a.source, &a.provider, &a.providerID); err != nil {
			rows.Close()
			return err
		}
		assets = append(assets, a)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, a := range assets {
		source, e := keys.Source(actor, a.source)
		if e != nil {
			return e
		}
		binding, e := keys.Asset(actor, a.source, a.provider, a.providerID, a.file)
		if e != nil {
			return e
		}
		if err = catalogTerminalFence(ctx, tx, "source", source[0]); err != nil {
			return err
		}
		if err = catalogTerminalFence(ctx, tx, "asset", binding[0]); err != nil {
			return err
		}
		var size int64
		var state string
		if err = tx.QueryRow(ctx, `SELECT size_bytes,status FROM files WHERE id=$1 FOR UPDATE`, a.file).Scan(&size, &state); err != nil {
			return err
		}
		if size > 0 && (state == "pending_upload" || state == "processing") {
			before, e := getFileByID(ctx, tx, a.file, "")
			if e != nil {
				return e
			}
			if _, err = tx.Exec(ctx, `UPDATE files SET status='failed',updated_at=clock_timestamp() WHERE id=$1`, a.file); err != nil {
				return err
			}
			after := before
			after.Status = "failed"
			if err = recordPreviewChange(ctx, tx, before, after); err != nil {
				return err
			}
		}
	}
	rows, err = tx.Query(ctx, `SELECT key_version,operation_hmac FROM file_catalog_import_operations WHERE actor_account_id=$1 AND actor_profile_id=$2 FOR UPDATE`, actor.AccountID, actor.ProfileID)
	if err != nil {
		return err
	}
	var operations []catalogasset.ReplayDigest
	for rows.Next() {
		var d catalogasset.ReplayDigest
		if err = rows.Scan(&d.KeyVersion, &d.HMAC); err != nil {
			rows.Close()
			return err
		}
		operations = append(operations, d)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, d := range operations {
		if err = catalogTerminalFence(ctx, tx, "operation", d); err != nil {
			return err
		}
	}
	if _, err = tx.Exec(ctx, `DELETE FROM file_catalog_import_operations WHERE actor_account_id=$1 AND actor_profile_id=$2`, actor.AccountID, actor.ProfileID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM file_catalog_assets WHERE actor_account_id=$1 AND actor_profile_id=$2`, actor.AccountID, actor.ProfileID); err != nil {
		return err
	}
	for _, a := range assets {
		// An internal unknown-size row has never reached the measured-facts or
		// object-write stage. Remove it only when the durable write declaration
		// and reference tables independently confirm that there is no work to
		// drain. The blob ledger survives File deletion for ordinary GC.
		var blob uuid.UUID
		err = tx.QueryRow(ctx, `DELETE FROM files f WHERE f.id=$1 AND f.catalog_asset
AND f.catalog_source_size_unknown AND f.size_bytes=0
AND NOT EXISTS(SELECT 1 FROM file_catalog_writes w WHERE w.file_id=f.id)
AND NOT EXISTS(SELECT 1 FROM file_references r WHERE r.file_id=f.id)
RETURNING blob_id`, a.file).Scan(&blob)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `UPDATE file_blobs SET state='GC_PENDING',gc_operation_id=COALESCE(gc_operation_id,$2),next_attempt_at=clock_timestamp() WHERE blob_id=$1 AND state='LIVE'`, blob, uuid.New()); err != nil {
			return err
		}
	}
	// No measured-blob GC or write-state transition here. STARTED/UNSETTLED writes need
	// actual runtime drain/storage fencing; elapsed time and local gate closure
	// cannot prove storage quiescence. Measured File rows and any declared writes
	// remain unsendable until the separately owned reconciliation consumer handles them.
	return tx.Commit(ctx)
}

func catalogActorLock(ctx context.Context, tx pgx.Tx, actor catalogasset.ActorPair, keys *catalogasset.ReplayKeys) error {
	// Stable across key rotation. This lock serializes create/restore/purge for
	// an account; it is not persisted or emitted as identity/analytics data.
	hash := sha256.Sum256(append([]byte("file.catalog.account.lock.v1:"), actor.AccountID[:]...))
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, int64(binary.BigEndian.Uint64(hash[:8]))); err != nil {
		return err
	}
	rows, err := tx.Query(ctx, `SELECT key_version FROM file_catalog_actor_gates UNION SELECT key_version FROM file_catalog_import_operations UNION SELECT key_version FROM file_catalog_terminal_fences UNION SELECT key_version FROM file_catalog_selection_heads`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var version string
		if err = rows.Scan(&version); err != nil {
			return err
		}
		if !keys.HasVersion(version) {
			return catalogasset.ErrReplayKeys
		}
	}
	return rows.Err()
}

func catalogTerminalFence(ctx context.Context, tx pgx.Tx, purpose string, d catalogasset.ReplayDigest) error {
	if len(d.HMAC) != 32 || d.KeyVersion == "" {
		return errors.New("invalid catalog terminal fence")
	}
	_, err := tx.Exec(ctx, `INSERT INTO file_catalog_terminal_fences(purpose,key_version,identity_hmac,outcome,fence_id) VALUES($1,$2,$3,'deleted',$4) ON CONFLICT(purpose,key_version,identity_hmac) DO NOTHING`, purpose, d.KeyVersion, d.HMAC, uuid.New())
	return err
}
