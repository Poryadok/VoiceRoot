package integrationtest

import (
	"context"
	"fmt"
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
	PostgresImage          = "postgres:16-bookworm"
	postgresUser           = "u"
	postgresPass           = "p"
	postgresCleanupTimeout = 30 * time.Second
)

var postgresRun = postgres.Run

var postgresTerminate = func(ctx context.Context, container *postgres.PostgresContainer) error {
	if container == nil {
		return nil
	}
	return container.Terminate(ctx)
}

func startPostgresContainer(ctx context.Context, dbName string) (*postgres.PostgresContainer, error) {
	container, err := postgresRun(ctx, PostgresImage,
		postgres.BasicWaitStrategies(),
		postgres.WithDatabase(dbName),
		postgres.WithUsername(postgresUser),
		postgres.WithPassword(postgresPass),
	)
	if err != nil && container != nil {
		if cleanupErr := terminatePostgresContainer(container); cleanupErr != nil {
			err = fmt.Errorf("start postgres container: %w; terminate partial container: %v", err, cleanupErr)
		}
	}
	return container, err
}

// terminatePostgresContainer does not inherit a test operation context: setup
// may already have expired when a partially started container needs cleanup.
func terminatePostgresContainer(container *postgres.PostgresContainer) error {
	cleanupCtx, cancel := context.WithTimeout(context.Background(), postgresCleanupTimeout)
	defer cancel()
	return postgresTerminate(cleanupCtx, container)
}

// StartPostgres runs a Postgres testcontainer, waits until the DB accepts connections,
// optionally applies a single migration SQL file, and returns a pgx pool with t.Cleanup hooks.
// migrationSQLPath is empty to skip migration.
func StartPostgres(t *testing.T, ctx context.Context, dbName, migrationSQLPath string) *pgxpool.Pool {
	t.Helper()
	if testing.Short() {
		t.Skip("integration test skipped in -short mode")
	}

	pgC, err := startPostgresContainer(ctx, dbName)
	require.NoError(t, err)
	var pool *pgxpool.Pool
	// Test cleanup must not inherit a cancelled operation context. Terminating
	// the container first releases any checked-out pool connections; both cleanup
	// paths are deliberately bounded so a failed test cannot consume go test's
	// package-level timeout while unwinding.
	t.Cleanup(func() {
		if cleanupErr := terminatePostgresContainer(pgC); cleanupErr != nil {
			t.Errorf("terminate postgres testcontainer: %v", cleanupErr)
		}
		if pool == nil {
			return
		}
		closed := make(chan struct{})
		go func() {
			pool.Close()
			close(closed)
		}()
		select {
		case <-closed:
		case <-time.After(postgresCleanupTimeout):
			t.Errorf("close postgres pool exceeded %s", postgresCleanupTimeout)
		}
	})

	connStr, err := pgC.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	connStr = strings.Replace(connStr, "localhost", "127.0.0.1", 1)
	connStr = strings.Replace(connStr, "[::1]", "127.0.0.1", 1)

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
	if migrationSQLPath != "" {
		sqlBytes, err := os.ReadFile(migrationSQLPath)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(sqlBytes))
		require.NoError(t, err)
	}

	return pool
}
