package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/pkg/integrationtest"
)

func TestSQLChatMentionsMeta_LoadSpaceMembers(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	chatPool := integrationtest.StartPostgres(t, ctx, "chatmentions", "")
	spacePool := integrationtest.StartPostgres(t, ctx, "spacementions", "")
	_, err := chatPool.Exec(ctx, `
CREATE TABLE chats (
    id UUID PRIMARY KEY,
    type TEXT NOT NULL,
    space_id UUID NULL,
    creator_profile_id UUID NOT NULL,
    slow_mode_seconds INT NOT NULL DEFAULT 0
);
CREATE TABLE chat_members (
    chat_id UUID NOT NULL,
    profile_id UUID NOT NULL,
    PRIMARY KEY (chat_id, profile_id)
)`)
	require.NoError(t, err)
	_, err = spacePool.Exec(ctx, `
CREATE TABLE space_members (
    space_id UUID NOT NULL,
    profile_id UUID NOT NULL,
    PRIMARY KEY (space_id, profile_id)
)`)
	require.NoError(t, err)

	chatID := uuid.New()
	spaceID := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	_, err = chatPool.Exec(ctx, `
INSERT INTO chats (id, type, space_id, creator_profile_id) VALUES ($1, 'channel', $2, $3)
`, chatID, spaceID, profA)
	require.NoError(t, err)
	_, err = spacePool.Exec(ctx, `
INSERT INTO space_members (space_id, profile_id) VALUES ($1, $2), ($1, $3)
`, spaceID, profA, profB)
	require.NoError(t, err)

	meta := &SQLChatMentionsMeta{Pool: chatPool, SpacePool: spacePool}
	got, err := meta.LoadChatMeta(ctx, chatID)
	require.NoError(t, err)
	require.Equal(t, "channel", got.ChatType)
	require.NotNil(t, got.SpaceID)
	require.Len(t, got.Members, 2)
}
