package store

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var ErrTrackedWrite = errors.New("tracked object write unavailable")

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
