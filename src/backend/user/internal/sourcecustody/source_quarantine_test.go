package sourcecustody

import (
	"context"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"voice/backend/pkg/integrationtest"
)

func TestQuarantineExistingSourceFencesAndPreservesData(t *testing.T) {
	ctx, control, admin, request := creationFixture(t)
	request.Database = "custody_test"
	physical := PhysicalSource{SystemIdentifier: request.SourceSystemID, Database: request.Database}
	require.NoError(t, admin.QueryRow(ctx, `SELECT oid FROM pg_database WHERE datname=$1`, request.Database).Scan(&physical.DatabaseOID))
	config := admin.Config().Copy()
	config.ConnConfig.Database = request.Database
	writer, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	defer writer.Close()
	_, err = writer.Exec(ctx, `CREATE TABLE existing_data(value text NOT NULL); INSERT INTO existing_data VALUES('preserved')`)
	require.NoError(t, err)
	require.NoError(t, QuarantineExistingSource(ctx, control, admin, request, physical))
	var phase string
	var allowed, stateExists bool
	var sessions int
	require.NoError(t, control.QueryRow(ctx, `SELECT phase,to_regclass('public.user_custody_state') IS NOT NULL FROM user_custody_creation WHERE singleton`).Scan(&phase, &stateExists))
	require.Equal(t, "QUARANTINED_EXISTING", phase)
	require.False(t, stateExists)
	require.NoError(t, admin.QueryRow(ctx, `SELECT datallowconn,(SELECT count(*) FROM pg_stat_activity WHERE datid=$1) FROM pg_database WHERE oid=$1`, physical.DatabaseOID).Scan(&allowed, &sessions))
	require.False(t, allowed)
	require.Zero(t, sessions)
	_, err = admin.Exec(ctx, `ALTER DATABASE custody_test WITH ALLOW_CONNECTIONS true`)
	require.NoError(t, err)
	probe, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	defer probe.Close()
	var value string
	require.NoError(t, probe.QueryRow(ctx, `SELECT value FROM existing_data`).Scan(&value))
	require.Equal(t, "preserved", value)
}

func TestQuarantineExistingSourceInterruptedIntentCanBeFenced(t *testing.T) {
	ctx, control, admin, request := creationFixture(t)
	request.Database = "custody_test"
	physical := PhysicalSource{SystemIdentifier: request.SourceSystemID, Database: request.Database}
	require.NoError(t, admin.QueryRow(ctx, `SELECT oid FROM pg_database WHERE datname=$1`, request.Database).Scan(&physical.DatabaseOID))
	trace := &fenceAlterTrace{entered: make(chan struct{}), release: make(chan struct{})}
	config := admin.Config().Copy()
	config.ConnConfig.Tracer = trace
	traced, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	defer traced.Close()
	call, cancel := context.WithCancel(ctx)
	defer cancel()
	result := make(chan error, 1)
	go func() { result <- QuarantineExistingSource(call, control, traced, request, physical) }()
	select {
	case <-trace.entered:
	case err = <-result:
		t.Fatalf("quarantine returned before fencing: %v", err)
	case <-time.After(10 * time.Second):
		t.Fatal("quarantine did not reach fencing")
	}
	var phase string
	require.NoError(t, control.QueryRow(ctx, `SELECT phase FROM user_custody_creation WHERE singleton`).Scan(&phase))
	require.Equal(t, "QUARANTINING", phase)
	contender, err := control.Begin(ctx)
	require.NoError(t, err)
	_, err = contender.Exec(ctx, `SET LOCAL lock_timeout='500ms'`)
	require.NoError(t, err)
	_, err = contender.Exec(ctx, `UPDATE user_custody_physical_source SET database_name=database_name || '_x' WHERE source_id=$1`, request.SourceID)
	var lockErr *pgconn.PgError
	require.ErrorAs(t, err, &lockErr)
	require.Equal(t, "55P03", lockErr.Code, "physical binding must stay locked while fencing")
	require.NoError(t, contender.Rollback(ctx))
	cancel()
	select {
	case err = <-result:
		require.Error(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("cancelled quarantine did not return")
	}
	require.NoError(t, RecoverInterruptedInitialization(ctx, control, admin, request, physical))
	var allowed bool
	require.NoError(t, admin.QueryRow(ctx, `SELECT datallowconn FROM pg_database WHERE oid=$1`, physical.DatabaseOID).Scan(&allowed))
	require.False(t, allowed)
	require.NoError(t, control.QueryRow(ctx, `SELECT phase FROM user_custody_creation WHERE singleton`).Scan(&phase))
	require.Equal(t, "RECOVERY_REQUIRED", phase)
}

func TestQuarantineExistingSourceRejectsWrongPhysicalIdentity(t *testing.T) {
	ctx, control, admin, request := creationFixture(t)
	request.Database = "custody_test"
	physical := PhysicalSource{SystemIdentifier: request.SourceSystemID, Database: request.Database}
	require.NoError(t, admin.QueryRow(ctx, `SELECT oid FROM pg_database WHERE datname=$1`, request.Database).Scan(&physical.DatabaseOID))
	physical.DatabaseOID++
	require.Error(t, QuarantineExistingSource(ctx, control, admin, request, physical))
	var allowed, stateExists bool
	require.NoError(t, admin.QueryRow(ctx, `SELECT datallowconn FROM pg_database WHERE datname=$1`, request.Database).Scan(&allowed))
	require.True(t, allowed)
	require.NoError(t, control.QueryRow(ctx, `SELECT to_regclass('public.user_custody_creation') IS NOT NULL`).Scan(&stateExists))
	require.False(t, stateExists)
}

func TestQuarantineExistingSourceRejectsPreparedTransaction(t *testing.T) {
	ctx := context.Background()
	control := custodyDatabase(t, ctx)
	integrationtest.ConfigureDockerTesting()
	pg, err := postgres.Run(ctx, integrationtest.PostgresImage, postgres.BasicWaitStrategies(), postgres.WithDatabase("custody_test"), postgres.WithUsername("custody_test"), postgres.WithPassword("fixture_only"), testcontainers.WithConfigModifier(func(cfg *container.Config) {
		cfg.Cmd = []string{"postgres", "-c", "max_prepared_transactions=10"}
	}))
	if pg != nil {
		t.Cleanup(func() {
			cleanup, stop := context.WithTimeout(context.Background(), 30*time.Second)
			defer stop()
			require.NoError(t, pg.Terminate(cleanup))
		})
	}
	require.NoError(t, err)
	dsn, err := pg.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	config, err := pgxpool.ParseConfig(dsn)
	require.NoError(t, err)
	writer, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	defer writer.Close()
	adminConfig := config.Copy()
	adminConfig.ConnConfig.Database = "postgres"
	admin, err := pgxpool.NewWithConfig(ctx, adminConfig)
	require.NoError(t, err)
	defer admin.Close()
	request := CreationRequest{SourceID: uuid.NewString(), Epoch: 1, Database: "custody_test"}
	require.NoError(t, control.QueryRow(ctx, `SELECT system_identifier::text FROM pg_control_system()`).Scan(&request.ControlSystemID))
	require.NoError(t, admin.QueryRow(ctx, `SELECT system_identifier::text FROM pg_control_system()`).Scan(&request.SourceSystemID))
	physical := PhysicalSource{SystemIdentifier: request.SourceSystemID, Database: request.Database}
	require.NoError(t, admin.QueryRow(ctx, `SELECT oid FROM pg_database WHERE datname=$1`, request.Database).Scan(&physical.DatabaseOID))
	_, err = writer.Exec(ctx, `CREATE TABLE prepared_existing_test(value integer)`)
	require.NoError(t, err)
	_, err = writer.Exec(ctx, `BEGIN; INSERT INTO prepared_existing_test VALUES(1); PREPARE TRANSACTION 'quarantine_fixture'`)
	require.NoError(t, err)
	require.Error(t, QuarantineExistingSource(ctx, control, admin, request, physical))
	var phase string
	var allowed bool
	require.NoError(t, control.QueryRow(ctx, `SELECT phase FROM user_custody_creation WHERE singleton`).Scan(&phase))
	require.Equal(t, "QUARANTINING", phase)
	require.NoError(t, admin.QueryRow(ctx, `SELECT datallowconn FROM pg_database WHERE oid=$1`, physical.DatabaseOID).Scan(&allowed))
	require.False(t, allowed)
}
