package store

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"voice/backend/pkg/integrationtest"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", ".."))
}

func StartMatchmakingDBForStoreTest(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()
	return integrationtest.StartPostgres(t, ctx, "matchmakingdb", "")
}

func applyMatchmakingMigrationsUpTo(t *testing.T, ctx context.Context, pool *pgxpool.Pool, lastFile string) {
	t.Helper()
	root := repoRoot(t)
	for _, name := range []string{
		"000001_init.up.sql",
		"000002_profile_game_entries.up.sql",
		"000003_search_sessions.up.sql",
		"000004_matches.up.sql",
		"000005_ratings_and_bans.up.sql",
		"000006_search_nudge.up.sql",
		"000007_match_history_index.up.sql",
	} {
		migrationPath := filepath.Join(root, "src", "backend", "migrations", "matchmaking_db", name)
		sqlBytes, err := os.ReadFile(migrationPath)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(sqlBytes))
		require.NoError(t, err)
		if name == lastFile {
			return
		}
	}
	t.Fatalf("unknown migration file %q", lastFile)
}

func ApplyMatchmakingMigrationsForStoreTest(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	applyMatchmakingMigrationsUpTo(t, ctx, pool, "000007_match_history_index.up.sql")
}

func ApplyMatchmakingMigrationsThrough005ForStoreTest(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	applyMatchmakingMigrationsUpTo(t, ctx, pool, "000006_search_nudge.up.sql")
}
