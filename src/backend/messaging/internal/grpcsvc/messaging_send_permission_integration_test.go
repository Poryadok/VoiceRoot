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

	"voice/backend/role/permissions"
)

// TestMessagingSendMessage_chatOverrideDenySendMessages documents RL-02 / P1.8:
// TEXT_CHAT_SEND_MESSAGES deny (chat_override path via HasChatPermission) → PermissionDenied.
func TestMessagingSendMessage_chatOverrideDenySendMessages(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000002_client_message_id.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000011_last_delivered_message_id.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000012_messages_content_type.up.sql"))

	chatID := uuid.New()
	spaceID := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	acctA := uuid.New()
	seedGroupChat(t, ctx, pool, chatID, profA, profB)
	applyModerationSchemasForMessagingTest(t, ctx, pool)
	_, err := pool.Exec(ctx, `UPDATE chats SET space_id = $2 WHERE id = $1`, chatID, spaceID)
	require.NoError(t, err)

	client, _ := startMessagingServerWired(t, pool, messagingWire{
		RolePermissions: selectiveRolePerms{deny: map[string]bool{
			permissions.TextChatSendMessages: true,
		}},
	})
	_, err = client.SendMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.SendMessageRequest{
		Chat:            chatGroupRef(chatID),
		Content:         "should be blocked",
		AttachmentsJson: "[]",
		MentionsJson:    "[]",
	})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

// TestMessagingSendMessage_chatSendAllowedWhenPermissionGranted is the positive counterpart for RL-02.
func TestMessagingSendMessage_chatSendAllowedWhenPermissionGranted(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000002_client_message_id.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000011_last_delivered_message_id.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000012_messages_content_type.up.sql"))

	chatID := uuid.New()
	spaceID := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	acctA := uuid.New()
	seedGroupChat(t, ctx, pool, chatID, profA, profB)
	applyModerationSchemasForMessagingTest(t, ctx, pool)
	_, err := pool.Exec(ctx, `UPDATE chats SET space_id = $2 WHERE id = $1`, chatID, spaceID)
	require.NoError(t, err)

	client, _ := startMessagingServerWired(t, pool, messagingWire{
		RolePermissions: selectiveRolePerms{allow: map[string]bool{
			permissions.TextChatSendMessages: true,
		}},
	})
	sent, err := client.SendMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.SendMessageRequest{
		Chat:            chatGroupRef(chatID),
		Content:         "allowed send",
		AttachmentsJson: "[]",
		MentionsJson:    "[]",
	})
	require.NoError(t, err)
	require.Equal(t, "allowed send", sent.GetMessage().GetContent())
}
