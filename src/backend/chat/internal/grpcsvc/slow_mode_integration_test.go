package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	chatv1 "voice.app/voice/chat/v1"
)

// TestSpaceSlowMode_SetTenSecondsOnSpaceChannel documents spaces.md: slow mode on space text chat (group|channel).
func TestSpaceSlowMode_SetTenSecondsOnSpaceChannel(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	profiles := profileMap(uuid.New())
	owner := uuidKey(profiles)
	pool := startChatPostgresForTest(t, context.Background())
	applyChatMigration(t, context.Background(), pool)
	client, cleanup := startChatGRPCTestServer(t, pool, profiles, nil, nil)
	t.Cleanup(cleanup)

	spaceID := uuid.New().String()
	name := "announcements"
	created, err := client.CreateChat(ctxFor(t, profiles, owner), &chatv1.CreateChatRequest{
		Type:    chatv1.ChatType_CHAT_TYPE_GROUP,
		Name:    &name,
		SpaceId: &spaceID,
	})
	require.NoError(t, err)
	chatID := created.GetChat().GetId()
	chatUUID, err := uuid.Parse(chatID)
	require.NoError(t, err)
	_, err = pool.Exec(context.Background(), `
INSERT INTO chat_members (chat_id, profile_id, role) VALUES ($1, $2, 'owner')
ON CONFLICT DO NOTHING
`, chatUUID, owner)
	require.NoError(t, err)

	slow := int32(10)
	updated, err := client.UpdateChat(ctxFor(t, profiles, owner), &chatv1.UpdateChatRequest{
		ChatId:          chatID,
		SlowModeSeconds: &slow,
	})
	require.NoError(t, err)
	require.Equal(t, int32(10), updated.GetChat().GetSlowModeSeconds())

	got, err := client.GetChat(ctxFor(t, profiles, owner), &chatv1.GetChatRequest{ChatId: chatID})
	require.NoError(t, err)
	require.Equal(t, int32(10), got.GetChat().GetSlowModeSeconds())
}
