package sourcecustody

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

type creationTrace struct {
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

type creationRecordTrace struct{ entered chan struct{} }

func (t *creationRecordTrace) TraceQueryStart(ctx context.Context, _ *pgx.Conn, query pgx.TraceQueryStartData) context.Context {
	if strings.HasPrefix(query.SQL, "INSERT INTO public.user_custody_physical_source ") {
		close(t.entered)
		<-ctx.Done()
	}
	return ctx
}
func (*creationRecordTrace) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

func TestCreateClosedSourceCanceledAfterCreateDoesNotAdopt(t *testing.T) {
	ctx, control, admin, request := creationFixture(t)
	trace := &creationRecordTrace{entered: make(chan struct{})}
	config := control.Config().Copy()
	config.ConnConfig.Tracer = trace
	traced, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	t.Cleanup(traced.Close)
	call, cancel := context.WithCancel(ctx)
	defer cancel()
	result := make(chan error, 1)
	go func() { _, e := CreateClosedSource(call, traced, admin, request); result <- e }()
	select {
	case <-trace.entered:
	case err = <-result:
		t.Fatalf("creation returned before record write: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("creation did not reach record write")
	}
	cancel()
	select {
	case err = <-result:
		require.Error(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("cancelled creation did not finish")
	}
	var phase string
	var count int
	var allowed bool
	require.NoError(t, control.QueryRow(ctx, `SELECT phase FROM user_custody_creation WHERE singleton`).Scan(&phase))
	require.Equal(t, "CREATING", phase)
	require.NoError(t, control.QueryRow(ctx, `SELECT count(*) FROM user_custody_physical_source`).Scan(&count))
	require.Zero(t, count)
	require.NoError(t, admin.QueryRow(ctx, `SELECT datallowconn FROM pg_database WHERE datname=$1`, request.Database).Scan(&allowed))
	require.False(t, allowed, "an unrecorded source remains inaccessible")
	_, err = CreateClosedSource(ctx, control, admin, request)
	require.Error(t, err, "uncertain creation must not be adopted on retry")
}

func (t *creationTrace) TraceQueryStart(ctx context.Context, _ *pgx.Conn, query pgx.TraceQueryStartData) context.Context {
	if strings.HasPrefix(query.SQL, "CREATE DATABASE ") {
		t.once.Do(func() { close(t.entered) })
		select {
		case <-t.release:
		case <-ctx.Done():
		}
	}
	return ctx
}
func (*creationTrace) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

func TestCreateClosedSourceConflictLeavesIntentClosed(t *testing.T) {
	ctx, control, admin, request := creationFixture(t)
	trace := &creationTrace{entered: make(chan struct{}), release: make(chan struct{})}
	config := admin.Config().Copy()
	config.ConnConfig.Tracer = trace
	traced, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	t.Cleanup(traced.Close)
	result := make(chan error, 1)
	go func() { _, e := CreateClosedSource(ctx, control, traced, request); result <- e }()
	var release sync.Once
	stop := func() { release.Do(func() { close(trace.release) }) }
	defer stop()
	select {
	case <-trace.entered:
	case err = <-result:
		t.Fatalf("creation returned before source creation: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("creation did not reach source creation")
	}
	var phase string
	require.NoError(t, control.QueryRow(ctx, `SELECT phase FROM user_custody_creation WHERE singleton`).Scan(&phase))
	require.Equal(t, "CREATING", phase, "closed intent must commit before physical creation")
	competing := request
	competing.Database = "competing_source_test"
	_, err = CreateClosedSource(ctx, control, admin, competing)
	require.Error(t, err, "a second creator must not share the control incarnation")
	var competitorExists bool
	require.NoError(t, admin.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname=$1)`, competing.Database).Scan(&competitorExists))
	require.False(t, competitorExists)
	_, err = admin.Exec(ctx, `CREATE DATABASE voice_source_test WITH ALLOW_CONNECTIONS false`)
	require.NoError(t, err)
	stop()
	select {
	case err = <-result:
		require.Error(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("creation did not return after collision")
	}
	var count int
	require.NoError(t, control.QueryRow(ctx, `SELECT count(*) FROM user_custody_physical_source`).Scan(&count))
	require.Zero(t, count, "an unowned colliding source must never be adopted")
	require.NoError(t, control.QueryRow(ctx, `SELECT phase FROM user_custody_creation WHERE singleton`).Scan(&phase))
	require.Equal(t, "CREATING", phase)
	_, err = CreateClosedSource(ctx, control, admin, request)
	require.Error(t, err)
}

func creationFixture(t *testing.T) (context.Context, *pgxpool.Pool, *pgxpool.Pool, CreationRequest) {
	t.Helper()
	ctx := context.Background()
	control := custodyDatabase(t, ctx)
	source := custodyDatabase(t, ctx)
	config := source.Config().Copy()
	config.ConnConfig.Database = "postgres"
	admin, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	t.Cleanup(admin.Close)
	request := CreationRequest{SourceID: uuid.NewString(), Epoch: 1, Database: "voice_source_test"}
	require.NoError(t, control.QueryRow(ctx, `SELECT system_identifier::text FROM pg_control_system()`).Scan(&request.ControlSystemID))
	require.NoError(t, admin.QueryRow(ctx, `SELECT system_identifier::text FROM pg_control_system()`).Scan(&request.SourceSystemID))
	return ctx, control, admin, request
}

func TestCreateClosedSourceRetainsIndependentPhysicalIdentity(t *testing.T) {
	ctx, control, admin, request := creationFixture(t)
	created, err := CreateClosedSource(ctx, control, admin, request)
	require.NoError(t, err)
	require.Equal(t, request.SourceSystemID, created.SystemIdentifier)
	require.Equal(t, request.Database, created.Database)
	require.NotZero(t, created.DatabaseOID)
	var oid uint32
	var allowed bool
	require.NoError(t, admin.QueryRow(ctx, `SELECT oid,datallowconn FROM pg_database WHERE datname=$1`, request.Database).Scan(&oid, &allowed))
	require.Equal(t, created.DatabaseOID, oid)
	require.False(t, allowed)
	var publicConnect bool
	require.NoError(t, admin.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_database d CROSS JOIN LATERAL aclexplode(COALESCE(d.datacl,acldefault('d',d.datdba))) a WHERE d.oid=$1 AND a.grantee=0 AND a.privilege_type='CONNECT')`, oid).Scan(&publicConnect))
	require.False(t, publicConnect, "a future maintenance opening must not expose default PUBLIC CONNECT")
	var phase, sourceID, systemID, name string
	var epoch int64
	var recordedOID uint32
	require.NoError(t, control.QueryRow(ctx, `SELECT phase,source_id,epoch,source_system_id,database_name,database_oid FROM user_custody_creation WHERE singleton`).Scan(&phase, &sourceID, &epoch, &systemID, &name, &recordedOID))
	require.Equal(t, "CREATED_CLOSED", phase)
	require.Equal(t, request.SourceID, sourceID)
	require.Equal(t, request.Epoch, epoch)
	require.Equal(t, created.SystemIdentifier, systemID)
	require.Equal(t, created.Database, name)
	require.Equal(t, created.DatabaseOID, recordedOID)
	var pinned PhysicalSource
	require.NoError(t, control.QueryRow(ctx, `SELECT system_identifier,database_name,database_oid FROM user_custody_physical_source WHERE source_id=$1 AND epoch=$2`, request.SourceID, request.Epoch).Scan(&pinned.SystemIdentifier, &pinned.Database, &pinned.DatabaseOID))
	require.Equal(t, created, pinned)
	// No controller can start from this record: key lineage, state schema and
	// writer qualification have not been created by this closed-only step.
	_, err = Start(ctx, control, Binding{Epoch: request.Epoch, SourceID: request.SourceID, HistoryGeneration: uuid.New(), UserSourceGeneration: uuid.New(), KeysetDigest: [32]byte{1}})
	require.Error(t, err)
	config := admin.Config().Copy()
	config.ConnConfig.Database = request.Database
	probe, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	defer probe.Close()
	bounded, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	require.Error(t, probe.Ping(bounded))
	_, err = CreateClosedSource(ctx, control, admin, request)
	require.Error(t, err, "a retry must not adopt or replace an existing source")
}

func TestCreateClosedSourceRejectsExistingOrWrongCluster(t *testing.T) {
	ctx, control, admin, request := creationFixture(t)
	existing := request
	existing.Database = "postgres"
	_, err := CreateClosedSource(ctx, control, admin, existing)
	require.Error(t, err)
	wrong := request
	wrong.SourceSystemID = request.ControlSystemID
	_, err = CreateClosedSource(ctx, control, admin, wrong)
	require.Error(t, err)
	var exists bool
	require.NoError(t, control.QueryRow(ctx, `SELECT to_regclass('public.user_custody_creation') IS NOT NULL`).Scan(&exists))
	require.False(t, exists, "invalid requests must not create a control record")
	require.NoError(t, admin.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname=$1)`, request.Database).Scan(&exists))
	require.False(t, exists)
}
