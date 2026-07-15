package integrationtest

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// UserDBMigrationFiles lists user_db migrations in apply order.
var UserDBMigrationFiles = []string{
	"000001_init.up.sql",
	"000002_privacy_settings.up.sql",
	"000003_profile_subscription.up.sql",
	"000004_profiles_verification.up.sql",
	"000005_privacy_guest_audience.up.sql",
	"000006_privacy_audience.up.sql",
	"000007_profile_accent_color.up.sql",
}

// ApplyUserDBMigrations runs all user_db *.up.sql migrations in order.
func ApplyUserDBMigrations(t *testing.T, ctx context.Context, pool *pgxpool.Pool, repoRoot string) {
	t.Helper()
	for _, name := range UserDBMigrationFiles {
		path := filepath.Join(repoRoot, "src", "backend", "migrations", "user_db", name)
		sqlBytes, err := os.ReadFile(path)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(sqlBytes))
		require.NoError(t, err)
	}
}
