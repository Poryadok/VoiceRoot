package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
)

func TestChatDeletedOutboxMigrationDownRefusesAndPreservesRetainedRows(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	ctx := context.Background()
	pool := startChatDBForStoreTest(t, ctx)
	applyChatMigrationsForStoreTest(t, ctx, pool)
	applyChatMigrationFile(t, ctx, pool, "000021_chat_deleted_event_outbox.up.sql")

	eventID, chatID, spaceID, operationID, manifestID := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	eventBytes := []byte("immutable pending chat.deleted envelope")
	_, err := pool.Exec(ctx, `INSERT INTO chat_deleted_event_outbox
		(event_id,chat_id,space_id,deletion_operation_id,generation,manifest_id,manifest_sha256,event_bytes,occurred_at)
		VALUES($1,$2,$3,$4,2,$5,$6,$7,clock_timestamp())`, eventID, chatID, spaceID, operationID, manifestID, make([]byte, 32), eventBytes)
	require.NoError(t, err)
	deliveredID, deliveredBytes := uuid.New(), []byte("immutable delivered chat.deleted envelope")
	_, err = pool.Exec(ctx, `INSERT INTO chat_deleted_event_outbox
		(event_id,chat_id,space_id,deletion_operation_id,generation,manifest_id,manifest_sha256,event_bytes,occurred_at,published_at)
		VALUES($1,$2,$3,$4,2,$5,$6,$7,clock_timestamp(),clock_timestamp())`, deliveredID, uuid.New(), spaceID, uuid.New(), uuid.New(), make([]byte, 32), deliveredBytes)
	require.NoError(t, err)

	downBytes, err := os.ReadFile(filepath.Join(chatRepoRoot(t), "src", "backend", "migrations", "chat_db", "000021_chat_deleted_event_outbox.down.sql"))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(downBytes))
	require.Error(t, err, "DOWN must preserve retained outbox evidence")
	var remaining []byte
	require.NoError(t, pool.QueryRow(ctx, `SELECT event_bytes FROM chat_deleted_event_outbox WHERE event_id=$1`, eventID).Scan(&remaining))
	require.Equal(t, eventBytes, remaining)
	require.NoError(t, pool.QueryRow(ctx, `SELECT event_bytes FROM chat_deleted_event_outbox WHERE event_id=$1`, deliveredID).Scan(&remaining))
	require.Equal(t, deliveredBytes, remaining)
	var rows int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM chat_deleted_event_outbox`).Scan(&rows))
	require.Equal(t, 2, rows)
	var tableExists bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT to_regclass('chat_deleted_event_outbox') IS NOT NULL`).Scan(&tableExists))
	require.True(t, tableExists)
}

func TestChatDeletedOutboxMigrationDownAllowsEmptyTable(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	ctx := context.Background()
	pool := startChatDBForStoreTest(t, ctx)
	applyChatMigrationsForStoreTest(t, ctx, pool)
	applyChatMigrationFile(t, ctx, pool, "000021_chat_deleted_event_outbox.up.sql")
	applyChatMigrationFile(t, ctx, pool, "000021_chat_deleted_event_outbox.down.sql")
	var tableExists bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT to_regclass('chat_deleted_event_outbox') IS NOT NULL`).Scan(&tableExists))
	require.False(t, tableExists)
	applyChatMigrationFile(t, ctx, pool, "000021_chat_deleted_event_outbox.up.sql")
	var rows int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM chat_deleted_event_outbox`).Scan(&rows))
	require.Zero(t, rows)
}

func TestChatDeletedOutboxMigrationDownPreservesConcurrentFirstWriter(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	pool := startChatDBForStoreTest(t, ctx)
	applyChatMigrationFile(t, ctx, pool, "000021_chat_deleted_event_outbox.up.sql")

	writer, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = writer.Rollback(context.Background()) }()
	eventID := uuid.New()
	eventBytes := []byte("immutable first pending chat.deleted envelope")
	_, err = writer.Exec(ctx, `INSERT INTO chat_deleted_event_outbox
		(event_id,chat_id,space_id,deletion_operation_id,generation,manifest_id,manifest_sha256,event_bytes,occurred_at)
		VALUES($1,$2,$3,$4,2,$5,$6,$7,clock_timestamp())`, eventID, uuid.New(), uuid.New(), uuid.New(), uuid.New(), make([]byte, 32), eventBytes)
	require.NoError(t, err, "first valid outbox write remains uncommitted")

	down, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer down.Release()
	downBytes, err := os.ReadFile(filepath.Join(chatRepoRoot(t), "src", "backend", "migrations", "chat_db", "000021_chat_deleted_event_outbox.down.sql"))
	require.NoError(t, err)
	done := make(chan error, 1)
	go func() {
		_, downErr := down.Exec(ctx, string(downBytes))
		done <- downErr
	}()
	// Observe the actual migration waiting on the writer, rather than relying on
	// a sleep or adding a synchronization hook to the production SQL.
	require.Eventually(t, func() bool {
		var waiting bool
		err := writer.QueryRow(ctx, `SELECT EXISTS (
			SELECT 1 FROM pg_locks WHERE pid=$1 AND relation='chat_deleted_event_outbox'::regclass
			AND mode='AccessExclusiveLock' AND NOT granted)`, down.Conn().PgConn().PID()).Scan(&waiting)
		return err == nil && waiting
	}, 5*time.Second, 10*time.Millisecond, "DOWN reaches a conflicting table lock while the first writer is open")
	require.NoError(t, writer.Commit(ctx), "the first pending event commits successfully")
	downErr := <-done
	var pgErr *pgconn.PgError
	require.ErrorAs(t, downErr, &pgErr, "DOWN must refuse the newly committed row")
	require.Equal(t, "P0001", pgErr.Code)
	require.Contains(t, pgErr.Message, "retained evidence")
	var remaining []byte
	require.NoError(t, pool.QueryRow(ctx, `SELECT event_bytes FROM chat_deleted_event_outbox WHERE event_id=$1 AND published_at IS NULL`, eventID).Scan(&remaining))
	require.Equal(t, eventBytes, remaining, "pending delivery bytes survive rollback")
}

func TestChatDeletedOutboxMigrationDownRejectsStaleTransactionSnapshots(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	for _, isolation := range []pgx.TxIsoLevel{pgx.RepeatableRead, pgx.Serializable} {
		t.Run(string(isolation), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			pool := startChatDBForStoreTest(t, ctx)
			applyChatMigrationFile(t, ctx, pool, "000021_chat_deleted_event_outbox.up.sql")
			down, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: isolation})
			require.NoError(t, err)
			defer func() { _ = down.Rollback(context.Background()) }()
			var rows int
			require.NoError(t, down.QueryRow(ctx, `SELECT count(*) FROM chat_deleted_event_outbox`).Scan(&rows))
			require.Zero(t, rows, "establish the rollback transaction's empty snapshot")
			eventID := uuid.New()
			eventBytes := []byte("pending event committed after the rollback snapshot")
			_, err = pool.Exec(ctx, `INSERT INTO chat_deleted_event_outbox
				(event_id,chat_id,space_id,deletion_operation_id,generation,manifest_id,manifest_sha256,event_bytes,occurred_at)
				VALUES($1,$2,$3,$4,2,$5,$6,$7,clock_timestamp())`, eventID, uuid.New(), uuid.New(), uuid.New(), uuid.New(), make([]byte, 32), eventBytes)
			require.NoError(t, err, "another connection commits the first pending event")
			downBytes, err := os.ReadFile(filepath.Join(chatRepoRoot(t), "src", "backend", "migrations", "chat_db", "000021_chat_deleted_event_outbox.down.sql"))
			require.NoError(t, err)
			_, err = down.Exec(ctx, string(downBytes))
			var pgErr *pgconn.PgError
			require.ErrorAs(t, err, &pgErr, "a stale transaction snapshot cannot authorize DROP")
			require.Equal(t, "55000", pgErr.Code)
			require.Contains(t, pgErr.Message, "requires read committed isolation")
			require.NoError(t, down.Rollback(ctx))
			var remaining []byte
			require.NoError(t, pool.QueryRow(ctx, `SELECT event_bytes FROM chat_deleted_event_outbox WHERE event_id=$1 AND published_at IS NULL`, eventID).Scan(&remaining))
			require.Equal(t, eventBytes, remaining)
		})
	}
}
