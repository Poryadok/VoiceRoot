package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAttachmentIntentEnabledSchemaPreflightAndRollback(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForStoreTest(t, ctx)
	require.ErrorIs(t, RequireAttachmentIntentSchema(ctx, pool), ErrAttachmentIntentSchema)
	seedMessagingSchema(t, ctx, pool)
	// A present table does not make an interrupted or unrecorded migration ready.
	require.ErrorIs(t, RequireAttachmentIntentSchema(ctx, pool), ErrAttachmentIntentSchema)
	_, err := pool.Exec(ctx, `CREATE TABLE schema_migrations(version bigint NOT NULL PRIMARY KEY,dirty boolean NOT NULL);INSERT INTO schema_migrations VALUES(25,false)`)
	require.NoError(t, err)
	require.ErrorIs(t, RequireAttachmentIntentSchema(ctx, pool), ErrAttachmentIntentSchema)
	_, err = pool.Exec(ctx, `UPDATE schema_migrations SET version=26,dirty=true`)
	require.NoError(t, err)
	require.ErrorIs(t, RequireAttachmentIntentSchema(ctx, pool), ErrAttachmentIntentSchema)
	_, err = pool.Exec(ctx, `UPDATE schema_migrations SET dirty=false`)
	require.NoError(t, err)
	require.NoError(t, RequireAttachmentIntentSchema(ctx, pool))
	_, err = pool.Exec(ctx, `ALTER TABLE messaging_attachment_send_intents DROP COLUMN references_bytes`)
	require.NoError(t, err)
	require.ErrorIs(t, RequireAttachmentIntentSchema(ctx, pool), ErrAttachmentIntentSchema)
	_, err = pool.Exec(ctx, `ALTER TABLE messaging_attachment_send_intents ADD COLUMN references_bytes text NOT NULL DEFAULT ''`)
	require.NoError(t, err)
	require.ErrorIs(t, RequireAttachmentIntentSchema(ctx, pool), ErrAttachmentIntentSchema)
	_, err = pool.Exec(ctx, `ALTER TABLE messaging_attachment_send_intents DROP COLUMN references_bytes;ALTER TABLE messaging_attachment_send_intents ADD COLUMN references_bytes bytea NOT NULL`)
	require.NoError(t, err)
	require.NoError(t, RequireAttachmentIntentSchema(ctx, pool))
	_, err = pool.Exec(ctx, `INSERT INTO messaging_attachment_send_intents(operation_id,message_id,chat_id,request_sha256,references_bytes) VALUES('00000000-0000-4000-8000-000000000001','00000000-0000-4000-8000-000000000002','00000000-0000-4000-8000-000000000003',decode(repeat('00',32),'hex'),decode('010203','hex'))`)
	require.NoError(t, err)
	down, err := os.ReadFile(filepath.Join(repoRoot(t), "src/backend/migrations/messaging_db/000026_attachment_send_intents.down.sql"))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(down))
	require.ErrorContains(t, err, "rollback requires drained, exported evidence")
	var saved []byte
	require.NoError(t, pool.QueryRow(ctx, `SELECT references_bytes FROM messaging_attachment_send_intents`).Scan(&saved))
	require.Equal(t, []byte{1, 2, 3}, saved)
	require.NoError(t, RequireAttachmentIntentSchema(ctx, pool))
}
