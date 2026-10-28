// Package testdb owns bounded disposable PostgreSQL fixtures for Bot tests.
package testdb

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

// StartPostgres limits the database to 0.5 CPU/512 MiB. Run one test case at a
// time with a 1.5 CPU/2.5 GiB Go container for a combined 2 CPU/3 GiB budget.
func StartPostgres(t *testing.T, ctx context.Context, name, migration string) *pgxpool.Pool {
	t.Helper()
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	startup, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	pg, err := postgres.Run(startup, "postgres:16-bookworm", postgres.BasicWaitStrategies(), postgres.WithDatabase(name), postgres.WithUsername("u"), postgres.WithPassword("p"), testcontainers.WithHostConfigModifier(func(config *container.HostConfig) { config.Memory = 512 * 1024 * 1024; config.NanoCPUs = 500_000_000 }))
	var pool *pgxpool.Pool
	t.Cleanup(func() {
		cleanup, done := context.WithTimeout(context.Background(), 30*time.Second)
		defer done()
		if pg != nil {
			if e := pg.Terminate(cleanup); e != nil {
				t.Errorf("terminate owned Postgres: %v", e)
			}
		}
		if pool != nil {
			closed := make(chan struct{})
			go func() { pool.Close(); close(closed) }()
			select {
			case <-closed:
			case <-cleanup.Done():
				t.Errorf("owned Postgres pool cleanup: %v", cleanup.Err())
			}
		}
	})
	require.NoError(t, err)
	actual, err := pg.Inspect(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(512*1024*1024), actual.HostConfig.Memory, "owned PG memory cap")
	require.Equal(t, int64(500_000_000), actual.HostConfig.NanoCPUs, "owned PG CPU cap")
	dsn, err := pg.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	pool, err = pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	require.NoError(t, pool.Ping(ctx))
	if migration != "" {
		raw, e := os.ReadFile(migration)
		require.NoError(t, e)
		_, e = pool.Exec(ctx, string(raw))
		require.NoError(t, e)
	}
	return pool
}
