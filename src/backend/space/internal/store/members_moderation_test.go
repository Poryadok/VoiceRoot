package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// TestSpaceModeration_Store_Migration_applies documents space_db moderation schema (space_bans, member timeouts).
func TestSpaceModeration_Store_Migration_applies(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSpacePostgresForStoreTest(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)

	var bansTable, timeoutsTable string
	err := pool.QueryRow(ctx, `
SELECT table_name FROM information_schema.tables
WHERE table_schema = 'public' AND table_name = 'space_bans'
`).Scan(&bansTable)
	require.NoError(t, err)
	require.Equal(t, "space_bans", bansTable)

	err = pool.QueryRow(ctx, `
SELECT table_name FROM information_schema.tables
WHERE table_schema = 'public' AND table_name = 'space_member_timeouts'
`).Scan(&timeoutsTable)
	require.NoError(t, err)
	require.Equal(t, "space_member_timeouts", timeoutsTable)
}

func applyModerationMigrationForStoreTest(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	migrationPath := filepath.Join(repoRoot(t), "src", "backend", "migrations", "space_db", "000004_moderation.up.sql")
	sqlBytes, err := os.ReadFile(migrationPath)
	require.NoError(t, err, "moderation migration must exist before store tests pass")
	_, err = pool.Exec(ctx, string(sqlBytes))
	require.NoError(t, err)
}
