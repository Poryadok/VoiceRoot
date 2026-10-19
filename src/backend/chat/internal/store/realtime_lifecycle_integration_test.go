//go:build realtimep3integration

package store

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"
	chatv1 "voice.app/voice/chat/v1"
)

func TestRealtimeNativeLifecycleReadsOnlyActualChatAndFailsClosedWithoutSchema(t *testing.T) {
	ctx := context.Background()
	dsn := os.Getenv("REALTIME_P3_TEST_DATABASE_URL")
	require.NotEmpty(t, dsn, "owned isolated PostgreSQL required")
	admin, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(admin.Close)
	database := "chatp3_" + uuid.New().String()[:8]
	_, err = admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{database}.Sanitize())
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := admin.Exec(ctx, "DROP DATABASE "+pgx.Identifier{database}.Sanitize())
		require.NoError(t, err)
	})
	cfg, err := pgxpool.ParseConfig(dsn)
	require.NoError(t, err)
	cfg.ConnConfig.Database = database
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	paths, err := filepath.Glob("../../../migrations/chat_db/*.up.sql")
	require.NoError(t, err)
	require.NotEmpty(t, paths)
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(raw))
		require.NoError(t, err, path)
	}
	store := &SpaceLifecycleStore{Pool: pool}
	dm, channel, space, profile, operation := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	_, err = pool.Exec(ctx, `INSERT INTO chats(id,type,space_id,name,creator_profile_id) VALUES($1,'dm',NULL,NULL,$4),($2,'channel',$3,'channel',$4)`, dm, channel, space, profile)
	require.NoError(t, err)
	live, err := store.ResolveChatLifecycle(ctx, dm)
	require.NoError(t, err)
	require.Empty(t, live.SpaceId)
	require.Equal(t, chatv1.ChatLifecycleState_CHAT_LIFECYCLE_STATE_LIVE, live.State)
	live, err = store.ResolveChatLifecycle(ctx, channel)
	require.NoError(t, err)
	require.Equal(t, space.String(), live.SpaceId)
	require.Zero(t, live.Generation)
	_, err = pool.Exec(ctx, `INSERT INTO chat_space_lifecycle_fences(space_id,state) VALUES($1,'LIVE')`, space)
	require.NoError(t, err)
	live, err = store.ResolveChatLifecycle(ctx, channel)
	require.NoError(t, err, "explicit native LIVE0 row has nullable operation")
	require.Zero(t, live.Generation)
	require.Empty(t, live.DeletionOperationId)
	for state, want := range map[string]chatv1.ChatLifecycleState{"LIVE": chatv1.ChatLifecycleState_CHAT_LIFECYCLE_STATE_LIVE, "FROZEN": chatv1.ChatLifecycleState_CHAT_LIFECYCLE_STATE_FROZEN, "PURGE_DECIDED": chatv1.ChatLifecycleState_CHAT_LIFECYCLE_STATE_PURGE_DECIDED, "PURGED": chatv1.ChatLifecycleState_CHAT_LIFECYCLE_STATE_PURGED} {
		_, err = pool.Exec(ctx, `UPDATE chat_space_lifecycle_fences SET generation=2,state=$2,deletion_operation_id=$3,schedule_generation=1,manifest_id=$4,manifest_sha256=decode(repeat('ab',32),'hex'),manifest_item_count=1,source_manifest_id=$4,source_manifest_sha256=decode(repeat('ab',32),'hex'),source_manifest_item_count=1 WHERE space_id=$1`, space, state, operation, uuid.New())
		require.NoError(t, err)
		response, err := store.ResolveChatLifecycle(ctx, channel)
		require.NoError(t, err)
		require.Equal(t, want, response.State)
		require.Equal(t, uint64(2), response.Generation)
		require.Equal(t, operation.String(), response.DeletionOperationId)
		require.Equal(t, uint32(1), response.SchemaVersion)
	}
	_, err = store.ResolveChatLifecycle(ctx, uuid.New())
	require.ErrorIs(t, err, pgx.ErrNoRows)
	_, err = pool.Exec(ctx, `ALTER TABLE chat_space_lifecycle_fences RENAME TO inaccessible_lifecycle_fences`)
	require.NoError(t, err)
	_, err = store.ResolveChatLifecycle(ctx, dm)
	require.Error(t, err, "ordinary DM cannot become LIVE from missing authority schema")
}
