package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const lifecycleAdvisoryLock int64 = 0x5345415243485232
const lifecycleSpaceLockNamespace int32 = 0x53454152

// AcquireLifecycleSpaceLock serializes lifecycle generation changes and
// retention compaction for one Space. It is transaction-scoped by PostgreSQL.
func AcquireLifecycleSpaceLock(ctx context.Context, tx pgx.Tx, spaceID uuid.UUID) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1::integer,hashtext($2::text))`, lifecycleSpaceLockNamespace, spaceID.String())
	return err
}

func beginGovernedTx(ctx context.Context, pool *pgxpool.Pool) (pgx.Tx, error) {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock_shared($1)`, lifecycleAdvisoryLock); err != nil {
		_ = tx.Rollback(ctx)
		return nil, err
	}
	ready, err := lifecycleSchemaReady(ctx, tx)
	if err != nil || !ready {
		_ = tx.Rollback(ctx)
		if err != nil {
			return nil, err
		}
		return nil, errors.New("search lifecycle schema unavailable")
	}
	return tx, nil
}

func lifecycleFrozenSpace(ctx context.Context, tx pgx.Tx, spaceID uuid.UUID) (bool, error) {
	var found bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM search_space_lifecycle_fences WHERE space_id=$1 AND state IN ('FROZEN','PURGE_DECIDED','PURGED'))`, spaceID).Scan(&found)
	return found, err
}

func lifecycleFrozenChat(ctx context.Context, tx pgx.Tx, chatID uuid.UUID) (bool, error) {
	var found bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM search_space_chat_manifest_items p JOIN search_space_lifecycle_fences f ON f.space_id=p.space_id AND f.deletion_operation_id=p.deletion_operation_id WHERE p.chat_id=$1 AND f.state IN ('FROZEN','PURGE_DECIDED','PURGED') UNION ALL SELECT 1 FROM search_space_purged_chat_fences WHERE chat_id=$1)`, chatID).Scan(&found)
	return found, err
}

func lifecycleGateSpace(ctx context.Context, tx pgx.Tx, id uuid.UUID) error {
	blocked, err := lifecycleFrozenSpace(ctx, tx, id)
	if err != nil {
		return err
	}
	if blocked {
		return fmt.Errorf("space projection frozen")
	}
	return nil
}
func lifecycleGateChat(ctx context.Context, tx pgx.Tx, id uuid.UUID) error {
	blocked, err := lifecycleFrozenChat(ctx, tx, id)
	if err != nil {
		return err
	}
	if blocked {
		return fmt.Errorf("chat projection frozen")
	}
	return nil
}

func lifecycleGateAnySpaceSearch(ctx context.Context, tx pgx.Tx, query string) error {
	var blocked bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM space_search_documents s JOIN search_space_lifecycle_fences f ON f.space_id=s.space_id WHERE f.state IN ('FROZEN','PURGE_DECIDED','PURGED') AND (s.name ILIKE '%' || $1 || '%' OR s.description ILIKE '%' || $1 || '%'))`, query).Scan(&blocked)
	if err != nil {
		return err
	}
	if blocked {
		return fmt.Errorf("space projection frozen")
	}
	return nil
}

type queryRower interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func lifecycleSchemaReady(ctx context.Context, db queryRower) (bool, error) {
	var ready bool
	err := db.QueryRow(ctx, `SELECT to_regclass('search_space_lifecycle_fences') IS NOT NULL AND to_regclass('search_space_lifecycle_operations') IS NOT NULL AND to_regclass('search_space_lifecycle_receipts') IS NOT NULL AND to_regclass('search_space_purge_receipts') IS NOT NULL AND to_regclass('search_space_chat_manifests') IS NOT NULL AND to_regclass('search_space_chat_manifest_pages') IS NOT NULL AND to_regclass('search_space_chat_manifest_items') IS NOT NULL AND to_regclass('search_space_purged_chat_fences') IS NOT NULL`).Scan(&ready)
	return ready, err
}
