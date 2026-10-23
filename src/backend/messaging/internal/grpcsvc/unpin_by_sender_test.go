package grpcsvc

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	messagingv1 "voice.app/voice/messaging/v1"
)

// Ordinary identity metadata cannot authorize the protected Bot cleanup effect.
func TestUnpinMessagesBySenderInChats_rejectsLegacyCleanupWithoutEffects(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000001_init.up.sql"))
	applyBaseMessagingMigrations(t, ctx, pool)

	chatA := uuid.New()
	chatB := uuid.New()
	botProfile := uuid.New()
	humanProfile := uuid.New()
	acctHuman := uuid.New()
	seedDMChat(t, ctx, pool, chatA, humanProfile, botProfile)
	seedDMChat(t, ctx, pool, chatB, humanProfile, botProfile)

	client, _ := startMessagingServer(t, pool)

	botMsgA, err := client.SendMessage(withProfileCtx(ctx, acctHuman, botProfile), &messagingv1.SendMessageRequest{
		Chat: chatDMRef(chatA), Content: "bot pin A", AttachmentsJson: "[]", MentionsJson: "[]",
	})
	require.NoError(t, err)
	botMsgB, err := client.SendMessage(withProfileCtx(ctx, acctHuman, botProfile), &messagingv1.SendMessageRequest{
		Chat: chatDMRef(chatB), Content: "bot pin B", AttachmentsJson: "[]", MentionsJson: "[]",
	})
	require.NoError(t, err)
	humanMsg, err := client.SendMessage(withProfileCtx(ctx, acctHuman, humanProfile), &messagingv1.SendMessageRequest{
		Chat: chatDMRef(chatA), Content: "human pin", AttachmentsJson: "[]", MentionsJson: "[]",
	})
	require.NoError(t, err)

	for _, msgID := range []string{botMsgA.GetMessage().GetId(), botMsgB.GetMessage().GetId(), humanMsg.GetMessage().GetId()} {
		_, err = client.PinMessage(withProfileCtx(ctx, acctHuman, humanProfile), &messagingv1.PinMessageRequest{
			Chat: chatDMRef(chatA), MessageId: msgID,
		})
		if msgID == botMsgB.GetMessage().GetId() {
			_, err = client.PinMessage(withProfileCtx(ctx, acctHuman, humanProfile), &messagingv1.PinMessageRequest{
				Chat: chatDMRef(chatB), MessageId: msgID,
			})
		}
		require.NoError(t, err)
	}

	_, err = client.UnpinMessagesBySenderInChats(withProfileCtx(ctx, acctHuman, humanProfile), &messagingv1.UnpinMessagesBySenderInChatsRequest{
		SenderProfileId: botProfile.String(),
		ChatIds:         []string{chatA.String(), chatB.String()},
	})
	require.Equal(t, codes.PermissionDenied, status.Code(err))

	var botPinCount int
	err = pool.QueryRow(ctx, `
SELECT COUNT(*) FROM pins p
JOIN messages m ON m.id = p.message_id
WHERE m.sender_profile_id = $1`, botProfile).Scan(&botPinCount)
	require.NoError(t, err)
	require.Equal(t, 2, botPinCount, "legacy cleanup must not remove bot pins")

	var humanPinCount int
	err = pool.QueryRow(ctx, `
SELECT COUNT(*) FROM pins p
JOIN messages m ON m.id = p.message_id
WHERE m.sender_profile_id = $1`, humanProfile).Scan(&humanPinCount)
	require.NoError(t, err)
	require.Equal(t, 1, humanPinCount, "non-bot pins must remain")

	var botMsgCount int
	err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM messages WHERE sender_profile_id = $1`, botProfile).Scan(&botMsgCount)
	require.NoError(t, err)
	require.Equal(t, 2, botMsgCount, "unpin cleanup must not delete messages")
}
