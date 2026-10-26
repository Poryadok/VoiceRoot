package currentauthority

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"voice/backend/pkg/integrationtest"
)

func cutoverDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Join(filepath.Dir(file), "../../../migrations/subscription_db")
	pool := integrationtest.StartPostgres(t, context.Background(), "authority_cutover", "")
	for _, migration := range []string{"000001_init", "000002_grace_reminders", "000003_space_lifecycle_provider_dedup", "000004_entitlement_outbox", "000005_provider_lifecycle", "000006_grace_reminder_outbox", "000007_authority_fence"} {
		body, err := os.ReadFile(filepath.Join(root, migration+".up.sql"))
		require.NoError(t, err)
		_, err = pool.Exec(context.Background(), string(body))
		require.NoError(t, err)
	}
	_, err := pool.Exec(context.Background(), `CREATE ROLE old_runtime LOGIN PASSWORD 'fixture-only'; CREATE ROLE old_group; GRANT old_group TO old_runtime;
CREATE ROLE canonical_writer LOGIN PASSWORD 'fixture-only'; CREATE ROLE canonical_reader LOGIN PASSWORD 'fixture-only';
GRANT USAGE ON SCHEMA public TO old_runtime;
GRANT SELECT,INSERT,UPDATE,DELETE ON ALL TABLES IN SCHEMA public TO old_group`)
	require.NoError(t, err)
	return pool
}

// These are actual PostgreSQL logins, not SET ROLE on a privileged session.
func login(t *testing.T, admin *pgxpool.Pool, role string) *pgx.Conn {
	t.Helper()
	cfg := admin.Config().ConnConfig.Copy()
	cfg.User, cfg.Password = role, "fixture-only"
	conn, err := pgx.ConnectConfig(context.Background(), cfg)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close(context.Background())) })
	return conn
}

func fenceRequest() FenceRequest {
	return FenceRequest{Generation: uuid.NewString(), SourceCommit: strings.Repeat("a", 40),
		WriterRole: "canonical_writer", ReaderRole: "canonical_reader", LegacyRoles: []string{"old_group", "old_runtime"}}
}

func TestFenceDrainsOldWriterAndRejectsExistingAndRestartedConnections(t *testing.T) {
	pool := cutoverDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	old := login(t, pool, "old_runtime")
	tx, err := old.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(context.Background()) }()
	_, err = tx.Exec(ctx, "UPDATE subscriptions SET updated_at=clock_timestamp()")
	require.NoError(t, err)
	request := fenceRequest()
	finished := make(chan error, 1)
	go func() { finished <- InstallFence(ctx, pool, request) }()
	// Prove causal blocking at PostgreSQL, without timing a sleep to guess it.
	require.Eventually(t, func() bool {
		var waiting bool
		err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE $1=ANY(pg_blocking_pids(pid)))`, old.PgConn().PID()).Scan(&waiting)
		return err == nil && waiting
	}, 5*time.Second, 10*time.Millisecond)
	require.NoError(t, tx.Commit(ctx))
	require.NoError(t, <-finished)
	_, err = old.Exec(ctx, "UPDATE subscriptions SET updated_at=clock_timestamp()")
	require.Error(t, err, "a pre-existing legacy connection must lose authority")
	restarted := login(t, pool, "old_runtime")
	_, err = restarted.Exec(ctx, "DELETE FROM subscription_entitlement_aggregates")
	require.Error(t, err, "a restarted old binary must not regain canonical writes")
	writer := login(t, pool, "canonical_writer")
	_, err = writer.Exec(ctx, "UPDATE subscription_entitlement_aggregates SET aggregate_revision=aggregate_revision")
	require.NoError(t, err)
	_, err = writer.Exec(ctx, "UPDATE subscription_authority_fence SET source_commit=source_commit")
	require.Error(t, err, "runtime writer cannot certify its own fence")
	reader := login(t, pool, "canonical_reader")
	_, err = reader.Exec(ctx, "SELECT * FROM subscription_entitlement_aggregates")
	require.NoError(t, err)
	_, err = reader.Exec(ctx, "DELETE FROM subscription_entitlement_aggregates")
	require.Error(t, err)
	var state string
	require.NoError(t, pool.QueryRow(ctx, "SELECT state FROM subscription_authority_fence").Scan(&state))
	require.Equal(t, "FENCED", state, "a privilege fence does not certify any CURRENT account")
	require.NoError(t, InstallFence(ctx, pool, request), "identical cutover is idempotent")
	request.SourceCommit = strings.Repeat("b", 40)
	require.ErrorIs(t, InstallFence(ctx, pool, request), ErrFenceConflict)
}

func TestFenceRejectsInheritedWritesAndUnsafeRolesAtomically(t *testing.T) {
	pool := cutoverDB(t)
	ctx := context.Background()
	request := fenceRequest()
	request.LegacyRoles = []string{"old_runtime"}
	require.ErrorIs(t, InstallFence(ctx, pool, request), ErrUnsafeRole, "unlisted inherited group retains real DML")
	var count int
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM subscription_authority_fence").Scan(&count))
	require.Zero(t, count)
	old := login(t, pool, "old_runtime")
	_, err := old.Exec(ctx, "UPDATE subscriptions SET updated_at=clock_timestamp()")
	require.NoError(t, err, "failed fence rolls back all privilege mutations")
	request = fenceRequest()
	request.WriterRole = pool.Config().ConnConfig.User
	require.ErrorIs(t, InstallFence(ctx, pool, request), ErrUnsafeRole, "superuser/owner is not a serving writer")
	request = fenceRequest()
	_, err = pool.Exec(ctx, "GRANT canonical_writer TO old_runtime")
	require.NoError(t, err)
	require.ErrorIs(t, InstallFence(ctx, pool, request), ErrUnsafeRole, "old runtime cannot inherit replacement writer")
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM subscription_authority_fence").Scan(&count))
	require.Zero(t, count)
}

func TestFenceCannotCertifyRuntimeWithInheritedControlOrReaderWrites(t *testing.T) {
	for _, target := range []string{"canonical_writer", "canonical_reader"} {
		t.Run(target, func(t *testing.T) {
			pool := cutoverDB(t)
			ctx := context.Background()
			_, err := pool.Exec(ctx, "CREATE ROLE unintended_grants; GRANT unintended_grants TO "+pgx.Identifier{target}.Sanitize()+"; GRANT UPDATE ON subscription_authority_fence TO unintended_grants")
			require.NoError(t, err)
			require.ErrorIs(t, InstallFence(ctx, pool, fenceRequest()), ErrUnsafeRole, "control rights through any inherited role must fail")
		})
	}
	pool := cutoverDB(t)
	ctx := context.Background()
	_, err := pool.Exec(ctx, "CREATE ROLE unintended_grants; GRANT unintended_grants TO canonical_reader; GRANT DELETE ON subscription_entitlement_aggregates TO unintended_grants")
	require.NoError(t, err)
	require.ErrorIs(t, InstallFence(ctx, pool, fenceRequest()), ErrUnsafeRole, "a reader cannot inherit canonical DML")
}

func TestFenceReplayRevalidatesEffectivePrivileges(t *testing.T) {
	pool := cutoverDB(t)
	ctx := context.Background()
	request := fenceRequest()
	require.NoError(t, InstallFence(ctx, pool, request))
	_, err := pool.Exec(ctx, "GRANT DELETE ON subscriptions TO old_group")
	require.NoError(t, err)
	require.ErrorIs(t, InstallFence(ctx, pool, request), ErrUnsafeRole, "saved request hash cannot hide external role drift")
	_, err = pool.Exec(ctx, "REVOKE DELETE ON subscriptions FROM old_group; ALTER ROLE canonical_writer CREATEROLE")
	require.NoError(t, err)
	require.ErrorIs(t, InstallFence(ctx, pool, request), ErrUnsafeRole)
}

func TestFenceRejectsSetRoleOnlyCapabilitiesAndSchemaOwnership(t *testing.T) {
	for _, role := range []string{"old_runtime", "canonical_reader", "canonical_writer"} {
		t.Run(role, func(t *testing.T) {
			pool := cutoverDB(t)
			ctx := context.Background()
			_, err := pool.Exec(ctx, "CREATE ROLE set_only; GRANT set_only TO "+pgx.Identifier{role}.Sanitize()+" WITH INHERIT FALSE, SET TRUE; GRANT USAGE ON SCHEMA public TO set_only; GRANT DELETE ON subscriptions TO set_only; GRANT UPDATE ON subscription_authority_fence TO set_only")
			require.NoError(t, err)
			conn := login(t, pool, role)
			_, err = conn.Exec(ctx, "SET ROLE set_only; DELETE FROM subscriptions; UPDATE subscription_authority_fence SET source_commit='bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb'; RESET ROLE")
			require.NoError(t, err, "real login proves SET-only authority exists before fence")
			require.ErrorIs(t, InstallFence(ctx, pool, fenceRequest()), ErrUnsafeRole)
			var count int
			require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM subscription_authority_fence").Scan(&count))
			require.Zero(t, count)
		})
	}
	for _, control := range []string{"fence", "schema"} {
		t.Run(control, func(t *testing.T) {
			pool := cutoverDB(t)
			ctx := context.Background()
			_, err := pool.Exec(ctx, "CREATE ROLE control_owner; GRANT control_owner TO old_runtime")
			require.NoError(t, err)
			if control == "fence" {
				_, err = pool.Exec(ctx, "GRANT UPDATE ON subscription_authority_fence TO control_owner")
			} else {
				_, err = pool.Exec(ctx, "ALTER SCHEMA public OWNER TO control_owner")
			}
			require.NoError(t, err)
			require.ErrorIs(t, InstallFence(ctx, pool, fenceRequest()), ErrUnsafeRole)
		})
	}
}

func TestFenceRejectsOmittedWritableLogin(t *testing.T) {
	pool := cutoverDB(t)
	ctx := context.Background()
	_, err := pool.Exec(ctx, "CREATE ROLE omitted_runtime LOGIN; GRANT old_group TO omitted_runtime WITH INHERIT FALSE, SET TRUE")
	require.NoError(t, err)
	// old_group is in the manifest and revoked, so this descendant is harmless.
	require.NoError(t, InstallFence(ctx, pool, fenceRequest()))
	_, err = pool.Exec(ctx, "CREATE ROLE unlisted_writer; GRANT DELETE ON subscriptions TO unlisted_writer; GRANT unlisted_writer TO omitted_runtime WITH INHERIT FALSE, SET TRUE")
	require.NoError(t, err)
	var generation string
	require.NoError(t, pool.QueryRow(ctx, "SELECT generation::text FROM subscription_authority_fence").Scan(&generation))
	request := fenceRequest()
	request.Generation = generation
	require.ErrorIs(t, InstallFence(ctx, pool, request), ErrUnsafeRole, "an omitted writable login invalidates the source-role inventory")
}
