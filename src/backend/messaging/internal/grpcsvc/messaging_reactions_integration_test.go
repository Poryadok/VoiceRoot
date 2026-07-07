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

// TestMessagingAddReaction_persistsEmoji documents text-chat.md / text-chat.md:
// users can add Unicode emoji reactions to messages in chats they belong to.
func TestMessagingAddReaction_persistsEmoji(t *testing.T) {
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
		Chat:            chatDMRef(chatID),
		Content:         "react to me",
		AttachmentsJson: "[]",
		MentionsJson:    "[]",
	})
	require.NoError(t, err)
	msgID := sent.GetMessage().GetId()

	_, err = client.AddReaction(withProfileCtx(ctx, acctA, profB), &messagingv1.AddReactionRequest{
		MessageId: msgID,
		Emoji:     "👍",
	})
	require.NoError(t, err)

	var count int
	err = pool.QueryRow(ctx, `
SELECT COUNT(*) FROM reactions
WHERE message_id = $1 AND profile_id = $2 AND emoji = $3
`, msgID, profB, "👍").Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

// TestMessagingAddReaction_idempotentForSameUser ensures duplicate adds do not inflate counts.
func TestMessagingAddReaction_idempotentForSameUser(t *testing.T) {
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
		Chat: chatDMRef(chatID), Content: "hi", AttachmentsJson: "[]", MentionsJson: "[]",
	})
	require.NoError(t, err)
	msgID := sent.GetMessage().GetId()

	for range 2 {
		_, err = client.AddReaction(withProfileCtx(ctx, acctA, profB), &messagingv1.AddReactionRequest{
			MessageId: msgID,
			Emoji:     "🔥",
		})
		require.NoError(t, err)
	}

	var count int
	err = pool.QueryRow(ctx, `
SELECT COUNT(*) FROM reactions WHERE message_id = $1 AND emoji = $2
`, msgID, "🔥").Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

// TestMessagingAddReaction_aggregatesCounts documents emoji counters on messages.
func TestMessagingAddReaction_aggregatesCounts(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000002_client_message_id.up.sql"))
	chatID := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	profC := uuid.New()
	acctA := uuid.New()
	acctC := uuid.New()
	seedDMChat(t, ctx, pool, chatID, profA, profB)
	_, err := pool.Exec(ctx, `
INSERT INTO chat_members (chat_id, profile_id, role) VALUES ($1, $2, 'member')
`, chatID, profC)
	require.NoError(t, err)

	client, _ := startMessagingServer(t, pool)

	sent, err := client.SendMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.SendMessageRequest{
		Chat: chatDMRef(chatID), Content: "popular", AttachmentsJson: "[]", MentionsJson: "[]",
	})
	require.NoError(t, err)
	msgID := sent.GetMessage().GetId()

	_, err = client.AddReaction(withProfileCtx(ctx, acctA, profB), &messagingv1.AddReactionRequest{
		MessageId: msgID, Emoji: "❤️",
	})
	require.NoError(t, err)
	_, err = client.AddReaction(withProfileCtx(ctx, acctC, profC), &messagingv1.AddReactionRequest{
		MessageId: msgID, Emoji: "❤️",
	})
	require.NoError(t, err)

	var count int
	err = pool.QueryRow(ctx, `
SELECT COUNT(*) FROM reactions WHERE message_id = $1 AND emoji = $2
`, msgID, "❤️").Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 2, count)
}

// TestMessagingRemoveReaction_clearsReaction documents toggling off a reaction.
func TestMessagingRemoveReaction_clearsReaction(t *testing.T) {
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
		Chat: chatDMRef(chatID), Content: "toggle", AttachmentsJson: "[]", MentionsJson: "[]",
	})
	require.NoError(t, err)
	msgID := sent.GetMessage().GetId()

	_, err = client.AddReaction(withProfileCtx(ctx, acctA, profB), &messagingv1.AddReactionRequest{
		MessageId: msgID, Emoji: "👍",
	})
	require.NoError(t, err)

	_, err = client.RemoveReaction(withProfileCtx(ctx, acctA, profB), &messagingv1.RemoveReactionRequest{
		MessageId: msgID, Emoji: "👍",
	})
	require.NoError(t, err)

	var count int
	err = pool.QueryRow(ctx, `
SELECT COUNT(*) FROM reactions WHERE message_id = $1 AND emoji = $2
`, msgID, "👍").Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 0, count)
}

// TestMessagingAddReaction_nonMemberDenied ensures only chat members can react.
func TestMessagingAddReaction_nonMemberDenied(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000002_client_message_id.up.sql"))
	chatID := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	profOutsider := uuid.New()
	acctOutsider := uuid.New()
	seedDMChat(t, ctx, pool, chatID, profA, profB)

	client, _ := startMessagingServer(t, pool)

	sent, err := client.SendMessage(withProfileCtx(ctx, uuid.New(), profA), &messagingv1.SendMessageRequest{
		Chat: chatDMRef(chatID), Content: "members only", AttachmentsJson: "[]", MentionsJson: "[]",
	})
	require.NoError(t, err)

	_, err = client.AddReaction(withProfileCtx(ctx, acctOutsider, profOutsider), &messagingv1.AddReactionRequest{
		MessageId: sent.GetMessage().GetId(),
		Emoji:     "👍",
	})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

