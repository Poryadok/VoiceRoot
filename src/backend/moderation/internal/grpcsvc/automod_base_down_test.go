package grpcsvc

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func baseDownSQL(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot(t), "src/backend/migrations/moderation_db/000006_async_automod.down.sql"))
	require.NoError(t, err)
	return string(data)
}
func TestAutoModBaseDownPreservesPermanentTerminalHistory(t *testing.T) {
	if testing.Short() {
		t.Skip("requires disposable PostgreSQL")
	}
	ctx := context.Background()
	pool := automodAdmissionDatabase(t)
	integrationtestApplySQL(t, ctx, pool, filepath.Join(repoRoot(t), "src/backend/migrations/moderation_db/000006_async_automod.up.sql"))
	subject := uuid.New()
	_, err := pool.Exec(ctx, `INSERT INTO automod_terminal_subjects(subject_id) VALUES($1)`, subject)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, baseDownSQL(t))
	require.ErrorContains(t, err, "automod history prevents destructive downgrade")
	var retained bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM automod_terminal_subjects WHERE subject_id=$1)`, subject).Scan(&retained))
	require.True(t, retained)
}
func TestAutoModBaseDownPreservesConcurrentFirstTerminal(t *testing.T) {
	if testing.Short() {
		t.Skip("requires disposable PostgreSQL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool := automodAdmissionDatabase(t)
	integrationtestApplySQL(t, ctx, pool, filepath.Join(repoRoot(t), "src/backend/migrations/moderation_db/000006_async_automod.up.sql"))
	writer, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = writer.Rollback(context.Background()) }()
	subject := uuid.New()
	_, err = writer.Exec(ctx, `INSERT INTO automod_terminal_subjects(subject_id) VALUES($1)`, subject)
	require.NoError(t, err)
	down, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer down.Release()
	defer func() { _, _ = down.Exec(context.Background(), "ROLLBACK") }()
	var pid int32
	require.NoError(t, down.QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&pid))
	statement := baseDownSQL(t)
	result := make(chan error, 1)
	go func() { _, err := down.Exec(ctx, statement); result <- err }()
	require.Eventually(t, func() bool {
		var waiting bool
		err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE pid=$1 AND wait_event_type='Lock')`, pid).Scan(&waiting)
		return err == nil && waiting
	}, 10*time.Second, 10*time.Millisecond)
	require.NoError(t, writer.Commit(ctx))
	require.ErrorContains(t, <-result, "automod history prevents destructive downgrade")
	_, err = down.Exec(ctx, "ROLLBACK")
	require.NoError(t, err)
	var retained bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM automod_terminal_subjects WHERE subject_id=$1)`, subject).Scan(&retained))
	require.True(t, retained)
}
func TestAutoModBaseDownEmptyStatePreservesHumanSanction(t *testing.T) {
	if testing.Short() {
		t.Skip("requires disposable PostgreSQL")
	}
	ctx := context.Background()
	pool := automodAdmissionDatabase(t)
	integrationtestApplySQL(t, ctx, pool, filepath.Join(repoRoot(t), "src/backend/migrations/moderation_db/000006_async_automod.up.sql"))
	id := uuid.New()
	_, err := pool.Exec(ctx, `INSERT INTO sanctions(id,target_account_id,type,reason,issued_by) VALUES($1,$2,'warning','human fixture',$3)`, id, uuid.New(), uuid.New())
	require.NoError(t, err)
	_, err = pool.Exec(ctx, baseDownSQL(t))
	require.NoError(t, err)
	var retained bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM sanctions WHERE id=$1 AND reason='human fixture') AND to_regclass('automod_terminal_subjects') IS NULL`, id).Scan(&retained))
	require.True(t, retained)
	integrationtestApplySQL(t, ctx, pool, filepath.Join(repoRoot(t), "src/backend/migrations/moderation_db/000006_async_automod.up.sql"))
}
