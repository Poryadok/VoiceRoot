//go:build integration

package nativeinstaller

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	"voice/backend/space/internal/store"
)

// This opt-in requires an owned disposable server with fast, cost-delayed
// autovacuum. It creates and drops only a previously absent space_db.
func TestMaintenanceWindowWithRealAutovacuum(t *testing.T) {
	dsn := os.Getenv("NATIVE_INSTALLER_WINDOW_TEST_ADMIN_URL")
	if dsn == "" {
		t.Skip("owned disposable PostgreSQL URL required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cfg, err := pgx.ParseConfig(dsn)
	require.NoError(t, err)
	require.Equal(t, "postgres", cfg.Database)
	const applicationName = "native-installer-window-regression"
	cfg.RuntimeParams["application_name"] = applicationName
	admin, err := pgx.ConnectConfig(ctx, cfg)
	require.NoError(t, err)
	defer func() { require.NoError(t, admin.Close(context.Background())) }()
	_, err = admin.Exec(ctx, `CREATE DATABASE space_db`)
	require.NoError(t, err, "requires a private server with absent space_db")
	defer func() {
		_, dropErr := admin.Exec(context.Background(), `DROP DATABASE space_db`)
		require.NoError(t, dropErr)
	}()
	_, err = admin.Exec(ctx, `SELECT pg_advisory_lock(hashtextextended('voice-space-native-installer',0))`)
	require.NoError(t, err)
	var pid int32
	var role string
	creation := &store.NativeInstallerSourceFacts{}
	require.NoError(t, admin.QueryRow(ctx, `SELECT pg_backend_pid(),session_user,(SELECT oid FROM pg_database WHERE datname='space_db')`).Scan(&pid, &role, &creation.DatabaseOID))
	check := func() error { return checkMaintenanceWindow(ctx, admin, pid, role, applicationName, creation) }
	require.NoError(t, check(), "fresh exclusive owner")
	targetCfg := cfg.Copy()
	targetCfg.Database = "space_db"
	target, err := pgx.ConnectConfig(ctx, targetCfg)
	require.NoError(t, err)
	defer func() { require.NoError(t, target.Close(context.Background())) }()
	_, err = target.Exec(ctx, `CREATE TABLE vacuum_probe WITH (autovacuum_vacuum_threshold=0,autovacuum_vacuum_scale_factor=0,autovacuum_analyze_threshold=1000000000) AS SELECT i,repeat(md5(i::text),32) payload FROM generate_series(1,50000) i; DELETE FROM vacuum_probe WHERE i%2=0`)
	require.NoError(t, err)
	var tableOID uint32
	require.NoError(t, target.QueryRow(ctx, `SELECT 'vacuum_probe'::regclass::oid`).Scan(&tableOID))
	var workerPID int32
	require.Eventually(t, func() bool {
		// Observe the real worker while it is still in the first half of the
		// cost-delayed probe-table scan, not an unrelated or near-finished worker.
		err := admin.QueryRow(ctx, `SELECT a.pid FROM pg_stat_activity a JOIN pg_stat_progress_vacuum p USING(pid) WHERE a.datname='space_db' AND a.backend_type='autovacuum worker' AND p.relid=$1 AND p.heap_blks_total>1 AND p.heap_blks_scanned < p.heap_blks_total/2 LIMIT 1`, tableOID).Scan(&workerPID)
		return err == nil
	}, 30*time.Second, 50*time.Millisecond, "requires real PostgreSQL autovacuum, never an application_name imitation")
	require.NoError(t, check(), "real autovacuum must preserve the drained application's maintenance window")
	var backendType, workerApplication string
	require.NoError(t, admin.QueryRow(ctx, `SELECT backend_type,application_name FROM pg_stat_activity WHERE pid=$1 AND datname='space_db'`, workerPID).Scan(&backendType, &workerApplication), "same real worker must remain present across the successful check")
	require.Equal(t, "autovacuum worker", backendType)
	require.NotEqual(t, applicationName, workerApplication)
	t.Logf("same actual %s PID %d remained across successful production owner check", backendType, workerPID)
	foreignCfg := targetCfg.Copy()
	foreignCfg.RuntimeParams["application_name"] = "foreign-writer"
	foreign, err := pgx.ConnectConfig(ctx, foreignCfg)
	require.NoError(t, err)
	defer func(c *pgx.Conn) { require.NoError(t, c.Close(context.Background())) }(foreign)
	_, err = foreign.Exec(ctx, `BEGIN; INSERT INTO vacuum_probe VALUES (50001,'foreign')`)
	require.NoError(t, err)
	require.ErrorContains(t, check(), "maintenance window lost", "actual foreign writer must block")
	require.NoError(t, foreign.Close(ctx))
	require.NoError(t, check(), "foreign disconnect restores the same owner")
	foreignCfg.RuntimeParams["application_name"] = ""
	foreign, err = pgx.ConnectConfig(ctx, foreignCfg)
	require.NoError(t, err)
	defer func(c *pgx.Conn) { require.NoError(t, c.Close(context.Background())) }(foreign)
	require.ErrorContains(t, check(), "maintenance window lost", "idle unnamed foreign client also blocks")
	require.NoError(t, foreign.Close(ctx))
	require.NoError(t, check())
	require.Error(t, checkMaintenanceWindow(ctx, admin, pid+1, role, applicationName, creation), "wrong owner PID")
	require.Error(t, checkMaintenanceWindow(ctx, admin, pid, role+"-wrong", applicationName, creation), "wrong owner role")
	require.Error(t, checkMaintenanceWindow(ctx, admin, pid, role, applicationName, &store.NativeInstallerSourceFacts{DatabaseOID: creation.DatabaseOID + 1}), "changed database OID")
	_, err = admin.Exec(ctx, `SET ROLE pg_read_all_stats`)
	require.NoError(t, err)
	require.Error(t, check(), "changed effective role")
	_, err = admin.Exec(ctx, `RESET ROLE`)
	require.NoError(t, err)
	require.NoError(t, check(), "restored effective role")
	_, err = admin.Exec(ctx, `SELECT pg_advisory_unlock_all()`)
	require.NoError(t, err)
	require.Error(t, check(), "lost owner lock")
	t.Log("foreign writer/unnamed idle client, wrong PID/role, effective role, changed OID and lost lock all HOLD")
}
