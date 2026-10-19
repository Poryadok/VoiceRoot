package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// Exact transaction-only fixture extracted from de4512b36e60's
// native_membership_readiness_test.go. This acquires a real fence; it neither
// initializes native readiness nor grants any serving permission.
func nativeReadinessTx(ctx context.Context, pool *pgxpool.Pool, space uuid.UUID, fn func(pgx.Tx) error) error {
	tx, e := pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	if _, e = tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, spaceMutationAdvisoryKey(space)); e != nil {
		return e
	}
	if e = fn(tx); e != nil {
		return e
	}
	return tx.Commit(ctx)
}

// Exact nonowner-login fixture from the same source. The production SQL guard
// independently validates session_user; this helper cannot initialize readiness.
func nativeReadinessLogin(t *testing.T, ctx context.Context, admin *pgxpool.Pool) *pgxpool.Pool {
	t.Helper()
	role := "readiness_" + uuid.New().String()[:8]
	_, err := admin.Exec(ctx, "CREATE ROLE "+pgx.Identifier{role}.Sanitize()+" LOGIN PASSWORD 'isolated-fixture-only'")
	require.NoError(t, err)
	_, err = admin.Exec(ctx, "GRANT USAGE ON SCHEMA public TO "+pgx.Identifier{role}.Sanitize()+"; GRANT SELECT,INSERT,UPDATE,DELETE ON ALL TABLES IN SCHEMA public TO "+pgx.Identifier{role}.Sanitize())
	require.NoError(t, err)
	cfg := admin.Config().Copy()
	cfg.ConnConfig.User = role
	cfg.ConnConfig.Password = "isolated-fixture-only"
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(func() {
		pool.Close()
		_, _ = admin.Exec(context.Background(), "DROP OWNED BY "+pgx.Identifier{role}.Sanitize())
		_, _ = admin.Exec(context.Background(), "DROP ROLE "+pgx.Identifier{role}.Sanitize())
	})
	var actual string
	require.NoError(t, pool.QueryRow(ctx, "SELECT session_user").Scan(&actual))
	require.Equal(t, role, actual)
	return pool
}
