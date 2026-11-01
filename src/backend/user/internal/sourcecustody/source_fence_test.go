package sourcecustody

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"voice/backend/pkg/integrationtest"
)

type fenceAlterTrace struct {
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (t *fenceAlterTrace) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	if strings.HasPrefix(data.SQL, "ALTER DATABASE ") {
		t.once.Do(func() { close(t.entered) })
		select {
		case <-t.release:
		case <-ctx.Done():
		}
	}
	return ctx
}
func (*fenceAlterTrace) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

func TestFenceSourceLocksPhysicalRecordThroughFence(t *testing.T) {
	ctx, control, user, binding, controller := custodyFixture(t)
	trace := &fenceAlterTrace{entered: make(chan struct{}), release: make(chan struct{})}
	config := user.Config().Copy()
	config.ConnConfig.Database = "postgres"
	config.ConnConfig.Tracer = trace
	admin, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	t.Cleanup(admin.Close)
	source := PhysicalSource{Database: "custody_test"}
	require.NoError(t, admin.QueryRow(ctx, `SELECT system_identifier::text FROM pg_control_system()`).Scan(&source.SystemIdentifier))
	require.NoError(t, admin.QueryRow(ctx, `SELECT oid FROM pg_database WHERE datname=$1`, source.Database).Scan(&source.DatabaseOID))
	installPhysicalRecord(t, control, binding, source)
	results := make(chan error, 1)
	go func() { results <- controller.FenceSource(ctx, admin, source) }()
	var release sync.Once
	stop := func() { release.Do(func() { close(trace.release) }) }
	defer stop()
	select {
	case <-trace.entered:
	case err = <-results:
		t.Fatalf("fence returned before physical transition: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("fence did not reach physical transition")
	}
	blocked, cancel := context.WithTimeout(ctx, 250*time.Millisecond)
	_, err = control.Exec(blocked, `UPDATE user_custody_physical_source SET database_oid=$3 WHERE source_id=$1 AND epoch=$2`, binding.SourceID, binding.Epoch, source.DatabaseOID+1)
	cancel()
	require.ErrorIs(t, err, context.DeadlineExceeded, "physical binding must stay locked throughout the source transition")
	stop()
	select {
	case err = <-results:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("fence did not finish after source transition was released")
	}
}

func TestFenceSourceRejectsMissingPhysicalRecord(t *testing.T) {
	ctx, control, user, _, controller := custodyFixture(t)
	config := user.Config().Copy()
	config.ConnConfig.Database = "postgres"
	admin, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	t.Cleanup(admin.Close)
	source := PhysicalSource{Database: "custody_test"}
	require.NoError(t, admin.QueryRow(ctx, `SELECT system_identifier::text FROM pg_control_system()`).Scan(&source.SystemIdentifier))
	require.NoError(t, admin.QueryRow(ctx, `SELECT oid FROM pg_database WHERE datname=$1`, source.Database).Scan(&source.DatabaseOID))
	require.Error(t, controller.FenceSource(ctx, admin, source))
	var state string
	var allowed bool
	require.NoError(t, control.QueryRow(ctx, `SELECT state FROM user_custody_state WHERE singleton`).Scan(&state))
	require.Equal(t, "CLOSED", state)
	require.NoError(t, admin.QueryRow(ctx, `SELECT datallowconn FROM pg_database WHERE oid=$1`, source.DatabaseOID).Scan(&allowed))
	require.True(t, allowed)
	require.NoError(t, user.Ping(ctx))
}

func installPhysicalRecord(t *testing.T, control *pgxpool.Pool, binding Binding, source PhysicalSource) {
	t.Helper()
	ctx := context.Background()
	_, err := control.Exec(ctx, `CREATE TABLE user_custody_physical_source(source_id text NOT NULL, epoch bigint NOT NULL, system_identifier text NOT NULL, database_name text NOT NULL, database_oid oid NOT NULL, PRIMARY KEY(source_id,epoch))`)
	require.NoError(t, err)
	_, err = control.Exec(ctx, `INSERT INTO user_custody_physical_source VALUES($1,$2,$3,$4,$5)`, binding.SourceID, binding.Epoch, source.SystemIdentifier, source.Database, source.DatabaseOID)
	require.NoError(t, err)
}

func TestFenceSourceRejectsCallerTargetDifferentFromControlRecord(t *testing.T) {
	ctx, control, user, binding, controller := custodyFixture(t)
	config := user.Config().Copy()
	config.ConnConfig.Database = "postgres"
	admin, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	t.Cleanup(admin.Close)
	source := PhysicalSource{Database: "custody_test"}
	require.NoError(t, admin.QueryRow(ctx, `SELECT system_identifier::text FROM pg_control_system()`).Scan(&source.SystemIdentifier))
	require.NoError(t, admin.QueryRow(ctx, `SELECT oid FROM pg_database WHERE datname=$1`, source.Database).Scan(&source.DatabaseOID))
	wrong := source
	wrong.DatabaseOID++
	installPhysicalRecord(t, control, binding, wrong)
	require.Error(t, controller.FenceSource(ctx, admin, source), "caller must not redirect a grant to another physical source")
	var allowed bool
	var state string
	require.NoError(t, admin.QueryRow(ctx, `SELECT datallowconn FROM pg_database WHERE oid=$1`, source.DatabaseOID).Scan(&allowed))
	require.True(t, allowed)
	require.NoError(t, control.QueryRow(ctx, `SELECT state FROM user_custody_state WHERE singleton`).Scan(&state))
	require.Equal(t, "CLOSED", state)
	require.NoError(t, user.Ping(ctx))
}

func TestFenceSourceTerminatesWriterAndPreventsReconnect(t *testing.T) {
	ctx, control, user, binding, controller := custodyFixture(t)
	config := user.Config().Copy()
	config.ConnConfig.Database = "postgres"
	admin, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	t.Cleanup(admin.Close)
	var source PhysicalSource
	source.Database = "custody_test"
	require.NoError(t, admin.QueryRow(ctx, `SELECT system_identifier::text FROM pg_control_system()`).Scan(&source.SystemIdentifier))
	require.NoError(t, admin.QueryRow(ctx, `SELECT oid FROM pg_database WHERE datname=$1`, source.Database).Scan(&source.DatabaseOID))
	installPhysicalRecord(t, control, binding, source)
	_, err = user.Exec(ctx, `CREATE TABLE fence_test(value integer)`)
	require.NoError(t, err)
	writer, err := user.Begin(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = writer.Rollback(context.Background()) })
	_, err = writer.Exec(ctx, `INSERT INTO fence_test VALUES(1)`)
	require.NoError(t, err)
	bounded, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	require.NoError(t, controller.FenceSource(bounded, admin, source))
	require.Error(t, writer.Commit(ctx), "the fenced transaction must not commit")
	var accepts bool
	var sessions int
	require.NoError(t, admin.QueryRow(ctx, `SELECT datallowconn FROM pg_database WHERE oid=$1`, source.DatabaseOID).Scan(&accepts))
	require.False(t, accepts)
	require.NoError(t, admin.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE datid=$1`, source.DatabaseOID).Scan(&sessions))
	require.Zero(t, sessions)
	// pgxpool connects lazily; a real connection attempt must fail.
	probe, err := pgxpool.NewWithConfig(ctx, user.Config().Copy())
	require.NoError(t, err)
	defer probe.Close()
	require.Error(t, probe.Ping(bounded))
	// Reopen only the owned disposable fixture to prove the uncommitted write
	// was rolled back when its PostgreSQL backend was terminated.
	_, err = admin.Exec(ctx, `ALTER DATABASE custody_test WITH ALLOW_CONNECTIONS true`)
	require.NoError(t, err)
	verify, err := pgxpool.NewWithConfig(ctx, user.Config().Copy())
	require.NoError(t, err)
	defer verify.Close()
	var rows int
	require.NoError(t, verify.QueryRow(ctx, `SELECT count(*) FROM fence_test`).Scan(&rows))
	require.Zero(t, rows)
}

func TestFenceSourceRefusesPreparedTransaction(t *testing.T) {
	ctx, control, _, binding, controller := custodyFixture(t)
	integrationtest.ConfigureDockerTesting()
	pg, err := postgres.Run(ctx, integrationtest.PostgresImage, postgres.BasicWaitStrategies(), postgres.WithDatabase("custody_prepared"), postgres.WithUsername("custody_test"), postgres.WithPassword("fixture_only"), testcontainers.WithConfigModifier(func(cfg *container.Config) {
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
	config, err := pgxpool.ParseConfig(dsn)
	require.NoError(t, err)
	user, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	t.Cleanup(user.Close)
	adminConfig := config.Copy()
	adminConfig.ConnConfig.Database = "postgres"
	admin, err := pgxpool.NewWithConfig(ctx, adminConfig)
	require.NoError(t, err)
	t.Cleanup(admin.Close)
	source := PhysicalSource{Database: "custody_prepared"}
	require.NoError(t, admin.QueryRow(ctx, `SELECT system_identifier::text FROM pg_control_system()`).Scan(&source.SystemIdentifier))
	require.NoError(t, admin.QueryRow(ctx, `SELECT oid FROM pg_database WHERE datname=$1`, source.Database).Scan(&source.DatabaseOID))
	installPhysicalRecord(t, control, binding, source)
	_, err = user.Exec(ctx, `CREATE TABLE prepared_fence_test(value integer)`)
	require.NoError(t, err)
	_, err = user.Exec(ctx, `BEGIN; INSERT INTO prepared_fence_test VALUES(1); PREPARE TRANSACTION 'custody_fixture'`)
	require.NoError(t, err)
	bounded, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	require.Error(t, controller.FenceSource(bounded, admin, source), "prepared transactions must prevent a successful fence")
	var allowed bool
	require.NoError(t, admin.QueryRow(ctx, `SELECT datallowconn FROM pg_database WHERE oid=$1`, source.DatabaseOID).Scan(&allowed))
	require.False(t, allowed, "failed qualification must leave source closed")
	var state string
	require.NoError(t, control.QueryRow(ctx, `SELECT state FROM user_custody_state WHERE singleton`).Scan(&state))
	require.Equal(t, "CLOSED", state)
	var prepared int
	require.NoError(t, admin.QueryRow(ctx, `SELECT count(*) FROM pg_prepared_xacts WHERE database=$1`, source.Database).Scan(&prepared))
	require.Equal(t, 1, prepared)
}

func TestFenceSourceRejectsWrongIdentityAfterClosingAdmission(t *testing.T) {
	ctx, control, user, _, controller := custodyFixture(t)
	config := user.Config().Copy()
	config.ConnConfig.Database = "postgres"
	admin, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	t.Cleanup(admin.Close)
	var source PhysicalSource
	source.Database = "custody_test"
	require.NoError(t, admin.QueryRow(ctx, `SELECT system_identifier::text FROM pg_control_system()`).Scan(&source.SystemIdentifier))
	require.NoError(t, admin.QueryRow(ctx, `SELECT oid FROM pg_database WHERE datname=$1`, source.Database).Scan(&source.DatabaseOID))
	for _, wrong := range []PhysicalSource{
		{SystemIdentifier: source.SystemIdentifier, Database: source.Database, DatabaseOID: source.DatabaseOID + 1},
		{SystemIdentifier: "0", Database: source.Database, DatabaseOID: source.DatabaseOID},
		{SystemIdentifier: source.SystemIdentifier, Database: "postgres; DROP DATABASE custody_test", DatabaseOID: source.DatabaseOID},
	} {
		require.Error(t, controller.FenceSource(ctx, admin, wrong))
	}
	var allowed bool
	var state string
	require.NoError(t, admin.QueryRow(ctx, `SELECT datallowconn FROM pg_database WHERE oid=$1`, source.DatabaseOID).Scan(&allowed))
	require.True(t, allowed)
	require.NoError(t, control.QueryRow(ctx, `SELECT state FROM user_custody_state WHERE singleton`).Scan(&state))
	require.Equal(t, "CLOSED", state, "uncertain source identity must close admission")
	require.NoError(t, user.Ping(ctx))
}

func TestFenceSourceRejectsControlCluster(t *testing.T) {
	ctx, control, _, _, controller := custodyFixture(t)
	config := control.Config().Copy()
	config.ConnConfig.Database = "postgres"
	admin, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	t.Cleanup(admin.Close)
	source := PhysicalSource{Database: "custody_test"}
	require.NoError(t, admin.QueryRow(ctx, `SELECT system_identifier::text FROM pg_control_system()`).Scan(&source.SystemIdentifier))
	require.NoError(t, admin.QueryRow(ctx, `SELECT oid FROM pg_database WHERE datname=$1`, source.Database).Scan(&source.DatabaseOID))
	require.Error(t, controller.FenceSource(ctx, admin, source))
	var allowed bool
	require.NoError(t, admin.QueryRow(ctx, `SELECT datallowconn FROM pg_database WHERE oid=$1`, source.DatabaseOID).Scan(&allowed))
	require.True(t, allowed)
	var state string
	require.NoError(t, control.QueryRow(ctx, `SELECT state FROM user_custody_state WHERE singleton`).Scan(&state))
	require.Equal(t, "CLOSED", state)
	require.NoError(t, control.Ping(ctx))
}
