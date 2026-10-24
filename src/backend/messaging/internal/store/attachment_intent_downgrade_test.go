package store

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func attachmentDowngradePool(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("MSG26_TEST_DATABASE_URL")
	if dsn == "" {
		return startPostgresForStoreTest(t, ctx)
	}
	admin, err := pgx.Connect(ctx, dsn)
	require.NoError(t, err)
	name := "msg26_" + uuid.New().String()[:8]
	_, err = admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize())
	require.NoError(t, err)
	config, err := pgxpool.ParseConfig(dsn)
	require.NoError(t, err)
	config.ConnConfig.Database = name
	config.MaxConns = 4
	pool, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	t.Cleanup(func() {
		pool.Close()
		cleanup, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, dropErr := admin.Exec(cleanup, "DROP DATABASE "+pgx.Identifier{name}.Sanitize()+" WITH (FORCE)")
		require.NoError(t, dropErr)
		require.NoError(t, admin.Close(cleanup))
	})
	return pool
}

func attachmentDowngradeSQL(t *testing.T, direction string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot(t), "src/backend/migrations/messaging_db/000026_attachment_send_intents."+direction+".sql"))
	require.NoError(t, err)
	return string(data)
}

const attachmentDowngradeInsert = `INSERT INTO messaging_attachment_send_intents
 (operation_id,message_id,chat_id,scope_space_id,request_sha256,references_bytes,state)
 VALUES ('00000000-0000-4000-8000-000000000001','00000000-0000-4000-8000-000000000002',
 '00000000-0000-4000-8000-000000000003','00000000-0000-4000-8000-000000000004',
 decode(repeat('ab',32),'hex'),decode('010203','hex'),$1)`

func requireAttachmentDowngradeBinding(t *testing.T, ctx context.Context, pool *pgxpool.Pool, state string) {
	t.Helper()
	var saved string
	require.NoError(t, pool.QueryRow(ctx, `SELECT operation_id::text||'/'||message_id::text||'/'||chat_id::text||'/'||scope_space_id::text||'/'||encode(request_sha256,'hex')||'/'||encode(references_bytes,'hex')||'/'||state FROM messaging_attachment_send_intents`).Scan(&saved))
	require.Equal(t, fmt.Sprintf("00000000-0000-4000-8000-000000000001/00000000-0000-4000-8000-000000000002/00000000-0000-4000-8000-000000000003/00000000-0000-4000-8000-000000000004/%s/010203/%s", "abababababababababababababababababababababababababababababababab", state), saved)
}

func TestAttachmentIntentDowngradeConcurrentFirstWriter(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool := attachmentDowngradePool(t, ctx)
	_, err := pool.Exec(ctx, attachmentDowngradeSQL(t, "up"))
	require.NoError(t, err)
	writer, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = writer.Rollback(context.Background()) }()
	_, err = writer.Exec(ctx, attachmentDowngradeInsert, "PENDING")
	require.NoError(t, err)
	migrator, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer migrator.Release()
	pid := migrator.Conn().PgConn().PID()
	down := attachmentDowngradeSQL(t, "down")
	downCtx, stop := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() {
		_, execErr := migrator.Exec(downCtx, down)
		done <- execErr
	}()
	joined := false
	defer func() {
		stop()
		if !joined {
			<-done
		}
	}()
	require.Eventually(t, func() bool {
		var blocked bool
		err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_locks WHERE pid=$1 AND relation='messaging_attachment_send_intents'::regclass AND NOT granted AND mode='AccessExclusiveLock')`, pid).Scan(&blocked)
		return err == nil && blocked
	}, 5*time.Second, 10*time.Millisecond, "DOWN must reach its actual relation lock wait")
	require.NoError(t, writer.Commit(ctx))
	err = <-done
	joined = true
	require.ErrorContains(t, err, "rollback requires drained, exported evidence")
	requireAttachmentDowngradeBinding(t, ctx, pool, "PENDING")
}

func TestAttachmentIntentDowngradeRejectsOldSnapshot(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool := attachmentDowngradePool(t, ctx)
	_, err := pool.Exec(ctx, attachmentDowngradeSQL(t, "up"))
	require.NoError(t, err)
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(context.Background()) }()
	var count int
	require.NoError(t, tx.QueryRow(ctx, `SELECT count(*) FROM messaging_attachment_send_intents`).Scan(&count))
	require.Zero(t, count)
	_, err = pool.Exec(ctx, attachmentDowngradeInsert, "PENDING")
	require.NoError(t, err)
	_, err = tx.Exec(ctx, attachmentDowngradeSQL(t, "down"))
	require.ErrorContains(t, err, "read committed")
	require.NoError(t, tx.Rollback(ctx))
	requireAttachmentDowngradeBinding(t, ctx, pool, "PENDING")
}

func TestAttachmentIntentDowngradeControls(t *testing.T) {
	for _, state := range []string{"PENDING", "COMMITTED", "RELEASED", "empty"} {
		t.Run(state, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			pool := attachmentDowngradePool(t, ctx)
			_, err := pool.Exec(ctx, attachmentDowngradeSQL(t, "up"))
			require.NoError(t, err)
			if state != "empty" {
				_, err = pool.Exec(ctx, attachmentDowngradeInsert, state)
				require.NoError(t, err)
			}
			_, err = pool.Exec(ctx, attachmentDowngradeSQL(t, "down"))
			if state != "empty" {
				require.ErrorContains(t, err, "rollback requires drained, exported evidence")
				requireAttachmentDowngradeBinding(t, ctx, pool, state)
				return
			}
			require.NoError(t, err)
			_, err = pool.Exec(ctx, attachmentDowngradeSQL(t, "up"))
			require.NoError(t, err)
			_, err = pool.Exec(ctx, attachmentDowngradeInsert, "PENDING")
			require.NoError(t, err)
			requireAttachmentDowngradeBinding(t, ctx, pool, "PENDING")
		})
	}
}
