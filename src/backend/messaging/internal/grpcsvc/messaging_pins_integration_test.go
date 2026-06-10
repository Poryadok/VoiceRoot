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
	"voice/backend/messaging/internal/store"
)

func TestMessagingPinMessage_persistsPin(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000002_client_message_id.up.sql"))
	chatID := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	acctA := uuid.New()
	seedDMChat(t, ctx, pool, chatID, profA, profB)

	client, _ := startMessagingServer(t, pool)

	sent, err := client.SendMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.SendMessageRequest{
		Chat: chatDMRef(chatID), Content: "pin me", AttachmentsJson: "[]", MentionsJson: "[]",
	})
	require.NoError(t, err)
	msgID := sent.GetMessage().GetId()

	_, err = client.PinMessage(withProfileCtx(ctx, acctA, profB), &messagingv1.PinMessageRequest{
		Chat: chatDMRef(chatID), MessageId: msgID,
	})
	require.NoError(t, err)

	var count int
	err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM pins WHERE chat_id = $1 AND message_id = $2`, chatID, msgID).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 1, count)

	listed, err := client.GetPinnedMessages(withProfileCtx(ctx, acctA, profB), &messagingv1.GetPinnedMessagesRequest{
		Chat: chatDMRef(chatID),
	})
	require.NoError(t, err)
	require.Len(t, listed.GetMessageList().GetMessages(), 1)
	require.True(t, listed.GetMessageList().GetMessages()[0].GetIsPinned())
}

func TestMessagingPinMessage_idempotent(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000002_client_message_id.up.sql"))
	chatID := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	acctA := uuid.New()
	seedDMChat(t, ctx, pool, chatID, profA, profB)

	client, _ := startMessagingServer(t, pool)
	sent, err := client.SendMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.SendMessageRequest{
		Chat: chatDMRef(chatID), Content: "pin twice", AttachmentsJson: "[]", MentionsJson: "[]",
	})
	require.NoError(t, err)
	msgID := sent.GetMessage().GetId()

	for range 2 {
		_, err = client.PinMessage(withProfileCtx(ctx, acctA, profB), &messagingv1.PinMessageRequest{
			Chat: chatDMRef(chatID), MessageId: msgID,
		})
		require.NoError(t, err)
	}
	var count int
	err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM pins WHERE chat_id = $1`, chatID).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

func TestMessagingUnpinMessage_removesPin(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000002_client_message_id.up.sql"))
	chatID := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	acctA := uuid.New()
	seedDMChat(t, ctx, pool, chatID, profA, profB)

	client, _ := startMessagingServer(t, pool)
	sent, err := client.SendMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.SendMessageRequest{
		Chat: chatDMRef(chatID), Content: "unpin me", AttachmentsJson: "[]", MentionsJson: "[]",
	})
	require.NoError(t, err)
	msgID := sent.GetMessage().GetId()

	_, err = client.PinMessage(withProfileCtx(ctx, acctA, profB), &messagingv1.PinMessageRequest{
		Chat: chatDMRef(chatID), MessageId: msgID,
	})
	require.NoError(t, err)

	_, err = client.UnpinMessage(withProfileCtx(ctx, acctA, profB), &messagingv1.UnpinMessageRequest{
		Chat: chatDMRef(chatID), MessageId: msgID,
	})
	require.NoError(t, err)

	var count int
	err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM pins WHERE chat_id = $1`, chatID).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 0, count)
}

func TestMessagingPinMessage_nonMemberDenied(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000002_client_message_id.up.sql"))
	chatID := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	outsider := uuid.New()
	acctOut := uuid.New()
	seedDMChat(t, ctx, pool, chatID, profA, profB)

	client, _ := startMessagingServer(t, pool)
	sent, err := client.SendMessage(withProfileCtx(ctx, uuid.New(), profA), &messagingv1.SendMessageRequest{
		Chat: chatDMRef(chatID), Content: "members only", AttachmentsJson: "[]", MentionsJson: "[]",
	})
	require.NoError(t, err)

	_, err = client.PinMessage(withProfileCtx(ctx, acctOut, outsider), &messagingv1.PinMessageRequest{
		Chat: chatDMRef(chatID), MessageId: sent.GetMessage().GetId(),
	})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestMessagingPinMessage_deletedMessageNotFound(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000002_client_message_id.up.sql"))
	chatID := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	acctA := uuid.New()
	seedDMChat(t, ctx, pool, chatID, profA, profB)

	client, _ := startMessagingServer(t, pool)
	sent, err := client.SendMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.SendMessageRequest{
		Chat: chatDMRef(chatID), Content: "delete me", AttachmentsJson: "[]", MentionsJson: "[]",
	})
	require.NoError(t, err)
	msgID := sent.GetMessage().GetId()

	_, err = client.DeleteMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.DeleteMessageRequest{MessageId: msgID})
	require.NoError(t, err)

	_, err = client.PinMessage(withProfileCtx(ctx, acctA, profB), &messagingv1.PinMessageRequest{
		Chat: chatDMRef(chatID), MessageId: msgID,
	})
	require.Equal(t, codes.NotFound, status.Code(err))
}

func TestMessagingPinMessage_limit50(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000002_client_message_id.up.sql"))
	chatID := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	acctA := uuid.New()
	seedDMChat(t, ctx, pool, chatID, profA, profB)

	client, _ := startMessagingServer(t, pool)

	for range store.MaxPinsPerChat {
		sent, err := client.SendMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.SendMessageRequest{
			Chat: chatDMRef(chatID), Content: "msg", AttachmentsJson: "[]", MentionsJson: "[]",
		})
		require.NoError(t, err)
		_, err = client.PinMessage(withProfileCtx(ctx, acctA, profB), &messagingv1.PinMessageRequest{
			Chat: chatDMRef(chatID), MessageId: sent.GetMessage().GetId(),
		})
		require.NoError(t, err)
	}

	extra, err := client.SendMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.SendMessageRequest{
		Chat: chatDMRef(chatID), Content: "one too many", AttachmentsJson: "[]", MentionsJson: "[]",
	})
	require.NoError(t, err)
	_, err = client.PinMessage(withProfileCtx(ctx, acctA, profB), &messagingv1.PinMessageRequest{
		Chat: chatDMRef(chatID), MessageId: extra.GetMessage().GetId(),
	})
	require.Equal(t, codes.ResourceExhausted, status.Code(err))
}

func TestMessagingPinMessage_spacePermissionDenied(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000002_client_message_id.up.sql"))
	spaceID := uuid.New()
	chatID := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	acctA := uuid.New()
	_, err := pool.Exec(ctx, `
INSERT INTO chats (id, type, creator_profile_id, slow_mode_seconds, space_id)
VALUES ($1, 'group', $2, 0, $3)
`, chatID, profA, spaceID)
	require.NoError(t, err)
	for _, p := range []uuid.UUID{profA, profB} {
		_, err = pool.Exec(ctx, `INSERT INTO chat_members (chat_id, profile_id, role) VALUES ($1, $2, 'member')`, chatID, p)
		require.NoError(t, err)
	}

	client, _ := startMessagingServerWired(t, pool, messagingWire{RolePermissions: testRolePerms{allowed: false}})

	msgID := uuid.New()
	_, err = pool.Exec(ctx, `
INSERT INTO messages (id, chat_id, chat_type, sender_profile_id, content, attachments, mentions)
VALUES ($1, $2, 'dm', $3, 'space pin', '[]'::jsonb, '[]'::jsonb)
`, msgID, chatID, profA)
	require.NoError(t, err)

	_, err = client.PinMessage(withProfileCtx(ctx, acctA, profB), &messagingv1.PinMessageRequest{
		Chat: chatDMRef(chatID), MessageId: msgID.String(),
	})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestMessagingGetMessages_includesIsPinned(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000002_client_message_id.up.sql"))
	chatID := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	acctA := uuid.New()
	seedDMChat(t, ctx, pool, chatID, profA, profB)

	client, _ := startMessagingServer(t, pool)
	sent, err := client.SendMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.SendMessageRequest{
		Chat: chatDMRef(chatID), Content: "pinned in list", AttachmentsJson: "[]", MentionsJson: "[]",
	})
	require.NoError(t, err)
	msgID := sent.GetMessage().GetId()

	_, err = client.PinMessage(withProfileCtx(ctx, acctA, profB), &messagingv1.PinMessageRequest{
		Chat: chatDMRef(chatID), MessageId: msgID,
	})
	require.NoError(t, err)

	history, err := client.GetMessages(withProfileCtx(ctx, acctA, profB), &messagingv1.GetMessagesRequest{
		Chat: chatDMRef(chatID),
	})
	require.NoError(t, err)
	require.True(t, history.GetMessageList().GetMessages()[0].GetIsPinned())
}
