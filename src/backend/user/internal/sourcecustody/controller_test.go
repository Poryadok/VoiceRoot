package sourcecustody

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"voice/backend/pkg/integrationtest"
)

// These owned databases and synthetic OPEN row exercise only a local locking
// kernel. They do not establish production source qualification or custody.
func custodyDatabase(t *testing.T, ctx context.Context) *pgxpool.Pool {
	return custodyDatabaseTraced(t, ctx, nil)
}

func custodyDatabaseTraced(t *testing.T, ctx context.Context, tracer pgx.QueryTracer) *pgxpool.Pool {
	t.Helper()
	if testing.Short() {
		t.Skip("requires two isolated PostgreSQL servers")
	}
	integrationtest.ConfigureDockerTesting()
	pg, err := postgres.Run(ctx, integrationtest.PostgresImage, postgres.BasicWaitStrategies(), postgres.WithDatabase("custody_test"), postgres.WithUsername("custody_test"), postgres.WithPassword("fixture_only"))
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
	cfg.ConnConfig.Tracer = tracer
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	require.NoError(t, pool.Ping(ctx))
	return pool
}

func custodyBinding() Binding {
	return Binding{Epoch: 1, SourceID: uuid.NewString(), HistoryGeneration: uuid.New(), UserSourceGeneration: uuid.New(), KeysetDigest: [32]byte{1}}
}

func custodyFixture(t *testing.T) (context.Context, *pgxpool.Pool, *pgxpool.Pool, Binding, *Controller) {
	return custodyFixtureWithFailure(t, false)
}

func custodyFixtureWithFailure(t *testing.T, expectFailure bool) (context.Context, *pgxpool.Pool, *pgxpool.Pool, Binding, *Controller) {
	return custodyFixtureTraced(t, expectFailure, nil)
}

func custodyFixtureTraced(t *testing.T, expectFailure bool, tracer pgx.QueryTracer) (context.Context, *pgxpool.Pool, *pgxpool.Pool, Binding, *Controller) {
	return custodyFixtureBound(t, expectFailure, tracer, nil, nil)
}

func custodyFixtureBound(t *testing.T, expectFailure bool, tracer pgx.QueryTracer, configured *Binding, failureOverride *bool) (context.Context, *pgxpool.Pool, *pgxpool.Pool, Binding, *Controller) {
	t.Helper()
	ctx := context.Background()
	control, user := custodyDatabaseTraced(t, ctx, tracer), custodyDatabase(t, ctx)
	_, err := control.Exec(ctx, `CREATE TABLE user_custody_state(singleton boolean PRIMARY KEY CHECK(singleton),state text NOT NULL,epoch bigint NOT NULL,source_id text NOT NULL,history_generation uuid NOT NULL,user_source_generation uuid NOT NULL,keyset_digest bytea NOT NULL)`)
	require.NoError(t, err)
	binding := custodyBinding()
	if configured != nil {
		binding = *configured
	}
	_, err = control.Exec(ctx, `INSERT INTO user_custody_state VALUES(true,'OPEN',$1,$2,$3,$4,$5)`, binding.Epoch, binding.SourceID, binding.HistoryGeneration, binding.UserSourceGeneration, binding.KeysetDigest[:])
	require.NoError(t, err)
	c, err := Start(ctx, control, binding)
	require.NoError(t, err)
	t.Cleanup(func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err := c.Shutdown(cleanup)
		if expectFailure || (failureOverride != nil && *failureOverride) {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
		}
	})
	var state string
	require.NoError(t, control.QueryRow(ctx, `SELECT state FROM user_custody_state WHERE singleton`).Scan(&state))
	require.Equal(t, "CLOSED", state, "every controller startup fences a stale OPEN")
	// This deliberately bypasses qualification for the bounded lock test only.
	_, err = control.Exec(ctx, `UPDATE user_custody_state SET state='OPEN' WHERE singleton`)
	require.NoError(t, err)
	return ctx, control, user, binding, c
}

func TestCustodyClosePersistsBeforePermitDrain(t *testing.T) {
	ctx, control, user, binding, c := custodyFixture(t)
	permit, err := c.Acquire(ctx, binding)
	require.NoError(t, err)
	require.NotNil(t, permit)
	released := false
	t.Cleanup(func() {
		if !released {
			cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = permit.Release(cleanup)
		}
	})
	for _, wrong := range []Binding{
		{},
		{Epoch: binding.Epoch + 1, SourceID: binding.SourceID, HistoryGeneration: binding.HistoryGeneration, UserSourceGeneration: binding.UserSourceGeneration, KeysetDigest: binding.KeysetDigest},
		{Epoch: binding.Epoch, SourceID: uuid.NewString(), HistoryGeneration: binding.HistoryGeneration, UserSourceGeneration: binding.UserSourceGeneration, KeysetDigest: binding.KeysetDigest},
		{Epoch: binding.Epoch, SourceID: binding.SourceID, HistoryGeneration: uuid.New(), UserSourceGeneration: binding.UserSourceGeneration, KeysetDigest: binding.KeysetDigest},
		{Epoch: binding.Epoch, SourceID: binding.SourceID, HistoryGeneration: binding.HistoryGeneration, UserSourceGeneration: uuid.New(), KeysetDigest: binding.KeysetDigest},
		{Epoch: binding.Epoch, SourceID: binding.SourceID, HistoryGeneration: binding.HistoryGeneration, UserSourceGeneration: binding.UserSourceGeneration, KeysetDigest: [32]byte{2}},
	} {
		unexpected, acquireErr := c.Acquire(ctx, wrong)
		if unexpected != nil {
			_ = unexpected.Release(ctx)
		}
		err = acquireErr
		require.Error(t, err)
	}
	tx, err := user.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = tx.Exec(ctx, `CREATE TABLE held_user_transaction(id integer)`)
	require.NoError(t, err)
	require.NoError(t, c.CloseAdmission(ctx))
	var state string
	require.NoError(t, control.QueryRow(ctx, `SELECT state FROM user_custody_state WHERE singleton`).Scan(&state))
	require.Equal(t, "CLOSED", state)
	_, err = c.Acquire(ctx, binding)
	require.Error(t, err, "no new admission after durable close")
	deadline, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()
	require.Error(t, c.WaitPermits(deadline), "active permit prevents drain")
	require.NoError(t, tx.Commit(ctx))
	require.NoError(t, permit.Release(ctx))
	released = true
	// A direct connection cannot reuse the cancelled waiter's pooled session:
	// PostgreSQL advisory locks are reentrant on the same session.
	probe, err := pgx.ConnectConfig(ctx, control.Config().ConnConfig.Copy())
	require.NoError(t, err)
	defer func() { _ = probe.Close(context.Background()) }()
	var outstanding int
	require.NoError(t, probe.QueryRow(ctx, `SELECT count(*) FROM pg_locks WHERE locktype='advisory' AND mode='ExclusiveLock' AND objsubid=1 AND classid=(6219270163691208962::bigint >> 32)::oid AND objid=(6219270163691208962::bigint & 4294967295)::oid`).Scan(&outstanding))
	require.Zero(t, outstanding, "cancelled waiter must not retain a granted or pending barrier")
	var locked bool
	require.NoError(t, probe.QueryRow(ctx, `SELECT pg_try_advisory_lock(6219270163691208962::bigint)`).Scan(&locked))
	require.True(t, locked, "cancelled drain must not leak a subsequently acquired exclusive lock")
	require.NoError(t, probe.QueryRow(ctx, `SELECT pg_advisory_unlock(6219270163691208962::bigint)`).Scan(&locked))
	require.True(t, locked)
	finished, finishCancel := context.WithTimeout(ctx, 5*time.Second)
	defer finishCancel()
	require.NoError(t, c.WaitPermits(finished))
}

func TestCustodyRestartClosesStaleOpen(t *testing.T) {
	ctx, control, _, binding, c := custodyFixture(t)
	require.NoError(t, c.Shutdown(ctx))
	_, err := control.Exec(ctx, `UPDATE user_custody_state SET state='OPEN' WHERE singleton`)
	require.NoError(t, err)
	restarted, err := Start(ctx, control, binding)
	require.NoError(t, err)
	t.Cleanup(func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		require.NoError(t, restarted.Shutdown(cleanup))
	})
	var state string
	require.NoError(t, control.QueryRow(ctx, `SELECT state FROM user_custody_state WHERE singleton`).Scan(&state))
	require.Equal(t, "CLOSED", state)
	_, err = restarted.Acquire(ctx, binding)
	require.Error(t, err)
}

func TestCustodyLostPermitDoesNotProveUserSessionDrained(t *testing.T) {
	ctx, control, user, binding, c := custodyFixtureWithFailure(t, true)
	permit, err := c.Acquire(ctx, binding)
	require.NoError(t, err)
	tx, err := user.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()
	var userPID int32
	require.NoError(t, tx.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&userPID))
	_, err = tx.Exec(ctx, `CREATE TABLE held_after_lost_permit(id integer)`)
	require.NoError(t, err)
	// Match the exact bigint advisory key within this owned control server.
	var permitPID int32
	require.NoError(t, control.QueryRow(ctx, `SELECT min(pid) FROM pg_locks WHERE locktype='advisory' AND mode='ShareLock' AND granted AND objsubid=1 AND classid=(6219270163691208962::bigint >> 32)::oid AND objid=(6219270163691208962::bigint & 4294967295)::oid HAVING count(*)=1`).Scan(&permitPID))
	var terminated bool
	require.NoError(t, control.QueryRow(ctx, `SELECT pg_terminate_backend($1)`, permitPID).Scan(&terminated))
	require.True(t, terminated)
	require.Error(t, permit.Release(ctx), "lost session cannot report a confirmed unlock")
	// A fresh session can acquire the barrier, but the independent User session
	// remains live. This deliberately does not claim replacement is authorized.
	conn, err := pgx.ConnectConfig(ctx, control.Config().ConnConfig.Copy())
	require.NoError(t, err)
	defer func() { _ = conn.Close(context.Background()) }()
	var locked bool
	require.NoError(t, conn.QueryRow(ctx, `SELECT pg_try_advisory_lock(6219270163691208962::bigint)`).Scan(&locked))
	require.True(t, locked)
	var alive bool
	require.NoError(t, user.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE pid=$1)`, userPID).Scan(&alive))
	require.True(t, alive, "control lock loss does not terminate User SQL")
	require.NoError(t, tx.Commit(ctx))
	require.NoError(t, conn.QueryRow(ctx, `SELECT pg_advisory_unlock(6219270163691208962::bigint)`).Scan(&locked))
	require.True(t, locked)
}

func TestCustodyCompetingLeaderRejected(t *testing.T) {
	ctx, control, _, binding, _ := custodyFixture(t)
	deadline, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	other, err := Start(deadline, control, binding)
	if other != nil {
		_ = other.Shutdown(context.Background())
	}
	require.Error(t, err)
}
