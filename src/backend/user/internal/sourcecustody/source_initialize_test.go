package sourcecustody

import (
	"bytes"
	"context"
	"crypto/sha256"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"voice/backend/user/internal/identityassociation"
)

type migrationRecordTrace struct{ entered chan struct{} }

type heldMigrationRecordTrace struct {
	entered chan struct{}
	release chan struct{}
}

type heldPublicationTrace struct {
	pid     chan uint32
	release chan struct{}
}

func (t *heldPublicationTrace) TraceQueryStart(ctx context.Context, conn *pgx.Conn, query pgx.TraceQueryStartData) context.Context {
	if strings.HasPrefix(query.SQL, "CREATE TABLE public.user_custody_state") {
		t.pid <- conn.PgConn().PID()
		select {
		case <-t.release:
		case <-ctx.Done():
		}
	}
	return ctx
}

func (*heldPublicationTrace) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

func (t *heldMigrationRecordTrace) TraceQueryStart(ctx context.Context, _ *pgx.Conn, query pgx.TraceQueryStartData) context.Context {
	if strings.HasPrefix(query.SQL, "UPDATE user_custody_creation SET phase='MIGRATED'") {
		close(t.entered)
		select {
		case <-t.release:
		case <-ctx.Done():
		}
	}
	return ctx
}

func (*heldMigrationRecordTrace) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

func (t *migrationRecordTrace) TraceQueryStart(ctx context.Context, _ *pgx.Conn, query pgx.TraceQueryStartData) context.Context {
	if strings.HasPrefix(query.SQL, "UPDATE user_custody_creation SET phase='MIGRATED'") {
		close(t.entered)
		<-ctx.Done()
	}
	return ctx
}

func (*migrationRecordTrace) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

func migrationFixture(t *testing.T) (string, *identityassociation.Keyset) {
	t.Helper()
	dir, err := filepath.Abs("../../../migrations/user_db")
	require.NoError(t, err)
	key := bytes.Repeat([]byte{0x71}, 32)
	keys, err := identityassociation.NewKeyset(identityassociation.KeysetSnapshot{Revision: 1, ActiveVersion: 1, Versions: map[uint32][32]byte{1: sha256.Sum256(key)}}, map[uint32][]byte{1: key})
	require.NoError(t, err)
	return dir, keys
}

func TestInitializeClosedSourceMigratesAndPinsFreshLineage(t *testing.T) {
	ctx, control, admin, request := creationFixture(t)
	physical, err := CreateClosedSource(ctx, control, admin, request)
	require.NoError(t, err)
	dir, keys := migrationFixture(t)
	require.NoError(t, InitializeClosedSource(ctx, control, admin, request, physical, dir, keys))
	var phase, state string
	var history, source string
	var digest []byte
	require.NoError(t, control.QueryRow(ctx, `SELECT phase FROM user_custody_creation WHERE singleton`).Scan(&phase))
	require.Equal(t, "INITIALIZED_CLOSED", phase)
	require.NoError(t, control.QueryRow(ctx, `SELECT state,history_generation::text,user_source_generation::text,keyset_digest FROM user_custody_state WHERE singleton`).Scan(&state, &history, &source, &digest))
	require.Equal(t, "CLOSED", state)
	require.NotEmpty(t, history)
	require.NotEmpty(t, source)
	require.NotEqual(t, history, source)
	wantDigest, err := keys.CustodyDigest()
	require.NoError(t, err)
	require.Equal(t, wantDigest[:], digest)
	var allowed bool
	require.NoError(t, admin.QueryRow(ctx, `SELECT datallowconn FROM pg_database WHERE oid=$1`, physical.DatabaseOID).Scan(&allowed))
	require.False(t, allowed)
	_, err = admin.Exec(ctx, `ALTER DATABASE voice_source_test WITH ALLOW_CONNECTIONS true`)
	require.NoError(t, err)
	config := admin.Config().Copy()
	config.ConnConfig.Database = request.Database
	probe, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	defer probe.Close()
	var version int
	var dirty, guards bool
	var dbHistory, dbSource string
	require.NoError(t, probe.QueryRow(ctx, `SELECT version,dirty FROM schema_migrations`).Scan(&version, &dirty))
	require.Equal(t, 22, version)
	require.False(t, dirty)
	require.NoError(t, probe.QueryRow(ctx, `SELECT history_generation::text,user_source_generation::text,guards_active FROM identity_meta WHERE singleton`).Scan(&dbHistory, &dbSource, &guards))
	require.Equal(t, history, dbHistory)
	require.Equal(t, source, dbSource)
	require.False(t, guards, "initialization cannot activate unqualified writers")
}

func TestInitializeClosedSourceRejectsChangedMigrationsBeforeOpening(t *testing.T) {
	ctx, control, admin, request := creationFixture(t)
	physical, err := CreateClosedSource(ctx, control, admin, request)
	require.NoError(t, err)
	dir, keys := migrationFixture(t)
	files, err := filepath.Glob(filepath.Join(dir, "*.up.sql"))
	require.NoError(t, err)
	target := t.TempDir()
	for _, file := range files {
		data, readErr := os.ReadFile(file)
		require.NoError(t, readErr)
		if filepath.Base(file) == "000001_init.up.sql" {
			data = append(data, []byte("\n-- changed\n")...)
		}
		require.NoError(t, os.WriteFile(filepath.Join(target, filepath.Base(file)), data, 0600))
	}
	require.Error(t, InitializeClosedSource(context.Background(), control, admin, request, physical, target, keys))
	var phase string
	var allowed bool
	require.NoError(t, control.QueryRow(ctx, `SELECT phase FROM user_custody_creation WHERE singleton`).Scan(&phase))
	require.Equal(t, "CREATED_CLOSED", phase)
	require.NoError(t, admin.QueryRow(ctx, `SELECT datallowconn FROM pg_database WHERE oid=$1`, physical.DatabaseOID).Scan(&allowed))
	require.False(t, allowed)
}

func TestInitializeClosedSourceRejectsUntrackedWriter(t *testing.T) {
	ctx, control, admin, request := creationFixture(t)
	physical, err := CreateClosedSource(ctx, control, admin, request)
	require.NoError(t, err)
	_, err = admin.Exec(ctx, `CREATE ROLE custody_untracked_writer LOGIN`)
	require.NoError(t, err)
	_, err = admin.Exec(ctx, `GRANT CONNECT ON DATABASE voice_source_test TO custody_untracked_writer`)
	require.NoError(t, err)
	dir, keys := migrationFixture(t)
	require.Error(t, InitializeClosedSource(ctx, control, admin, request, physical, dir, keys))
	var phase string
	var allowed bool
	require.NoError(t, control.QueryRow(ctx, `SELECT phase FROM user_custody_creation WHERE singleton`).Scan(&phase))
	require.Equal(t, "CREATED_CLOSED", phase)
	require.NoError(t, admin.QueryRow(ctx, `SELECT datallowconn FROM pg_database WHERE oid=$1`, physical.DatabaseOID).Scan(&allowed))
	require.False(t, allowed)
}

func TestCreatedSourceDoesNotInheritTemplate1Objects(t *testing.T) {
	ctx, control, admin, request := creationFixture(t)
	config := admin.Config().Copy()
	config.ConnConfig.Database = "template1"
	template, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	_, err = template.Exec(ctx, `CREATE TABLE public.untrusted_template_marker(id integer)`)
	require.NoError(t, err)
	template.Close()
	physical, err := CreateClosedSource(ctx, control, admin, request)
	require.NoError(t, err)
	_, err = admin.Exec(ctx, `ALTER DATABASE voice_source_test WITH ALLOW_CONNECTIONS true`)
	require.NoError(t, err)
	config.ConnConfig.Database = request.Database
	probe, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	defer probe.Close()
	var marker bool
	require.NoError(t, probe.QueryRow(ctx, `SELECT to_regclass('public.untrusted_template_marker') IS NOT NULL`).Scan(&marker))
	require.False(t, marker, "a fresh source must not inherit objects from template1")
	require.NotZero(t, physical.DatabaseOID)
}

func TestInitializeCancellationClosesOpenedSource(t *testing.T) {
	ctx, control, admin, request := creationFixture(t)
	physical, err := CreateClosedSource(ctx, control, admin, request)
	require.NoError(t, err)
	dir, keys := migrationFixture(t)
	trace := &migrationRecordTrace{entered: make(chan struct{})}
	config := control.Config().Copy()
	config.ConnConfig.Tracer = trace
	traced, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	defer traced.Close()
	call, cancel := context.WithCancel(ctx)
	defer cancel()
	result := make(chan error, 1)
	go func() { result <- InitializeClosedSource(call, traced, admin, request, physical, dir, keys) }()
	select {
	case <-trace.entered:
	case err = <-result:
		t.Fatalf("initialization returned before recording lineage: %v", err)
	case <-time.After(30 * time.Second):
		t.Fatal("initialization did not reach lineage record")
	}
	cancel()
	select {
	case err = <-result:
		require.Error(t, err)
	case <-time.After(15 * time.Second):
		t.Fatal("cancelled initialization did not finish")
	}
	var phase string
	var allowed bool
	require.NoError(t, control.QueryRow(ctx, `SELECT phase FROM user_custody_creation WHERE singleton`).Scan(&phase))
	require.Equal(t, "MIGRATING", phase)
	require.NoError(t, admin.QueryRow(ctx, `SELECT datallowconn FROM pg_database WHERE oid=$1`, physical.DatabaseOID).Scan(&allowed))
	require.False(t, allowed)
	require.Error(t, InitializeClosedSource(ctx, control, admin, request, physical, dir, keys), "interrupted lineage cannot be adopted by retry")
}

func TestInitializeLocksPhysicalBindingUntilClosed(t *testing.T) {
	ctx, control, admin, request := creationFixture(t)
	physical, err := CreateClosedSource(ctx, control, admin, request)
	require.NoError(t, err)
	dir, keys := migrationFixture(t)
	trace := &heldMigrationRecordTrace{entered: make(chan struct{}), release: make(chan struct{})}
	config := control.Config().Copy()
	config.ConnConfig.Tracer = trace
	traced, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	defer traced.Close()
	call, cancel := context.WithCancel(ctx)
	defer cancel()
	result := make(chan error, 1)
	go func() { result <- InitializeClosedSource(call, traced, admin, request, physical, dir, keys) }()
	defer close(trace.release)
	select {
	case <-trace.entered:
	case err = <-result:
		t.Fatalf("initialization returned before recording lineage: %v", err)
	case <-time.After(30 * time.Second):
		t.Fatal("initialization did not reach lineage record")
	}
	contender, err := control.Begin(ctx)
	require.NoError(t, err)
	_, err = contender.Exec(ctx, `SET LOCAL lock_timeout='500ms'`)
	require.NoError(t, err)
	_, err = contender.Exec(ctx, `UPDATE user_custody_physical_source SET database_name=database_name || '_x' WHERE source_id=$1`, request.SourceID)
	var lockErr *pgconn.PgError
	require.ErrorAs(t, err, &lockErr)
	require.Equal(t, "55P03", lockErr.Code, "the physical binding must remain locked until the source is closed")
	require.NoError(t, contender.Rollback(ctx))
	cancel()
	select {
	case err = <-result:
		require.Error(t, err)
	case <-time.After(15 * time.Second):
		t.Fatal("cancelled initialization did not finish")
	}
}

func TestInitializeLostBindingSessionCannotPublish(t *testing.T) {
	ctx, control, admin, request := creationFixture(t)
	physical, err := CreateClosedSource(ctx, control, admin, request)
	require.NoError(t, err)
	dir, keys := migrationFixture(t)
	trace := &heldPublicationTrace{pid: make(chan uint32, 1), release: make(chan struct{})}
	config := control.Config().Copy()
	config.ConnConfig.Tracer = trace
	traced, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	defer traced.Close()
	call, cancel := context.WithCancel(ctx)
	defer cancel()
	result := make(chan error, 1)
	go func() { result <- InitializeClosedSource(call, traced, admin, request, physical, dir, keys) }()
	var pid uint32
	select {
	case pid = <-trace.pid:
	case err = <-result:
		t.Fatalf("initialization returned before publishing: %v", err)
	case <-time.After(30 * time.Second):
		t.Fatal("initialization did not reach publication")
	}
	var terminated bool
	require.NoError(t, control.QueryRow(ctx, `SELECT pg_terminate_backend($1)`, pid).Scan(&terminated))
	require.True(t, terminated)
	close(trace.release)
	select {
	case err = <-result:
		require.Error(t, err)
	case <-time.After(15 * time.Second):
		t.Fatal("initialization did not finish after binding session loss")
	}
	var present, allowed bool
	require.NoError(t, control.QueryRow(ctx, `SELECT to_regclass('public.user_custody_state') IS NOT NULL`).Scan(&present))
	require.False(t, present)
	require.NoError(t, admin.QueryRow(ctx, `SELECT datallowconn FROM pg_database WHERE oid=$1`, physical.DatabaseOID).Scan(&allowed))
	require.False(t, allowed)
}
