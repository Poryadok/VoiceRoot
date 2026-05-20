package integrationtest

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

const (
	// PostgresImage is the default Postgres image for integration tests.
	PostgresImage = "postgres:16-bookworm"
	postgresUser  = "u"
	postgresPass  = "p"
)

// StartPostgres runs a Postgres testcontainer, waits until the DB accepts connections,
// optionally applies a single migration SQL file, and returns a pgx pool with t.Cleanup hooks.
// migrationSQLPath is empty to skip migration.
func StartPostgres(t *testing.T, ctx context.Context, dbName, migrationSQLPath string) *pgxpool.Pool {
	t.Helper()

	pgC, err := postgres.Run(ctx, PostgresImage,
		postgres.BasicWaitStrategies(),
		postgres.WithDatabase(dbName),
		postgres.WithUsername(postgresUser),
		postgres.WithPassword(postgresPass),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pgC.Terminate(ctx) })

	connStr, err := pgC.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	connStr = strings.Replace(connStr, "localhost", "127.0.0.1", 1)
	connStr = strings.Replace(connStr, "[::1]", "127.0.0.1", 1)

	var pool *pgxpool.Pool
	for i := 0; i < 60; i++ {
		p, err := pgxpool.New(ctx, connStr)
		if err == nil {
			if pingErr := p.Ping(ctx); pingErr == nil {
				pool = p
				break
			}
			p.Close()
		}
		time.Sleep(500 * time.Millisecond)
	}
	require.NotNil(t, pool, "postgres did not become ready in time")
	t.Cleanup(pool.Close)

	if migrationSQLPath != "" {
		sqlBytes, err := os.ReadFile(migrationSQLPath)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(sqlBytes))
		require.NoError(t, err)
	}

	return pool
}
