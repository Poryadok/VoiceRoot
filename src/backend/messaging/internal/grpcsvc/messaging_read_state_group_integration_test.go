package grpcsvc

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	messagingv1 "voice.app/voice/messaging/v1"
)

// TestMessagingMarkRead_groupChat documents read receipts for group chats (text-chat.md view counts).
func TestMessagingMarkRead_groupChat(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000003_groups.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000002_client_message_id.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000011_last_delivered_message_id.up.sql"))

	groupID := uuid.New()
	owner := uuid.New()
	member := uuid.New()
	extra := uuid.New()
	acctOwner := uuid.New()
	seedGroupChatWithMembersForReadState(t, ctx, pool, groupID, owner, member, extra)

	client, _ := startMessagingServer(t, pool)

	sent, err := client.SendMessage(withProfileCtx(ctx, acctOwner, owner), &messagingv1.SendMessageRequest{
		Chat:            chatGroupRef(groupID),
		Content:         "group hello",
		AttachmentsJson: "[]",
		MentionsJson:    "[]",
	})
	require.NoError(t, err)

	_, err = client.MarkRead(withProfileCtx(ctx, uuid.New(), member), &messagingv1.MarkReadRequest{
		Chat:              chatGroupRef(groupID),
		LastReadMessageId: sent.GetMessage().GetId(),
	})
	require.NoError(t, err)

	rs, err := client.GetReadState(withProfileCtx(ctx, uuid.New(), member), &messagingv1.GetReadStateRequest{
		Chat: chatGroupRef(groupID),
	})
	require.NoError(t, err)
	require.Equal(t, sent.GetMessage().GetId(), rs.GetReadState().GetLastReadMessageId())
}

func seedGroupChatWithMembersForReadState(t *testing.T, ctx context.Context, pool *pgxpool.Pool, chatID, owner, member, extra uuid.UUID) {
	t.Helper()
	_, err := pool.Exec(ctx, `
INSERT INTO chats (id, type, creator_profile_id, slow_mode_seconds)
VALUES ($1, 'group', $2, 0)
`, chatID, owner)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
INSERT INTO chat_members (chat_id, profile_id, role) VALUES
  ($1, $2, 'owner'),
  ($1, $3, 'member'),
  ($1, $4, 'member')
`, chatID, owner, member, extra)
	require.NoError(t, err)
}
