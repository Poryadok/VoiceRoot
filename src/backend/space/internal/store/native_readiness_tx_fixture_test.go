package store

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Exact transaction-only fixture extracted from de4512b36e60's
// native_membership_readiness_test.go. This acquires a real fence; it neither
// initializes native readiness nor grants any serving permission.
func nativeReadinessTx(ctx context.Context, pool *pgxpool.Pool, space uuid.UUID, fn func(pgx.Tx) error) error {
	tx, e := pool.Begin(ctx)
	if e != nil { return e }
	defer tx.Rollback(ctx)
	if _, e = tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, spaceMutationAdvisoryKey(space)); e != nil { return e }
	if e = fn(tx); e != nil { return e }
	return tx.Commit(ctx)
}
