package store_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

func targetRetirementMigration(t *testing.T, direction string) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	raw, e := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "..", "migrations", "bot_db", "000016_purge_target_retirement."+direction+".sql"))
	require.NoError(t, e)
	return string(raw)
}

func TestLocalP3TargetMigrationIsolationAndActualRetainedResult(t *testing.T) {
	if testing.Short() {
		t.Skip("real PostgreSQL016 DOWN isolation")
	}
	t.Run("empty default RC roundtrip", func(t *testing.T) {
		st := startBotLifecycleStore(t)
		_, e := st.Pool.Exec(t.Context(), targetRetirementMigration(t, "down"))
		require.NoError(t, e)
		_, e = st.Pool.Exec(t.Context(), targetRetirementMigration(t, "up"))
		require.NoError(t, e)
	})
	t.Run("old RR cannot erase later local result", func(t *testing.T) {
		st, bot, _, c, _ := retirementFixture(t)
		ctx := t.Context()
		tx, e := st.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
		require.NoError(t, e)
		defer func() { _ = tx.Rollback(context.Background()) }()
		var n int
		require.NoError(t, tx.QueryRow(ctx, `SELECT count(*) FROM bot_uninstall_installations WHERE disposition='LOCAL_P3_COVERED'`).Scan(&n))
		require.Zero(t, n)
		saveRetirementEvidence(t, st, c, bot, true)
		_, e = st.RetireBotPurgeHandoff(ctx, c)
		require.NoError(t, e)
		_, e = tx.Exec(ctx, targetRetirementMigration(t, "down"))
		require.ErrorContains(t, e, "READ COMMITTED")
		require.NoError(t, tx.Rollback(ctx))
		_, e = st.Pool.Exec(ctx, targetRetirementMigration(t, "down"))
		require.ErrorContains(t, e, "retained local results")
		require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM bot_uninstall_installations WHERE disposition='LOCAL_P3_COVERED'`).Scan(&n))
		require.Equal(t, 1, n)
	})
}

func TestLocalP3TargetDownWaitsForActualFirstRetirementWriter(t *testing.T) {
	if testing.Short() {
		t.Skip("real first retirement transaction versus016 DOWN")
	}
	st, bot, _, c, _ := retirementFixture(t)
	ctx := t.Context()
	saveRetirementEvidence(t, st, c, bot, true)
	// Test-only pause AFTER the real target/effect transition but BEFORE own
	// receipt insert/commit. No production capability or participant is forged.
	_, e := st.Pool.Exec(ctx, `CREATE FUNCTION hold_local_p3_receipt() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN PERFORM pg_advisory_xact_lock(247016); RETURN NEW; END $$;CREATE TRIGGER zz_hold_local_p3_receipt BEFORE INSERT ON bot_space_lifecycle_purge_receipts FOR EACH ROW EXECUTE FUNCTION hold_local_p3_receipt()`)
	require.NoError(t, e)
	blocker, e := st.Pool.Acquire(ctx)
	require.NoError(t, e)
	defer blocker.Release()
	_, e = blocker.Exec(ctx, `SELECT pg_advisory_lock(247016)`)
	require.NoError(t, e)
	defer func() { _, _ = blocker.Exec(context.Background(), `SELECT pg_advisory_unlock(247016)`) }()
	retired := make(chan error, 1)
	go func() { _, e := st.RetireBotPurgeHandoff(ctx, c); retired <- e }()
	require.Eventually(t, func() bool {
		var waiting bool
		return st.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE query LIKE 'INSERT INTO bot_space_lifecycle_purge_receipts%' AND wait_event='advisory')`).Scan(&waiting) == nil && waiting
	}, 5*time.Second, 10*time.Millisecond)
	downSQL := targetRetirementMigration(t, "down")
	downed := make(chan error, 1)
	go func() { _, e := st.Pool.Exec(ctx, downSQL); downed <- e }()
	require.Eventually(t, func() bool {
		var waiting bool
		// DOWN locks the closed table list in order; the real retirement also
		// reads the global family before its ordinary target. Observe its actual
		// blocking chain, rather than assuming the second table is first held.
		return st.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_locks l WHERE l.relation IN ('bot_deletion_operations'::regclass,'bot_uninstall_operations'::regclass,'bot_deletion_installations'::regclass,'bot_uninstall_installations'::regclass,'bot_deletion_effects'::regclass,'bot_uninstall_effects'::regclass) AND l.mode='AccessExclusiveLock' AND NOT l.granted AND EXISTS(SELECT 1 FROM pg_stat_activity a WHERE a.pid=ANY(pg_blocking_pids(l.pid)) AND a.query LIKE 'INSERT INTO bot_space_lifecycle_purge_receipts%' AND a.wait_event='advisory'))`).Scan(&waiting) == nil && waiting
	}, 5*time.Second, 10*time.Millisecond)
	_, e = blocker.Exec(ctx, `SELECT pg_advisory_unlock(247016)`)
	require.NoError(t, e)
	require.NoError(t, <-retired)
	require.ErrorContains(t, <-downed, "retained local results")
}
