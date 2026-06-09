package grpcsvc

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	chatv1 "voice.app/voice/chat/v1"
	messagingv1 "voice.app/voice/messaging/v1"
)

// TestMessagingForwardMessage_preservesAttribution documents PLAN Phase 4 / forward-messages.md:
// forwarded message keeps original content and "Forwarded from [name]" metadata.
func TestMessagingForwardMessage_preservesAttribution(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000002_client_message_id.up.sql"))

	sourceChat := uuid.New()
	targetChat := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	profC := uuid.New()
	acctA := uuid.New()
	seedDMChat(t, ctx, pool, sourceChat, profA, profB)
	seedDMChat(t, ctx, pool, targetChat, profA, profC)

	client, _ := startMessagingServer(t, pool)

	original, err := client.SendMessage(withProfileCtx(ctx, acctA, profB), &messagingv1.SendMessageRequest{
		Chat:            chatDMRef(sourceChat),
		Content:         "original text",
		AttachmentsJson: "[]",
		MentionsJson:    "[]",
	})
	require.NoError(t, err)

	fwd, err := client.ForwardMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.ForwardMessageRequest{
		SourceMessageId: original.GetMessage().GetId(),
		TargetChat:      chatDMRef(targetChat),
	})
	require.NoError(t, err)

	msg := fwd.GetMessage()
	require.Equal(t, messagingv1.MessageKind_MESSAGE_KIND_FORWARD, msg.GetMessageKind())
	require.Equal(t, "forward", msg.GetType())
	require.Equal(t, original.GetMessage().GetId(), msg.GetForwardFromId())
	require.NotEmpty(t, msg.GetForwardFromSender())
	require.Equal(t, "original text", msg.GetContent())
	require.Equal(t, targetChat.String(), msg.GetChat().GetId())
	require.Equal(t, profA.String(), msg.GetSenderProfileId())
}

// TestMessagingForwardMessage_chainPointsToOriginal documents forward-messages.md:
// re-forwarding does not accumulate attribution — only the original source is kept.
func TestMessagingForwardMessage_chainPointsToOriginal(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000002_client_message_id.up.sql"))

	chatA := uuid.New()
	chatB := uuid.New()
	chatC := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	profC := uuid.New()
	profD := uuid.New()
	acctA := uuid.New()
	seedDMChat(t, ctx, pool, chatA, profA, profB)
	seedDMChat(t, ctx, pool, chatB, profA, profC)
	seedDMChat(t, ctx, pool, chatC, profA, profD)

	client, _ := startMessagingServer(t, pool)

	original, err := client.SendMessage(withProfileCtx(ctx, acctA, profB), &messagingv1.SendMessageRequest{
		Chat: chatDMRef(chatA), Content: "root", AttachmentsJson: "[]", MentionsJson: "[]",
	})
	require.NoError(t, err)

	firstHop, err := client.ForwardMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.ForwardMessageRequest{
		SourceMessageId: original.GetMessage().GetId(),
		TargetChat:      chatDMRef(chatB),
	})
	require.NoError(t, err)

	secondHop, err := client.ForwardMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.ForwardMessageRequest{
		SourceMessageId: firstHop.GetMessage().GetId(),
		TargetChat:      chatDMRef(chatC),
	})
	require.NoError(t, err)

	msg := secondHop.GetMessage()
	require.Equal(t, original.GetMessage().GetId(), msg.GetForwardFromId())
	require.NotEqual(t, firstHop.GetMessage().GetId(), msg.GetForwardFromId())
	require.Equal(t, "root", msg.GetContent())
}

// TestMessagingForwardMessage_toGroupChat documents forward-messages.md: forward into a group the user belongs to.
func TestMessagingForwardMessage_toGroupChat(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000003_groups.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000002_client_message_id.up.sql"))

	sourceChat := uuid.New()
	targetGroup := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	profC := uuid.New()
	acctA := uuid.New()
	seedDMChat(t, ctx, pool, sourceChat, profA, profB)
	seedGroupChat(t, ctx, pool, targetGroup, profA, profC)

	client, _ := startMessagingServer(t, pool)

	original, err := client.SendMessage(withProfileCtx(ctx, acctA, profB), &messagingv1.SendMessageRequest{
		Chat: chatDMRef(sourceChat), Content: "group-bound", AttachmentsJson: "[]", MentionsJson: "[]",
	})
	require.NoError(t, err)

	fwd, err := client.ForwardMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.ForwardMessageRequest{
		SourceMessageId: original.GetMessage().GetId(),
		TargetChat:      chatGroupRef(targetGroup),
	})
	require.NoError(t, err)

	msg := fwd.GetMessage()
	require.Equal(t, messagingv1.MessageKind_MESSAGE_KIND_FORWARD, msg.GetMessageKind())
	require.Equal(t, original.GetMessage().GetId(), msg.GetForwardFromId())
	require.NotEmpty(t, msg.GetForwardFromSender())
	require.Equal(t, "group-bound", msg.GetContent())
	require.Equal(t, targetGroup.String(), msg.GetChat().GetId())
}

func seedGroupChat(t *testing.T, ctx context.Context, pool *pgxpool.Pool, chatID, owner, member uuid.UUID) {
	t.Helper()
	_, err := pool.Exec(ctx, `
INSERT INTO chats (id, type, creator_profile_id, slow_mode_seconds)
VALUES ($1, 'group', $2, 0)
`, chatID, owner)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
INSERT INTO chat_members (chat_id, profile_id, role) VALUES
  ($1, $2, 'owner'),
  ($1, $3, 'member')
`, chatID, owner, member)
	require.NoError(t, err)
}

func chatGroupRef(chatID uuid.UUID) *chatv1.ChatRef {
	group := chatv1.ChatType_CHAT_TYPE_GROUP
	return &chatv1.ChatRef{Id: chatID.String(), Type: &group}
}

// TestMessagingForwardMessage_nonMemberDenied ensures the forwarder must belong to the target chat.
func TestMessagingForwardMessage_nonMemberDenied(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000002_client_message_id.up.sql"))

	sourceChat := uuid.New()
	targetChat := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	profC := uuid.New()
	profD := uuid.New()
	acctD := uuid.New()
	seedDMChat(t, ctx, pool, sourceChat, profA, profB)
	seedDMChat(t, ctx, pool, targetChat, profC, profD)

	client, _ := startMessagingServer(t, pool)

	original, err := client.SendMessage(withProfileCtx(ctx, uuid.New(), profA), &messagingv1.SendMessageRequest{
		Chat: chatDMRef(sourceChat), Content: "secret", AttachmentsJson: "[]", MentionsJson: "[]",
	})
	require.NoError(t, err)

	_, err = client.ForwardMessage(withProfileCtx(ctx, acctD, profD), &messagingv1.ForwardMessageRequest{
		SourceMessageId: original.GetMessage().GetId(),
		TargetChat:      chatDMRef(targetChat),
	})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}
