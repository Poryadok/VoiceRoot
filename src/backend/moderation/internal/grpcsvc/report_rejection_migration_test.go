package grpcsvc

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

func reportRejectionDown(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot(t), "src/backend/migrations/moderation_db/000012_report_target_rejections.down.sql"))
	require.NoError(t, err)
	return string(b)
}

func TestReportRejectionDownPreservesReceiptCommittedWhileWaiting(t *testing.T) {
	pool := reportIntakePool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	blocker, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = blocker.Rollback(context.Background()) }()
	_, err = blocker.Exec(ctx, `LOCK TABLE reports IN SHARE MODE`)
	require.NoError(t, err)
	conn, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer conn.Release()
	pid := conn.Conn().PgConn().PID()
	down := reportRejectionDown(t)
	done := make(chan error, 1)
	go func() {
		_, err := conn.Exec(ctx, down)
		done <- err
	}()
	require.Eventually(t, func() bool {
		var waiting bool
		err := pool.QueryRow(ctx, `SELECT cardinality(pg_blocking_pids($1))>0`, pid).Scan(&waiting)
		return err == nil && waiting
	}, 5*time.Second, 10*time.Millisecond, "observe the actual DOWN lock wait before writing the receipt")
	id := uuid.New()
	_, err = pool.Exec(ctx, `INSERT INTO report_target_rejections(report_id,decision,provenance,schema_version) VALUES($1,'MISSING','OWNER_VALIDATION',1)`, id)
	require.NoError(t, err, "receipt-only recovery may commit without a reports row")
	require.NoError(t, blocker.Commit(ctx))
	require.Error(t, <-done, "DOWN must recheck retained receipts after obtaining its write-excluding locks")
	var retained uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT report_id FROM report_target_rejections WHERE report_id=$1`, id).Scan(&retained))
	require.Equal(t, id, retained)
}

func TestReportRejectionDownRejectsOldSnapshot(t *testing.T) {
	pool := reportIntakePool(t)
	ctx := context.Background()
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()
	var count int
	require.NoError(t, tx.QueryRow(ctx, `SELECT count(*) FROM report_target_rejections`).Scan(&count))
	require.Zero(t, count)
	id := uuid.New()
	_, err = pool.Exec(ctx, `INSERT INTO report_target_rejections(report_id,decision,provenance,schema_version) VALUES($1,'DELETED','LEGACY_RECORDED_DECISION',1)`, id)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, reportRejectionDown(t))
	require.Error(t, err, "a preexisting repeatable-read snapshot cannot certify an empty downgrade")
	require.NoError(t, tx.Rollback(ctx))
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM report_target_rejections WHERE report_id=$1`, id).Scan(&count))
	require.Equal(t, 1, count)
}

func TestReportRejectionDownAllowsEmptyExpand(t *testing.T) {
	pool := reportIntakePool(t)
	ctx := context.Background()
	_, err := pool.Exec(ctx, reportRejectionDown(t))
	require.NoError(t, err)
	var exists bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT to_regclass('report_target_rejections') IS NOT NULL`).Scan(&exists))
	require.False(t, exists)
	integrationtestApplySQL(t, ctx, pool, filepath.Join(repoRoot(t), "src/backend/migrations/moderation_db/000012_report_target_rejections.up.sql"))
}
