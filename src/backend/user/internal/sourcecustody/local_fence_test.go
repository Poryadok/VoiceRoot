package sourcecustody

import (
	"context"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"voice/backend/pkg/integrationtest"
)

func TestLocalSourceRestrictionDoesNotNeedControlDatabase(t *testing.T) {
	ctx := context.Background()
	admin := custodyDatabase(t, ctx)
	_, err := admin.Exec(ctx, `CREATE ROLE local_fence_writer LOGIN PASSWORD 'fixture_writer'`)
	require.NoError(t, err)
	_, err = admin.Exec(ctx, `CREATE ROLE local_fence_super LOGIN SUPERUSER`)
	require.NoError(t, err)
	_, err = admin.Exec(ctx, `CREATE DATABASE local_fence_source`)
	require.NoError(t, err)
	var source PhysicalSource
	require.NoError(t, admin.QueryRow(ctx, `SELECT system_identifier::text FROM pg_control_system()`).Scan(&source.SystemIdentifier))
	source.Database = "local_fence_source"
	require.NoError(t, admin.QueryRow(ctx, `SELECT oid FROM pg_database WHERE datname=$1`, source.Database).Scan(&source.DatabaseOID))
	setupCfg := admin.Config().Copy()
	setupCfg.ConnConfig.Database = source.Database
	setup, err := pgxpool.NewWithConfig(ctx, setupCfg)
	require.NoError(t, err)
	_, err = setup.Exec(ctx, `CREATE TABLE local_fence_data(value integer); GRANT INSERT ON local_fence_data TO local_fence_writer`)
	require.NoError(t, err)
	require.Error(t, restrictLocalSource(ctx, setup, source, "local_fence_writer"), "maintenance must be outside the target database")
	setup.Close()
	cfg := admin.Config().Copy()
	cfg.ConnConfig.Database = source.Database
	cfg.ConnConfig.User = "local_fence_writer"
	cfg.ConnConfig.Password = "fixture_writer"
	writer, err := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(writer.Close)
	conn, err := writer.Acquire(ctx)
	require.NoError(t, err)
	defer conn.Release()
	tx, err := conn.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = tx.Exec(ctx, `INSERT INTO local_fence_data VALUES (1)`)
	require.NoError(t, err)
	wrong := source
	wrong.DatabaseOID++
	require.Error(t, restrictLocalSource(ctx, admin, wrong, "local_fence_writer"))
	wrong = source
	wrong.SystemIdentifier = "1"
	if wrong.SystemIdentifier == source.SystemIdentifier {
		wrong.SystemIdentifier = "2"
	}
	require.Error(t, restrictLocalSource(ctx, admin, wrong, "local_fence_writer"))
	require.Error(t, restrictLocalSource(ctx, admin, source, "custody_test"), "the maintenance superuser cannot be its own writer")
	require.Error(t, restrictLocalSource(ctx, admin, source, "local_fence_super"), "a distinct superuser cannot be the writer")
	var allowed, login bool
	require.NoError(t, admin.QueryRow(ctx, `SELECT datallowconn FROM pg_database WHERE oid=$1`, source.DatabaseOID).Scan(&allowed))
	require.NoError(t, admin.QueryRow(ctx, `SELECT rolcanlogin FROM pg_roles WHERE rolname='local_fence_writer'`).Scan(&login))
	require.True(t, allowed)
	require.True(t, login)
	bounded, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	require.NoError(t, restrictLocalSource(bounded, admin, source, "local_fence_writer"))
	require.Error(t, tx.Commit(ctx), "the held writer transaction must be terminated")
	require.NoError(t, admin.QueryRow(ctx, `SELECT datallowconn FROM pg_database WHERE oid=$1`, source.DatabaseOID).Scan(&allowed))
	require.NoError(t, admin.QueryRow(ctx, `SELECT rolcanlogin FROM pg_roles WHERE rolname='local_fence_writer'`).Scan(&login))
	require.False(t, allowed)
	require.False(t, login)
	require.Error(t, writer.Ping(ctx))
	// Quarantined test inspection: the transaction had not sent COMMIT when
	// the backend was terminated. No claim is made about an ambiguous COMMIT.
	_, err = admin.Exec(ctx, `ALTER DATABASE local_fence_source ALLOW_CONNECTIONS true`)
	require.NoError(t, err)
	inspect, err := pgxpool.NewWithConfig(ctx, setupCfg)
	require.NoError(t, err)
	var count int
	require.NoError(t, inspect.QueryRow(ctx, `SELECT count(*) FROM local_fence_data`).Scan(&count))
	inspect.Close()
	require.Zero(t, count)
	_, err = admin.Exec(ctx, `ALTER DATABASE local_fence_source ALLOW_CONNECTIONS false`)
	require.NoError(t, err)
}

func TestLocalSourceRestrictionLeavesPreparedTransactionsClosed(t *testing.T) {
	if testing.Short() {
		t.Skip("requires isolated PostgreSQL")
	}
	ctx := context.Background()
	integrationtest.ConfigureDockerTesting()
	pg, err := postgres.Run(ctx, integrationtest.PostgresImage, postgres.BasicWaitStrategies(), postgres.WithDatabase("local_prepared"), postgres.WithUsername("custody_test"), postgres.WithPassword("fixture_only"), testcontainers.WithConfigModifier(func(cfg *container.Config) {
		cfg.Cmd = []string{"postgres", "-c", "max_prepared_transactions=10"}
	}))
	if pg != nil {
		t.Cleanup(func() {
			cleanup, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			require.NoError(t, pg.Terminate(cleanup))
		})
	}
	require.NoError(t, err)
	dsn, err := pg.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	cfg, err := pgxpool.ParseConfig(dsn)
	require.NoError(t, err)
	sourcePool, err := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(sourcePool.Close)
	adminCfg := cfg.Copy()
	adminCfg.ConnConfig.Database = "postgres"
	admin, err := pgxpool.NewWithConfig(ctx, adminCfg)
	require.NoError(t, err)
	t.Cleanup(admin.Close)
	_, err = admin.Exec(ctx, `CREATE ROLE local_prepared_writer LOGIN`)
	require.NoError(t, err)
	_, err = sourcePool.Exec(ctx, `CREATE TABLE local_prepared_data(value integer)`)
	require.NoError(t, err)
	_, err = sourcePool.Exec(ctx, `BEGIN; INSERT INTO local_prepared_data VALUES(1); PREPARE TRANSACTION 'local_fence_fixture'`)
	require.NoError(t, err)
	source := PhysicalSource{Database: "local_prepared"}
	require.NoError(t, admin.QueryRow(ctx, `SELECT system_identifier::text FROM pg_control_system()`).Scan(&source.SystemIdentifier))
	require.NoError(t, admin.QueryRow(ctx, `SELECT oid FROM pg_database WHERE datname=$1`, source.Database).Scan(&source.DatabaseOID))
	require.Error(t, restrictLocalSource(ctx, admin, source, "local_prepared_writer"))
	var allowed, login bool
	require.NoError(t, admin.QueryRow(ctx, `SELECT datallowconn FROM pg_database WHERE oid=$1`, source.DatabaseOID).Scan(&allowed))
	require.NoError(t, admin.QueryRow(ctx, `SELECT rolcanlogin FROM pg_roles WHERE rolname='local_prepared_writer'`).Scan(&login))
	require.False(t, allowed)
	require.False(t, login)
}
