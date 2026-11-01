package sourcecustody

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"voice/backend/pkg/integrationtest"
)

type crashInitializationTrace struct{}

func (*crashInitializationTrace) TraceQueryStart(ctx context.Context, _ *pgx.Conn, query pgx.TraceQueryStartData) context.Context {
	if strings.HasPrefix(query.SQL, "UPDATE user_custody_creation SET phase='MIGRATED'") {
		fmt.Println("INITIALIZATION_OPEN")
		<-ctx.Done()
	}
	return ctx
}

func (*crashInitializationTrace) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

func TestInterruptedInitializationCrashChild(t *testing.T) {
	if os.Getenv("VOICE_CUSTODY_CRASH_CHILD") != "1" {
		return
	}
	ctx := context.Background()
	var request CreationRequest
	var physical PhysicalSource
	require.NoError(t, json.Unmarshal([]byte(os.Getenv("VOICE_CUSTODY_CRASH_REQUEST")), &request))
	require.NoError(t, json.Unmarshal([]byte(os.Getenv("VOICE_CUSTODY_CRASH_PHYSICAL")), &physical))
	config, err := pgxpool.ParseConfig(os.Getenv("VOICE_CUSTODY_CRASH_CONTROL"))
	require.NoError(t, err)
	config.ConnConfig.Tracer = &crashInitializationTrace{}
	control, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	defer control.Close()
	admin, err := pgxpool.New(ctx, os.Getenv("VOICE_CUSTODY_CRASH_ADMIN"))
	require.NoError(t, err)
	defer admin.Close()
	dir, keys := migrationFixture(t)
	_ = InitializeClosedSource(ctx, control, admin, request, physical, dir, keys)
	t.Fatal("child initialization unexpectedly finished")
}

func TestRecoverInterruptedInitializationAfterProcessCrash(t *testing.T) {
	ctx, control, admin, request := creationFixture(t)
	physical, err := CreateClosedSource(ctx, control, admin, request)
	require.NoError(t, err)
	binary, err := os.Executable()
	require.NoError(t, err)
	requestJSON, err := json.Marshal(request)
	require.NoError(t, err)
	physicalJSON, err := json.Marshal(physical)
	require.NoError(t, err)
	child := exec.Command(binary, "-test.run=^TestInterruptedInitializationCrashChild$")
	child.Env = append(os.Environ(), "VOICE_CUSTODY_CRASH_CHILD=1", "VOICE_CUSTODY_CRASH_CONTROL="+control.Config().ConnConfig.ConnString(), "VOICE_CUSTODY_CRASH_ADMIN="+admin.Config().ConnConfig.ConnString(), "VOICE_CUSTODY_CRASH_REQUEST="+string(requestJSON), "VOICE_CUSTODY_CRASH_PHYSICAL="+string(physicalJSON))
	stdout, err := child.StdoutPipe()
	require.NoError(t, err)
	require.NoError(t, child.Start())
	defer func() {
		if child.ProcessState == nil {
			_ = child.Process.Kill()
			_ = child.Wait()
		}
	}()
	ready := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		if scanner.Scan() {
			ready <- scanner.Text()
		} else {
			ready <- ""
		}
	}()
	select {
	case line := <-ready:
		require.Equal(t, "INITIALIZATION_OPEN", line, "child exited before reaching open initialization")
	case <-time.After(30 * time.Second):
		t.Fatal("child did not reach open initialization")
	}
	require.NoError(t, child.Process.Kill())
	require.Error(t, child.Wait())
	var phase string
	var allowed bool
	require.NoError(t, control.QueryRow(ctx, `SELECT phase FROM user_custody_creation WHERE singleton`).Scan(&phase))
	require.Equal(t, "MIGRATING", phase)
	require.NoError(t, admin.QueryRow(ctx, `SELECT datallowconn FROM pg_database WHERE oid=$1`, physical.DatabaseOID).Scan(&allowed))
	require.True(t, allowed, "an abrupt process loss bypasses graceful cleanup")
	require.Eventually(t, func() bool {
		var released bool
		return control.QueryRow(ctx, `SELECT NOT EXISTS(SELECT 1 FROM pg_locks WHERE locktype='advisory' AND granted)`).Scan(&released) == nil && released
	}, 5*time.Second, 25*time.Millisecond)
	require.NoError(t, RecoverInterruptedInitialization(ctx, control, admin, request, physical))
	require.NoError(t, admin.QueryRow(ctx, `SELECT datallowconn FROM pg_database WHERE oid=$1`, physical.DatabaseOID).Scan(&allowed))
	require.False(t, allowed)
	require.NoError(t, control.QueryRow(ctx, `SELECT phase FROM user_custody_creation WHERE singleton`).Scan(&phase))
	require.Equal(t, "RECOVERY_REQUIRED", phase)
}

func TestRecoverInterruptedInitializationFencesSessions(t *testing.T) {
	ctx, control, admin, request := creationFixture(t)
	physical, err := CreateClosedSource(ctx, control, admin, request)
	require.NoError(t, err)
	_, err = control.Exec(ctx, `UPDATE user_custody_creation SET phase='MIGRATING' WHERE singleton`)
	require.NoError(t, err)
	_, err = admin.Exec(ctx, `ALTER DATABASE voice_source_test WITH ALLOW_CONNECTIONS true`)
	require.NoError(t, err)
	config := admin.Config().Copy()
	config.ConnConfig.Database = request.Database
	writer, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	defer writer.Close()
	require.NoError(t, writer.Ping(ctx))
	require.NoError(t, RecoverInterruptedInitialization(ctx, control, admin, request, physical))
	var phase string
	var allowed, present bool
	var sessions int
	require.NoError(t, control.QueryRow(ctx, `SELECT phase FROM user_custody_creation WHERE singleton`).Scan(&phase))
	require.Equal(t, "RECOVERY_REQUIRED", phase)
	require.NoError(t, control.QueryRow(ctx, `SELECT to_regclass('public.user_custody_state') IS NOT NULL`).Scan(&present))
	require.False(t, present)
	require.NoError(t, admin.QueryRow(ctx, `SELECT datallowconn FROM pg_database WHERE oid=$1`, physical.DatabaseOID).Scan(&allowed))
	require.False(t, allowed)
	require.NoError(t, admin.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE datid=$1`, physical.DatabaseOID).Scan(&sessions))
	require.Zero(t, sessions)
	require.Error(t, RecoverInterruptedInitialization(ctx, control, admin, request, physical))
}

func TestRecoverInterruptedInitializationRejectsPreparedTransactions(t *testing.T) {
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
	config.ConnConfig.Database = "postgres"
	admin, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	defer admin.Close()
	request := CreationRequest{SourceID: uuid.NewString(), Epoch: 1, Database: "voice_source_test"}
	require.NoError(t, control.QueryRow(ctx, `SELECT system_identifier::text FROM pg_control_system()`).Scan(&request.ControlSystemID))
	require.NoError(t, admin.QueryRow(ctx, `SELECT system_identifier::text FROM pg_control_system()`).Scan(&request.SourceSystemID))
	physical, err := CreateClosedSource(ctx, control, admin, request)
	require.NoError(t, err)
	_, err = control.Exec(ctx, `UPDATE user_custody_creation SET phase='MIGRATING' WHERE singleton`)
	require.NoError(t, err)
	_, err = admin.Exec(ctx, `ALTER DATABASE voice_source_test WITH ALLOW_CONNECTIONS true`)
	require.NoError(t, err)
	config.ConnConfig.Database = request.Database
	writer, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	defer writer.Close()
	_, err = writer.Exec(ctx, `CREATE TABLE prepared_recovery_test(value integer)`)
	require.NoError(t, err)
	_, err = writer.Exec(ctx, `BEGIN; INSERT INTO prepared_recovery_test VALUES(1); PREPARE TRANSACTION 'recovery_fixture'`)
	require.NoError(t, err)
	require.Error(t, RecoverInterruptedInitialization(ctx, control, admin, request, physical))
	var phase string
	var allowed bool
	require.NoError(t, control.QueryRow(ctx, `SELECT phase FROM user_custody_creation WHERE singleton`).Scan(&phase))
	require.Equal(t, "MIGRATING", phase)
	require.NoError(t, admin.QueryRow(ctx, `SELECT datallowconn FROM pg_database WHERE oid=$1`, physical.DatabaseOID).Scan(&allowed))
	require.False(t, allowed)
}

func TestRecoverInterruptedInitializationRejectsChangedPhysicalIdentity(t *testing.T) {
	ctx, control, admin, request := creationFixture(t)
	physical, err := CreateClosedSource(ctx, control, admin, request)
	require.NoError(t, err)
	_, err = control.Exec(ctx, `UPDATE user_custody_creation SET phase='MIGRATING' WHERE singleton`)
	require.NoError(t, err)
	_, err = admin.Exec(ctx, `ALTER DATABASE voice_source_test WITH ALLOW_CONNECTIONS true`)
	require.NoError(t, err)
	wrong := physical
	wrong.DatabaseOID++
	require.Error(t, RecoverInterruptedInitialization(ctx, control, admin, request, wrong))
	var allowed bool
	require.NoError(t, admin.QueryRow(ctx, `SELECT datallowconn FROM pg_database WHERE oid=$1`, physical.DatabaseOID).Scan(&allowed))
	require.True(t, allowed, "recovery must not fence a target with a different physical binding")
	require.NoError(t, RecoverInterruptedInitialization(ctx, control, admin, request, physical))
}

func TestRecoverInterruptedInitializationRejectsLiveLeader(t *testing.T) {
	ctx, control, admin, request := creationFixture(t)
	physical, err := CreateClosedSource(ctx, control, admin, request)
	require.NoError(t, err)
	_, err = control.Exec(ctx, `UPDATE user_custody_creation SET phase='MIGRATING' WHERE singleton`)
	require.NoError(t, err)
	_, err = admin.Exec(ctx, `ALTER DATABASE voice_source_test WITH ALLOW_CONNECTIONS true`)
	require.NoError(t, err)
	leader, err := control.Acquire(ctx)
	require.NoError(t, err)
	defer leader.Release()
	var locked bool
	require.NoError(t, leader.QueryRow(ctx, `SELECT pg_try_advisory_lock($1::bigint)`, leaderKey).Scan(&locked))
	require.True(t, locked)
	defer func() {
		var unlocked bool
		require.NoError(t, leader.QueryRow(ctx, `SELECT pg_advisory_unlock($1::bigint)`, leaderKey).Scan(&unlocked))
		require.True(t, unlocked)
	}()
	require.Error(t, RecoverInterruptedInitialization(ctx, control, admin, request, physical))
	var allowed bool
	require.NoError(t, admin.QueryRow(ctx, `SELECT datallowconn FROM pg_database WHERE oid=$1`, physical.DatabaseOID).Scan(&allowed))
	require.True(t, allowed, "recovery must not interrupt the current custodian")
}
