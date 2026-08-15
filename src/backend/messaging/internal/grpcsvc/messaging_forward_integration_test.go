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
	commonv1 "voice.app/voice/common/v1"
	messagingv1 "voice.app/voice/messaging/v1"

	"voice/backend/role/permissions"
)

// TestMessagingForwardMessage_preservesAttribution documents text-chat.md / forward-messages.md:
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

// TestMessagingForwardMessage_commentaryInsertsSeparateMessage documents forward-messages.md optional commentary.
func TestMessagingForwardMessage_commentaryInsertsSeparateMessage(t *testing.T) {
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
		Chat: chatDMRef(sourceChat), Content: "payload", AttachmentsJson: "[]", MentionsJson: "[]",
	})
	require.NoError(t, err)

	commentary := "see this"
	fwd, err := client.ForwardMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.ForwardMessageRequest{
		SourceMessageId: original.GetMessage().GetId(),
		TargetChat:      chatDMRef(targetChat),
		Commentary:      &commentary,
	})
	require.NoError(t, err)
	require.Equal(t, "payload", fwd.GetMessage().GetContent())

	history, err := client.GetMessages(withProfileCtx(ctx, acctA, profA), &messagingv1.GetMessagesRequest{
		Chat: chatDMRef(targetChat),
		Page: &commonv1.CursorPageRequest{PageSize: 10},
	})
	require.NoError(t, err)
	msgs := history.GetMessageList().GetMessages()
	require.Len(t, msgs, 2)
	require.Equal(t, commentary, msgs[1].GetContent())
	require.Equal(t, messagingv1.MessageKind_MESSAGE_KIND_FORWARD, msgs[0].GetMessageKind())
}

// TestMessagingForwardMessage_toChannelSetsPostedAsChat documents FW-02 / forward-messages.md:
// channel without allow_user_main_feed stores the forward as posted_as_chat.
func TestMessagingForwardMessage_toChannelSetsPostedAsChat(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applyThreadMessagingMigrations(t, ctx, pool)

	sourceChat := uuid.New()
	targetChannel := uuid.New()
	spaceID := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	acctA := uuid.New()
	seedDMChat(t, ctx, pool, sourceChat, profA, profB)
	seedChannelChat(t, ctx, pool, targetChannel, profA)
	applyModerationSchemasForMessagingTest(t, ctx, pool)
	_, err := pool.Exec(ctx, `UPDATE chats SET space_id = $2 WHERE id = $1`, targetChannel, spaceID)
	require.NoError(t, err)

	client, _ := startMessagingServerWired(t, pool, messagingWire{
		RolePermissions: selectiveRolePerms{allow: map[string]bool{
			permissions.TextChatSendMessages: true,
		}},
	})

	original, err := client.SendMessage(withProfileCtx(ctx, acctA, profB), &messagingv1.SendMessageRequest{
		Chat: chatDMRef(sourceChat), Content: "channel-bound", AttachmentsJson: "[]", MentionsJson: "[]",
	})
	require.NoError(t, err)

	commentary := "heads up"
	fwd, err := client.ForwardMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.ForwardMessageRequest{
		SourceMessageId: original.GetMessage().GetId(),
		TargetChat:      chatChannelRef(targetChannel),
		Commentary:      &commentary,
	})
	require.NoError(t, err)
	msg := fwd.GetMessage()
	require.Equal(t, messagingv1.MessageKind_MESSAGE_KIND_FORWARD, msg.GetMessageKind())
	require.True(t, msg.GetPostedAsChat(), "channel forward must set posted_as_chat when main-feed is restricted")
	require.Equal(t, "channel-bound", msg.GetContent())

	history, err := client.GetMessages(withProfileCtx(ctx, acctA, profA), &messagingv1.GetMessagesRequest{
		Chat: chatChannelRef(targetChannel),
		Page: &commonv1.CursorPageRequest{PageSize: 10},
	})
	require.NoError(t, err)
	msgs := history.GetMessageList().GetMessages()
	require.Len(t, msgs, 2)
	require.Equal(t, commentary, msgs[1].GetContent())
	require.True(t, msgs[0].GetPostedAsChat())
}

// TestMessagingForwardMessage_channelSendPermissionDenied documents Forward send-perm hole fix (P1.5).
func TestMessagingForwardMessage_channelSendPermissionDenied(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applyThreadMessagingMigrations(t, ctx, pool)

	sourceChat := uuid.New()
	targetChannel := uuid.New()
	spaceID := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	acctA := uuid.New()
	seedDMChat(t, ctx, pool, sourceChat, profA, profB)
	seedChannelChat(t, ctx, pool, targetChannel, profA)
	applyModerationSchemasForMessagingTest(t, ctx, pool)
	_, err := pool.Exec(ctx, `UPDATE chats SET space_id = $2 WHERE id = $1`, targetChannel, spaceID)
	require.NoError(t, err)

	client, _ := startMessagingServerWired(t, pool, messagingWire{
		RolePermissions: selectiveRolePerms{deny: map[string]bool{
			permissions.TextChatSendMessages: true,
		}},
	})

	original, err := client.SendMessage(withProfileCtx(ctx, acctA, profB), &messagingv1.SendMessageRequest{
		Chat: chatDMRef(sourceChat), Content: "blocked-fwd", AttachmentsJson: "[]", MentionsJson: "[]",
	})
	require.NoError(t, err)

	_, err = client.ForwardMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.ForwardMessageRequest{
		SourceMessageId: original.GetMessage().GetId(),
		TargetChat:      chatChannelRef(targetChannel),
	})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

// TestMessagingForwardMessage_e2eToPlainDenied documents FW-06: E2E ciphertext must not land in plain DM.
func TestMessagingForwardMessage_e2eToPlainDenied(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000006_e2e_enabled.up.sql"))
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
	seedE2EEnabledDM(t, ctx, pool, sourceChat)

	client, _ := startMessagingServer(t, pool)
	isE2E := true
	original, err := client.SendMessage(withProfileCtx(ctx, acctA, profB), &messagingv1.SendMessageRequest{
		Chat: chatDMRef(sourceChat), Content: "opaque-cipher", IsE2E: &isE2E, AttachmentsJson: "[]", MentionsJson: "[]",
	})
	require.NoError(t, err)
	require.True(t, original.GetMessage().GetIsE2E())

	_, err = client.ForwardMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.ForwardMessageRequest{
		SourceMessageId: original.GetMessage().GetId(),
		TargetChat:      chatDMRef(targetChat),
	})
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}

// TestMessagingForwardMessage_e2eToE2EDMPreservesFlag documents FW-06: E2E→E2E DM keeps is_e2e.
func TestMessagingForwardMessage_e2eToE2EDMPreservesFlag(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000006_e2e_enabled.up.sql"))
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
	seedE2EEnabledDM(t, ctx, pool, sourceChat)
	seedE2EEnabledDM(t, ctx, pool, targetChat)

	client, _ := startMessagingServer(t, pool)
	isE2E := true
	cipher := "opaque-e2e-forward-payload"
	original, err := client.SendMessage(withProfileCtx(ctx, acctA, profB), &messagingv1.SendMessageRequest{
		Chat: chatDMRef(sourceChat), Content: cipher, IsE2E: &isE2E, AttachmentsJson: "[]", MentionsJson: "[]",
	})
	require.NoError(t, err)

	fwd, err := client.ForwardMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.ForwardMessageRequest{
		SourceMessageId: original.GetMessage().GetId(),
		TargetChat:      chatDMRef(targetChat),
	})
	require.NoError(t, err)
	require.True(t, fwd.GetMessage().GetIsE2E())
	require.Equal(t, cipher, fwd.GetMessage().GetContent())
}
