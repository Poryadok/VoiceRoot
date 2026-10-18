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
	applyBaseMessagingMigrations(t, ctx, pool)
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
	applyBaseMessagingMigrations(t, ctx, pool)
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

// Missing Role wiring cannot turn a Space into a standalone chat, including
// the alternative forward/copy path before its optional commentary insert.
func TestMessagingSendAndForward_missingRoleFailsClosed(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, "src/backend/migrations/chat_db/000001_init.up.sql")
	applyBaseMessagingMigrations(t, ctx, pool)
	applyModerationSchemasForMessagingTest(t, ctx, pool)
	targetChatID, sourceChatID, spaceID := uuid.New(), uuid.New(), uuid.New()
	actor, peer := uuid.New(), uuid.New()
	seedGroupChat(t, ctx, pool, targetChatID, actor, peer)
	seedGroupChat(t, ctx, pool, sourceChatID, actor, peer)
	_, err := pool.Exec(ctx, `UPDATE chats SET space_id=$2 WHERE id=$1`, targetChatID, spaceID)
	require.NoError(t, err)
	sourceID := uuid.New()
	_, err = pool.Exec(ctx, `INSERT INTO messages (id,chat_id,chat_type,sender_profile_id,content) VALUES ($1,$2,'group',$3,'source')`, sourceID, sourceChatID, actor)
	require.NoError(t, err)
	client, cleanup := startMessagingServerWired(t, pool, messagingWire{})
	defer cleanup()
	actorCtx := withProfileCtx(ctx, uuid.New(), actor)
	_, err = client.SendMessage(actorCtx, &messagingv1.SendMessageRequest{Chat: chatGroupRef(targetChatID), Content: "must not persist"})
	require.Equal(t, codes.Unavailable, status.Code(err))
	commentary := "must not persist either"
	for _, copyAsNew := range []bool{false, true} {
		_, err = client.ForwardMessage(actorCtx, &messagingv1.ForwardMessageRequest{SourceMessageId: sourceID.String(), TargetChat: chatGroupRef(targetChatID), WithoutAttribution: &copyAsNew, Commentary: &commentary})
		require.Equal(t, codes.Unavailable, status.Code(err))
	}
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM messages WHERE chat_id=$1`, targetChatID).Scan(&count))
	require.Zero(t, count)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM message_event_outbox WHERE chat_id=$1`, targetChatID).Scan(&count))
	require.Zero(t, count)
}
