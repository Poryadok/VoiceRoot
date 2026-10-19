package threadlist

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// This opt-in isolated database fixture never starts a host listener. The
// caller provides an owned throwaway PostgreSQL database, with no shared stand.
func TestPostgresImmutableNodesAndRollback(t *testing.T) {
	dsn := os.Getenv("THREADLIST_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("requires an owned PostgreSQL THREADLIST_TEST_DATABASE_URL")
	}
	ctx := context.Background()
	config, err := pgxpool.ParseConfig(dsn)
	require.NoError(t, err)
	schema := "threadlist_" + uuid.NewString()
	config.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize())
	require.NoError(t, err)
	t.Cleanup(func() {
		defer pool.Close()
		_, dropErr := pool.Exec(context.Background(), "DROP SCHEMA "+pgx.Identifier{schema}.Sanitize()+" CASCADE")
		require.NoError(t, dropErr)
	})
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Join(filepath.Dir(file), "../../../migrations/messaging_db")
	up, err := os.ReadFile(filepath.Join(root, "000031_thread_list_nodes.up.sql"))
	require.NoError(t, err)
	down, err := os.ReadFile(filepath.Join(root, "000031_thread_list_nodes.down.sql"))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(up))
	require.NoError(t, err)
	entry := testEntry(1, 100)
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	idx, err := (Index{Store: PostgresNodes{DB: tx}}).Upsert(ctx, entry)
	require.NoError(t, err)
	roots := idx.Roots
	require.NoError(t, tx.Commit(ctx))
	idx = Index{Store: PostgresNodes{DB: pool}, Roots: roots}
	page, err := idx.Page(ctx, 5)
	require.NoError(t, err)
	require.Equal(t, []Entry{entry}, page)
	// Updates are rejected by SQL, independently of application immutability.
	_, err = pool.Exec(ctx, "UPDATE thread_list_nodes SET payload=payload WHERE node_id=$1", roots.Activity[:])
	require.ErrorContains(t, err, "immutable")
	var before int
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM thread_list_nodes").Scan(&before))
	tx, err = pool.Begin(ctx)
	require.NoError(t, err)
	working, err := (Index{Store: PostgresNodes{DB: tx}, Roots: roots}).Upsert(ctx, testEntry(2, 101))
	require.NoError(t, err)
	_, err = working.Delete(ctx, entry.ParentID)
	require.NoError(t, err)
	require.NoError(t, tx.Rollback(ctx))
	var after int
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM thread_list_nodes").Scan(&after))
	require.Equal(t, before, after)
	page, err = idx.Page(ctx, 5)
	require.NoError(t, err)
	require.Equal(t, []Entry{entry}, page)
	// Exact retries use the same content-addressed row; no duplicate writes.
	n, err := idx.Store.Get(ctx, roots.Activity)
	require.NoError(t, err)
	repeated, err := idx.Store.Put(ctx, n)
	require.NoError(t, err)
	require.Equal(t, roots.Activity, repeated)
	// Migration round trip is deliberately exercised in this disposable DB.
	_, err = pool.Exec(ctx, string(down))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(up))
	require.NoError(t, err)
}
