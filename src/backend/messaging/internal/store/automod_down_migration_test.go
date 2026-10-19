package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAutoModDownPreservesConcurrentFirstKey(t *testing.T) {
	if testing.Short() {
		t.Skip("requires disposable PostgreSQL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool := automodSourceDatabase(t)
	writer, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = writer.Rollback(context.Background()) }()
	_, err = writer.Exec(ctx, `INSERT INTO automod_hmac_keys(key_id,fingerprint) VALUES('first',decode(repeat('ab',32),'hex'))`)
	require.NoError(t, err)
	down, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer down.Release()
	defer func() { _, _ = down.Exec(context.Background(), "ROLLBACK") }()
	var pid int32
	require.NoError(t, down.QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&pid))
	data, err := os.ReadFile(filepath.Join(repoRoot(t), "src/backend/migrations/messaging_db/000031_async_automod.down.sql"))
	require.NoError(t, err)
	result := make(chan error, 1)
	go func() {
		_, downErr := down.Exec(ctx, string(data))
		result <- downErr
	}()
	require.Eventually(t, func() bool {
		var waiting bool
		err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE pid=$1 AND wait_event_type='Lock')`, pid).Scan(&waiting)
		return err == nil && waiting
	}, 10*time.Second, 10*time.Millisecond)
	require.NoError(t, writer.Commit(ctx))
	require.ErrorContains(t, <-result, "cannot discard automod source, key or terminal history")
	_, err = down.Exec(ctx, "ROLLBACK")
	require.NoError(t, err)
	var preserved bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT fingerprint=decode(repeat('ab',32),'hex') FROM automod_hmac_keys WHERE key_id='first'`).Scan(&preserved))
	require.True(t, preserved)
	var guarded bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_trigger WHERE tgrelid='messages'::regclass AND tgname='automod_message_coverage')`).Scan(&guarded))
	require.True(t, guarded)
}

func TestAutoModDownAllowsEmptyInactiveProducer(t *testing.T) {
	if testing.Short() {
		t.Skip("requires disposable PostgreSQL")
	}
	ctx := context.Background()
	pool := automodSourceDatabase(t)
	data, err := os.ReadFile(filepath.Join(repoRoot(t), "src/backend/migrations/messaging_db/000031_async_automod.down.sql"))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(data))
	require.NoError(t, err)
	var absent bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT to_regclass('automod_hmac_keys') IS NULL AND to_regclass('messages') IS NOT NULL`).Scan(&absent))
	require.True(t, absent)
}
