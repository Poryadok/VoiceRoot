package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTerminalRoutingMigrationRollbackPreservesEvidence(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	pool := StartMatchmakingDBForStoreTest(t, ctx)
	ApplyMatchmakingMigrationsForStoreTest(t, ctx, pool)
	directory := filepath.Join(repoRoot(t), "src", "backend", "migrations", "matchmaking_db")
	down, err := os.ReadFile(filepath.Join(directory, "000020_terminal_routing_lifecycle.down.sql"))
	require.NoError(t, err)
	up, err := os.ReadFile(filepath.Join(directory, "000020_terminal_routing_lifecycle.up.sql"))
	require.NoError(t, err)
	// Empty rollback/re-upgrade is valid; a permanent lifecycle fence is not
	// disposable even when all raw terminal routing has already been erased.
	_, err = pool.Exec(ctx, string(down))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(up))
	require.NoError(t, err)
	writer, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = writer.Rollback(ctx) }()
	_, err = writer.Exec(ctx, `INSERT INTO matchmaking_terminal_routing_heads(fence_id,current_json) VALUES('concurrent','{}')`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(down))
	require.ErrorContains(t, err, "could not obtain lock", "DOWN must not race a writer")
	require.NoError(t, writer.Commit(ctx))
	_, err = pool.Exec(ctx, string(down))
	require.ErrorContains(t, err, "cannot remove terminal routing lifecycle evidence")
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM matchmaking_terminal_routing_heads WHERE fence_id='concurrent'`).Scan(&count))
	require.Equal(t, 1, count)
}
