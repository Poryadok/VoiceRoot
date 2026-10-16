package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
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
}
