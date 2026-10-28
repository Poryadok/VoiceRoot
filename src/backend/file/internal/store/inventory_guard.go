package store

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrInventoryBarrier = fmt.Errorf("%w: reference inventory migration", ErrFileRetained)

// LockInventoryGuardTx is the FIRST lock of overlapping File transactions.
// FOR SHARE conflicts with cohort creation/invalidation. No RPC may run while
// this guard is held. Missing schema only preserves pre-migration operation;
// the importer requires the actual schema and singleton row.
func LockInventoryGuardTx(ctx context.Context, tx pgx.Tx, destructive bool) (int64, error) {
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT to_regclass('file_reference_inventory_gate') IS NOT NULL`).Scan(&exists); err != nil {
		return 0, err
	}
	if !exists {
		return 0, nil
	}
	var blocked bool
	var generation int64
	if err := tx.QueryRow(ctx, `SELECT blocked,generation FROM file_reference_inventory_gate WHERE singleton FOR SHARE`).Scan(&blocked, &generation); err != nil {
		return 0, err
	}
	if destructive && blocked {
		return generation, ErrInventoryBarrier
	}
	return generation, nil
}

func BeginInventoryMutation(ctx context.Context, pool *pgxpool.Pool, destructive bool) (pgx.Tx, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	if _, err = LockInventoryGuardTx(ctx, tx, destructive); err != nil {
		_ = tx.Rollback(context.Background())
		return nil, err
	}
	return tx, nil
}

func inventoryBlobHeldTx(ctx context.Context, tx pgx.Tx, blob uuid.UUID) (bool, error) {
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT to_regclass('file_reference_inventory_holds') IS NOT NULL`).Scan(&exists); err != nil {
		return true, err
	}
	if !exists {
		return false, nil
	}
	var held bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM file_reference_inventory_holds WHERE blob_id=$1)`, blob).Scan(&held)
	return held, err
}
