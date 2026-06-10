package testutil

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
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", ".."))
}

// StartRoleDB boots Postgres for role_db integration tests.
func StartRoleDB(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()
	return integrationtest.StartPostgres(t, ctx, "roledb", "")
}

// ApplyRoleMigrations applies role_db DDL through the latest migration.
func ApplyRoleMigrations(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	dir := filepath.Join(repoRoot(t), "src", "backend", "migrations", "role_db")
	for _, name := range []string{"000001_init.up.sql", "000002_moderation_permissions.up.sql"} {
		sqlBytes, err := os.ReadFile(filepath.Join(dir, name))
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(sqlBytes))
		require.NoError(t, err)
	}
}
