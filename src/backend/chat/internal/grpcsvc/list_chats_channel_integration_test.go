package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	chatv1 "voice.app/voice/chat/v1"
)

// TestListChats_MembershipChannel_InMainInbox documents chat-service.md: channels with
// chat_members rows must surface in ListChats main inbox (not only space merge).
func TestListChats_MembershipChannel_InMainInbox(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatPostgresForTest(t, ctx)
	applyChatMigration(t, ctx, pool)

	owner := uuid.New()
	accOwner := uuid.New()
	profiles := mapProfileAccounts{owner: accOwner}

	client, cleanup := startChatGRPCTestServer(t, pool, profiles, nil, nil)
	t.Cleanup(cleanup)

	var channelID uuid.UUID
	err := pool.QueryRow(ctx, `
INSERT INTO chats (type, name, creator_profile_id, slow_mode_seconds, threads_enabled, allow_user_main_feed)
VALUES ('channel', 'News', $1, 0, true, false)
RETURNING id
`, owner).Scan(&channelID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
INSERT INTO chat_members (chat_id, profile_id, role, inbox_bucket)
VALUES ($1, $2, 'owner', 'main')
`, channelID, owner)
	require.NoError(t, err)

	list, err := client.ListChats(withAccountProfileCtx(ctx, accOwner, owner), &chatv1.ListChatsRequest{})
	require.NoError(t, err)
	require.Len(t, list.GetChatList().GetItems(), 1)
	item := list.GetChatList().GetItems()[0].GetChat()
	require.Equal(t, channelID.String(), item.GetId())
	require.Equal(t, chatv1.ChatType_CHAT_TYPE_CHANNEL, item.GetType())
	require.Equal(t, "News", item.GetName())
	require.True(t, item.GetThreadsEnabled())
	require.False(t, item.GetAllowUserMainFeed())
}
