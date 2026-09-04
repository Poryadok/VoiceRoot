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
	"voice/backend/messaging/internal/store"
)

// TestMessagingReadState_perMemberAcrossDMGroupAndChannel proves the A1
// invariant that one member reading a chat cannot advance another member's
// durable unread/read state.
func TestMessagingReadState_perMemberAcrossDMGroupAndChannel(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applyA1ReadProofMigrations(t, ctx, pool)

	reader := uuid.New()
	sender := uuid.New()
	otherMember := uuid.New()
	readerAccount := uuid.New()
	senderAccount := uuid.New()
	otherAccount := uuid.New()

	dmID := uuid.New()
	groupID := uuid.New()
	channelID := uuid.New()
	seedDMChat(t, ctx, pool, dmID, reader, sender)
	seedGroupChatWithMembersForReadState(t, ctx, pool, groupID, sender, reader, otherMember)
	seedA1ReadProofChannel(t, ctx, pool, channelID, sender, reader, otherMember)

	client, _ := startMessagingServer(t, pool)
	postedAsChat := true
	cases := []struct {
		name                     string
		chat                     *chatv1.ChatRef
		send                     *messagingv1.SendMessageRequest
		chatID                   uuid.UUID
		untouchedProfile         uuid.UUID
		untouchedAccount         uuid.UUID
		untouchedUnreadCountWant int64
	}{
		{
			name:                     "dm",
			chat:                     chatDMRef(dmID),
			chatID:                   dmID,
			untouchedProfile:         sender,
			untouchedAccount:         senderAccount,
			untouchedUnreadCountWant: 0,
			send: &messagingv1.SendMessageRequest{
				Chat: chatDMRef(dmID), Content: "dm unread", AttachmentsJson: "[]", MentionsJson: "[]",
			},
		},
		{
			name:                     "group",
			chat:                     chatGroupRef(groupID),
			chatID:                   groupID,
			untouchedProfile:         otherMember,
			untouchedAccount:         otherAccount,
			untouchedUnreadCountWant: 1,
			send: &messagingv1.SendMessageRequest{
				Chat: chatGroupRef(groupID), Content: "group unread", AttachmentsJson: "[]", MentionsJson: "[]",
			},
		},
		{
			name:                     "channel",
			chat:                     chatChannelRef(channelID),
			chatID:                   channelID,
			untouchedProfile:         otherMember,
			untouchedAccount:         otherAccount,
			untouchedUnreadCountWant: 1,
			send: &messagingv1.SendMessageRequest{
				Chat: chatChannelRef(channelID), Content: "channel unread", AttachmentsJson: "[]", MentionsJson: "[]", PostedAsChat: &postedAsChat,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sent, err := client.SendMessage(withProfileCtx(ctx, senderAccount, sender), tc.send)
			require.NoError(t, err)
			messageID := sent.GetMessage().GetId()

			before, err := client.GetChatListMetadata(withProfileCtx(ctx, readerAccount, reader), &messagingv1.GetChatListMetadataRequest{Chats: []*chatv1.ChatRef{tc.chat}})
			require.NoError(t, err)
			require.Equal(t, int64(1), before.GetByChatId()[tc.chatID.String()].GetUnreadCount())

			_, err = client.MarkRead(withProfileCtx(ctx, readerAccount, reader), &messagingv1.MarkReadRequest{Chat: tc.chat, LastReadMessageId: messageID})
			require.NoError(t, err)

			readerState, err := client.GetReadState(withProfileCtx(ctx, readerAccount, reader), &messagingv1.GetReadStateRequest{Chat: tc.chat})
			require.NoError(t, err)
			require.Equal(t, messageID, readerState.GetReadState().GetLastReadMessageId())

			otherState, err := client.GetReadState(withProfileCtx(ctx, tc.untouchedAccount, tc.untouchedProfile), &messagingv1.GetReadStateRequest{Chat: tc.chat})
			require.NoError(t, err)
			require.Nil(t, otherState.GetReadState(), "another member's cursor must remain untouched")

			readerMeta, err := client.GetChatListMetadata(withProfileCtx(ctx, readerAccount, reader), &messagingv1.GetChatListMetadataRequest{Chats: []*chatv1.ChatRef{tc.chat}})
			require.NoError(t, err)
			require.Equal(t, int64(0), readerMeta.GetByChatId()[tc.chatID.String()].GetUnreadCount())

			otherMeta, err := client.GetChatListMetadata(withProfileCtx(ctx, tc.untouchedAccount, tc.untouchedProfile), &messagingv1.GetChatListMetadataRequest{Chats: []*chatv1.ChatRef{tc.chat}})
			require.NoError(t, err)
			require.Equal(t, tc.untouchedUnreadCountWant, otherMeta.GetByChatId()[tc.chatID.String()].GetUnreadCount(), "another member's unread count must remain untouched")
		})
	}
}

// TestMessagingReadRPCs_nonMemberDeniedAcrossChatTypes keeps all durable read
// endpoints fail-closed for DM, group, and channel membership boundaries.
func TestMessagingReadRPCs_nonMemberDeniedAcrossChatTypes(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applyA1ReadProofMigrations(t, ctx, pool)

	member := uuid.New()
	peer := uuid.New()
	thirdMember := uuid.New()
	stranger := uuid.New()
	memberAccount := uuid.New()
	strangerAccount := uuid.New()

	dmID := uuid.New()
	groupID := uuid.New()
	channelID := uuid.New()
	seedDMChat(t, ctx, pool, dmID, member, peer)
	seedGroupChatWithMembersForReadState(t, ctx, pool, groupID, member, peer, thirdMember)
	seedA1ReadProofChannel(t, ctx, pool, channelID, member, peer, thirdMember)

	client, _ := startMessagingServer(t, pool)
	postedAsChat := true
	cases := []struct {
		name string
		chat *chatv1.ChatRef
		id   uuid.UUID
		send *messagingv1.SendMessageRequest
	}{
		{name: "dm", chat: chatDMRef(dmID), id: dmID, send: &messagingv1.SendMessageRequest{Chat: chatDMRef(dmID), Content: "private", AttachmentsJson: "[]", MentionsJson: "[]"}},
		{name: "group", chat: chatGroupRef(groupID), id: groupID, send: &messagingv1.SendMessageRequest{Chat: chatGroupRef(groupID), Content: "private", AttachmentsJson: "[]", MentionsJson: "[]"}},
		{name: "channel", chat: chatChannelRef(channelID), id: channelID, send: &messagingv1.SendMessageRequest{Chat: chatChannelRef(channelID), Content: "private", AttachmentsJson: "[]", MentionsJson: "[]", PostedAsChat: &postedAsChat}},
	}

	strangerCtx := withProfileCtx(ctx, strangerAccount, stranger)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sent, err := client.SendMessage(withProfileCtx(ctx, memberAccount, member), tc.send)
			require.NoError(t, err)

			_, err = client.MarkRead(strangerCtx, &messagingv1.MarkReadRequest{Chat: tc.chat, LastReadMessageId: sent.GetMessage().GetId()})
			require.Equal(t, codes.PermissionDenied, status.Code(err))

			_, err = client.GetReadState(strangerCtx, &messagingv1.GetReadStateRequest{Chat: tc.chat})
			require.Equal(t, codes.PermissionDenied, status.Code(err))

			_, err = client.GetBulkReadState(strangerCtx, &messagingv1.GetBulkReadStateRequest{Chats: []*chatv1.ChatRef{tc.chat}})
			require.Equal(t, codes.PermissionDenied, status.Code(err))

			_, err = client.GetChatListMetadata(strangerCtx, &messagingv1.GetChatListMetadataRequest{Chats: []*chatv1.ChatRef{tc.chat}})
			require.Equal(t, codes.PermissionDenied, status.Code(err))
		})
	}
}

// TestMessagingGetChatListMetadata_durablePreviewContentAndDeliveryForBothMembers
// covers the durable REST metadata that backs list previews and delivery ticks.
func TestMessagingGetChatListMetadata_durablePreviewContentAndDeliveryForBothMembers(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applyA1ReadProofMigrations(t, ctx, pool)

	chatID := uuid.New()
	sender := uuid.New()
	receiver := uuid.New()
	senderAccount := uuid.New()
	receiverAccount := uuid.New()
	fileID := uuid.New().String()
	seedDMChat(t, ctx, pool, chatID, sender, receiver)

	client, _ := startMessagingServerWired(t, pool, messagingWire{Files: fileMetadataMap{
		fileID: {
			Id: fileID, UploaderProfileId: sender.String(), OriginalName: "proof.png", MimeType: "image/png", SizeBytes: 1024,
			Status: "ready", FileType: "image", ScanResult: "clean", Chat: chatDMRef(chatID),
		},
	}})
	attachments := mustAttachmentJSON(t, []map[string]any{{"file_id": fileID, "type": "image"}})
	sent, err := client.SendMessage(withProfileCtx(ctx, senderAccount, sender), &messagingv1.SendMessageRequest{
		Chat: chatDMRef(chatID), Content: "durable photo preview", AttachmentsJson: attachments, MentionsJson: "[]",
	})
	require.NoError(t, err)

	senderMeta := getA1ProofMetadata(t, ctx, client, senderAccount, sender, chatID)
	require.Equal(t, "durable photo preview", senderMeta.GetLastMessagePreview())
	require.Equal(t, messagingv1.MessageContentType_MESSAGE_CONTENT_TYPE_PHOTO, senderMeta.GetLastMessageContentType())
	require.True(t, senderMeta.GetLastMessageIsOutgoing())
	require.Equal(t, messagingv1.LastMessageDeliveryState_LAST_MESSAGE_DELIVERY_STATE_SENT, senderMeta.GetLastMessageDeliveryState())
	require.Equal(t, int64(0), senderMeta.GetUnreadCount())

	receiverMeta := getA1ProofMetadata(t, ctx, client, receiverAccount, receiver, chatID)
	require.Equal(t, "durable photo preview", receiverMeta.GetLastMessagePreview())
	require.Equal(t, messagingv1.MessageContentType_MESSAGE_CONTENT_TYPE_PHOTO, receiverMeta.GetLastMessageContentType())
	require.False(t, receiverMeta.GetLastMessageIsOutgoing())
	require.Equal(t, messagingv1.LastMessageDeliveryState_LAST_MESSAGE_DELIVERY_STATE_NONE, receiverMeta.GetLastMessageDeliveryState())
	require.Equal(t, int64(1), receiverMeta.GetUnreadCount())

	messages := &store.MessagesStore{Pool: pool}
	require.NoError(t, messages.UpsertDeliveredCursor(ctx, chatID, receiver, uuid.MustParse(sent.GetMessage().GetId())))
	senderMeta = getA1ProofMetadata(t, ctx, client, senderAccount, sender, chatID)
	require.Equal(t, messagingv1.LastMessageDeliveryState_LAST_MESSAGE_DELIVERY_STATE_DELIVERED, senderMeta.GetLastMessageDeliveryState())

	_, err = client.MarkRead(withProfileCtx(ctx, receiverAccount, receiver), &messagingv1.MarkReadRequest{Chat: chatDMRef(chatID), LastReadMessageId: sent.GetMessage().GetId()})
	require.NoError(t, err)
	senderMeta = getA1ProofMetadata(t, ctx, client, senderAccount, sender, chatID)
	require.Equal(t, messagingv1.LastMessageDeliveryState_LAST_MESSAGE_DELIVERY_STATE_READ, senderMeta.GetLastMessageDeliveryState())
	receiverMeta = getA1ProofMetadata(t, ctx, client, receiverAccount, receiver, chatID)
	require.Equal(t, int64(0), receiverMeta.GetUnreadCount())
}

// TestMessagingGetChatListMetadata_groupAndChannelSuppressDeliveryTicks keeps
// durable delivery ticks DM-only: group and channel list rows must not expose
// another member's delivered/read cursor as an outgoing-message tick.
func TestMessagingGetChatListMetadata_groupAndChannelSuppressDeliveryTicks(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applyA1ReadProofMigrations(t, ctx, pool)

	sender := uuid.New()
	receiver := uuid.New()
	extra := uuid.New()
	senderAccount := uuid.New()
	receiverAccount := uuid.New()
	groupID := uuid.New()
	channelID := uuid.New()
	seedGroupChatWithMembersForReadState(t, ctx, pool, groupID, sender, receiver, extra)
	seedA1ReadProofChannel(t, ctx, pool, channelID, sender, receiver, extra)

	client, _ := startMessagingServer(t, pool)
	messages := &store.MessagesStore{Pool: pool}
	postedAsChat := true
	cases := []struct {
		name string
		chat *chatv1.ChatRef
		send *messagingv1.SendMessageRequest
		id   uuid.UUID
	}{
		{
			name: "group",
			chat: chatGroupRef(groupID),
			id:   groupID,
			send: &messagingv1.SendMessageRequest{
				Chat: chatGroupRef(groupID), Content: "group no ticks", AttachmentsJson: "[]", MentionsJson: "[]",
			},
		},
		{
			name: "channel",
			chat: chatChannelRef(channelID),
			id:   channelID,
			send: &messagingv1.SendMessageRequest{
				Chat: chatChannelRef(channelID), Content: "channel no ticks", AttachmentsJson: "[]", MentionsJson: "[]", PostedAsChat: &postedAsChat,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sent, err := client.SendMessage(withProfileCtx(ctx, senderAccount, sender), tc.send)
			require.NoError(t, err)
			messageID := sent.GetMessage().GetId()
			require.NoError(t, messages.UpsertDeliveredCursor(ctx, tc.id, receiver, uuid.MustParse(messageID)))
			_, err = client.MarkRead(withProfileCtx(ctx, receiverAccount, receiver), &messagingv1.MarkReadRequest{Chat: tc.chat, LastReadMessageId: messageID})
			require.NoError(t, err)

			metadata, err := client.GetChatListMetadata(withProfileCtx(ctx, senderAccount, sender), &messagingv1.GetChatListMetadataRequest{Chats: []*chatv1.ChatRef{tc.chat}})
			require.NoError(t, err)
			item := metadata.GetByChatId()[tc.id.String()]
			require.NotNil(t, item)
			require.Equal(t, messagingv1.LastMessageDeliveryState_LAST_MESSAGE_DELIVERY_STATE_NONE, item.GetLastMessageDeliveryState())
		})
	}
}

func applyA1ReadProofMigrations(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000003_groups.up.sql"))
	applyBaseMessagingMigrations(t, ctx, pool)
}

func seedA1ReadProofChannel(t *testing.T, ctx context.Context, pool *pgxpool.Pool, chatID, owner, member, extra uuid.UUID) {
	t.Helper()
	_, err := pool.Exec(ctx, `
INSERT INTO chats (id, type, creator_profile_id, slow_mode_seconds, threads_enabled, allow_user_main_feed)
VALUES ($1, 'channel', $2, 0, true, false)
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

func getA1ProofMetadata(t *testing.T, ctx context.Context, client messagingv1.MessagingServiceClient, accountID, profileID, chatID uuid.UUID) *messagingv1.ChatListMetadata {
	t.Helper()
	resp, err := client.GetChatListMetadata(withProfileCtx(ctx, accountID, profileID), &messagingv1.GetChatListMetadataRequest{Chats: []*chatv1.ChatRef{chatDMRef(chatID)}})
	require.NoError(t, err)
	item := resp.GetByChatId()[chatID.String()]
	require.NotNil(t, item)
	return item
}
