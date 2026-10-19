package spacegatetest

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"voice/backend/pkg/integrationtest"
)

// StartPostgres keeps the normal Testcontainers fixture, with an explicit opt-in
// for isolated Linux runners that already own a PostgreSQL server. Each test
// still gets a separate database and executes the original migrations/SQL.
func StartPostgres(t *testing.T, ctx context.Context, name, migration string) *pgxpool.Pool {
	t.Helper()
	if testing.Short() {
		t.Skip("PostgreSQL integration")
	}
	dsn := os.Getenv("VOICE_COMPOSITION_TEST_DATABASE_URL")
	if dsn == "" {
		return integrationtest.StartPostgres(t, ctx, name, migration)
	}
	admin, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(admin.Close)
	database := "voice_test_" + uuid.New().String()[:8]
	_, err = admin.Exec(ctx, `CREATE DATABASE `+database)
	require.NoError(t, err)
	config := admin.Config()
	config.ConnConfig.Database = database
	pool, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	t.Cleanup(func() {
		pool.Close()
		cleanup, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, err := admin.Exec(cleanup, `DROP DATABASE `+database+` WITH (FORCE)`)
		require.NoError(t, err)
	})
	if migration != "" {
		body, err := os.ReadFile(migration)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(body))
		require.NoError(t, err)
	}
	return pool
}
