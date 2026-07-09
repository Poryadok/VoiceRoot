package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	messagingv1 "voice.app/voice/messaging/v1"
)

// TestPlatformModeration_ShadowBannedSenderMessageHiddenFromPeer documents moderation (docs/features/reports.md) shadow ban:
// sender still sends successfully; peer must not see the message in GetMessages.
func TestPlatformModeration_ShadowBannedSenderMessageHiddenFromPeer(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, "src/backend/migrations/chat_db/000001_init.up.sql")
	applySQLFile(t, ctx, pool, "src/backend/migrations/messaging_db/000001_init.up.sql")
	applySQLFile(t, ctx, pool, "src/backend/migrations/messaging_db/000002_client_message_id.up.sql")

	profA, acctA := uuid.New(), uuid.New()
	profB, acctB := uuid.New(), uuid.New()
	profiles := profileAcctMap{profA: acctA, profB: acctB}

	chatID := uuid.New()
	seedDMChat(t, ctx, pool, chatID, profA, profB)

	client, cleanup := startMessagingServerWired(t, pool, messagingWire{
		UserProfiles: profiles,
		PlatformMod: fakeShadowBanMod{shadowBanned: map[uuid.UUID]bool{
			acctB: true,
		}},
	})
	t.Cleanup(cleanup)

	const shadowContent = "shadow-banned payload phase14"
	sendResp, err := client.SendMessage(withProfileCtx(ctx, acctB, profB), &messagingv1.SendMessageRequest{
		Chat:    chatDMRef(chatID),
		Content: shadowContent,
	})
	require.NoError(t, err)
	require.NotEmpty(t, sendResp.GetMessage().GetId(), "shadow-banned sender may still get optimistic send ack")

	peerMsgs, err := client.GetMessages(withProfileCtx(ctx, acctA, profA), &messagingv1.GetMessagesRequest{
		Chat: chatDMRef(chatID),
	})
	require.NoError(t, err)
	for _, m := range peerMsgs.GetMessageList().GetMessages() {
		require.NotEqual(t, shadowContent, m.GetContent(), "peer must not receive shadow-banned sender content")
	}
	require.Empty(t, peerMsgs.GetMessageList().GetMessages(), "peer history must omit shadow-banned messages entirely")
}
