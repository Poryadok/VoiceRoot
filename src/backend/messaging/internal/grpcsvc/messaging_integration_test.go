package grpcsvc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"voice/backend/pkg/integrationtest"

	"voice/backend/chat/testchat"
	"voice/backend/messaging/internal/authctx"
	"voice/backend/messaging/internal/mentions"
	"voice/backend/messaging/internal/messageevents"
	"voice/backend/messaging/internal/s2s"
	"voice/backend/messaging/internal/store"

	chatv1 "voice.app/voice/chat/v1"
	commonv1 "voice.app/voice/common/v1"
	filev1 "voice.app/voice/file/v1"
	messagingv1 "voice.app/voice/messaging/v1"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", ".."))
}

func startPostgresForTest(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()
	return integrationtest.StartPostgres(t, ctx, "db", "")
}

func applySQLFile(t *testing.T, ctx context.Context, pool *pgxpool.Pool, relPath string) {
	t.Helper()
	p := filepath.Join(repoRoot(t), relPath)
	b, err := os.ReadFile(p)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(b))
	require.NoError(t, err)
	if strings.HasSuffix(relPath, filepath.Join("messaging_db", "000002_client_message_id.up.sql")) {
		applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000003_attachment_only_messages.up.sql"))
		applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000004_delete_for_me.up.sql"))
		applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000005_reactions.up.sql"))
		applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000006_pins.up.sql"))
		applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000007_thread_index.up.sql"))
		applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000008_shared_media_indexes.up.sql"))
	}
	if strings.HasSuffix(relPath, filepath.Join("chat_db", "000001_init.up.sql")) {
		applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000002_dm_requests.up.sql"))
		applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000003_groups.up.sql"))
		applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000005_thread_settings.up.sql"))
	}
}

func withProfileCtx(ctx context.Context, accountID, profileID uuid.UUID) context.Context {
	ctx = metadata.AppendToOutgoingContext(ctx, authctx.HeaderUserID, accountID.String())
	return metadata.AppendToOutgoingContext(ctx, authctx.HeaderProfileID, profileID.String())
}

func incomingProfileCtx(ctx context.Context, accountID, profileID uuid.UUID) context.Context {
	return metadata.NewIncomingContext(ctx, metadata.Pairs(
		authctx.HeaderUserID, accountID.String(),
		authctx.HeaderProfileID, profileID.String(),
	))
}

func chatDMRef(chatID uuid.UUID) *chatv1.ChatRef {
	dm := chatv1.ChatType_CHAT_TYPE_DM
	return &chatv1.ChatRef{Id: chatID.String(), Type: &dm}
}

func seedDMChat(t *testing.T, ctx context.Context, pool *pgxpool.Pool, chatID, profA, profB uuid.UUID) {
	t.Helper()
	_, err := pool.Exec(ctx, `
INSERT INTO chats (id, type, creator_profile_id, slow_mode_seconds)
VALUES ($1, 'dm', $2, 0)
`, chatID, profA)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
INSERT INTO chat_members (chat_id, profile_id, role) VALUES
  ($1, $2, 'member'),
  ($1, $3, 'member')
`, chatID, profA, profB)
	require.NoError(t, err)
}

func startMessagingServerWired(t *testing.T, pool *pgxpool.Pool, w messagingWire) (messagingv1.MessagingServiceClient, func()) {
	t.Helper()
	guard := w.ChatGuard
	if guard == nil {
		guard = &store.SQLChatGuard{Pool: pool}
	}
	const bufSize = 1 << 20
	lis := bufconn.Listen(bufSize)
	srv := grpc.NewServer()
	moderation := w.Moderation
	if moderation == nil {
		moderation = &store.SQLModerationGuard{Pool: pool}
	}
	messagingv1.RegisterMessagingServiceServer(srv, &MessagingGRPC{
		Messages:          &store.MessagesStore{Pool: pool},
		Reactions:         &store.ReactionsStore{Pool: pool},
		Pins:              &store.PinsStore{Pool: pool},
		SharedMedia:       &store.SharedMediaStore{Pool: pool},
		ChatGuard:         guard,
		Blocks:            w.Blocks,
		UserProfiles:      w.UserProfiles,
		MessageEvents:     w.MessageEvents,
		Files:             w.Files,
		Moderation:        moderation,
		ChatMentionsMeta:  &store.SQLChatMentionsMeta{Pool: pool},
		RolePermissions:   w.RolePermissions,
		UserPresence:      w.UserPresence,
		ChatThreadPolicy:  &store.SQLChatThreadPolicy{Pool: pool},
	})
	go func() {
		if err := srv.Serve(lis); err != nil {
			t.Logf("grpc serve: %v", err)
		}
	}()
	t.Cleanup(func() { srv.Stop() })

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	return messagingv1.NewMessagingServiceClient(conn), func() { _ = conn.Close() }
}

type messagingWire struct {
	ChatGuard        ChatGuard
	Moderation       *store.SQLModerationGuard
	UserProfiles     ProfileAccountLookup
	Blocks           AccountPairBlockChecker
	MessageEvents    messageevents.MessageEventsPublisher
	Files            FileMetadataLookup
	RolePermissions  mentions.RolePermissionChecker
	UserPresence     mentions.OnlinePresenceLookup
}

func startMessagingServer(t *testing.T, pool *pgxpool.Pool) (messagingv1.MessagingServiceClient, func()) {
	t.Helper()
	return startMessagingServerWired(t, pool, messagingWire{})
}

func startMessagingDirect(t *testing.T, pool *pgxpool.Pool) *MessagingGRPC {
	t.Helper()
	guard := &store.SQLChatGuard{Pool: pool}
	return &MessagingGRPC{
		Messages:         &store.MessagesStore{Pool: pool},
		Reactions:        &store.ReactionsStore{Pool: pool},
		Pins:             &store.PinsStore{Pool: pool},
		SharedMedia:      &store.SharedMediaStore{Pool: pool},
		ChatGuard:        guard,
		Moderation:       &store.SQLModerationGuard{Pool: pool},
		ChatMentionsMeta: &store.SQLChatMentionsMeta{Pool: pool},
		ChatThreadPolicy: &store.SQLChatThreadPolicy{Pool: pool},
	}
}

type profileAcctMap map[uuid.UUID]uuid.UUID

func (m profileAcctMap) AccountIDByProfileID(_ context.Context, profileID uuid.UUID) (uuid.UUID, error) {
	a, ok := m[profileID]
	if !ok {
		return uuid.Nil, status.Error(codes.NotFound, "profile not found")
	}
	return a, nil
}

type boolBlocks bool

func (b boolBlocks) AccountPairBlocked(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return bool(b), nil
}

type fileMetadataMap map[string]*filev1.FileMetadata

func (m fileMetadataMap) GetBulkMetadata(_ context.Context, req *filev1.GetBulkMetadataRequest, _ ...grpc.CallOption) (*filev1.GetBulkMetadataResponse, error) {
	out := map[string]*filev1.FileMetadata{}
	for _, id := range req.GetFileIds() {
		if meta := m[id]; meta != nil {
			out[id] = meta
		}
	}
	return &filev1.GetBulkMetadataResponse{BulkFileMetadata: &filev1.BulkFileMetadata{ByFileId: out}}, nil
}

func TestMessagingSendGetMarkRead(t *testing.T) {
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

	clientID := uuid.New().String()
	mk := messagingv1.MessageKind_MESSAGE_KIND_REGULAR
	send := func(profile, acct uuid.UUID, content string) *messagingv1.SendMessageResponse {
		t.Helper()
		outCtx := withProfileCtx(ctx, acct, profile)
		resp, err := client.SendMessage(outCtx, &messagingv1.SendMessageRequest{
			Chat:            chatDMRef(chatID),
			Content:         content,
			ClientMessageId: &clientID,
			AttachmentsJson: "[]",
			MentionsJson:    "[]",
			MessageKind:     &mk,
		})
		require.NoError(t, err)
		return resp
	}

	r1 := send(profA, acctA, "hello")
	require.NotEmpty(t, r1.GetMessage().GetId())
	r2 := send(profA, acctA, "retry")
	require.Equal(t, r1.GetMessage().GetId(), r2.GetMessage().GetId(), "idempotent SendMessage must return same message id")
	require.Equal(t, "hello", r2.GetMessage().GetContent(), "idempotent retry must return original message body")

	_, err := client.SendMessage(withProfileCtx(ctx, acctA, profB), &messagingv1.SendMessageRequest{
		Chat:            chatDMRef(chatID),
		Content:         "from-b",
		AttachmentsJson: "[]",
		MentionsJson:    "[]",
	})
	require.NoError(t, err)

	list, err := client.GetMessages(withProfileCtx(ctx, acctA, profA), &messagingv1.GetMessagesRequest{
		Chat: chatDMRef(chatID),
		Page: &commonv1.CursorPageRequest{PageSize: 10},
	})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(list.GetMessageList().GetMessages()), 2)

	missing := uuid.New()
	missingStr := missing.String()
	gfb, err := client.GetMessages(withProfileCtx(ctx, acctA, profA), &messagingv1.GetMessagesRequest{
		Chat:          chatDMRef(chatID),
		LastMessageId: &missingStr,
		Page:          &commonv1.CursorPageRequest{PageSize: 2},
	})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(gfb.GetMessageList().GetMessages()), 2, "missing cursor must trigger fallback latest page")

	firstID := list.GetMessageList().GetMessages()[len(list.GetMessageList().GetMessages())-1].GetId()
	afterPage, err := client.GetMessages(withProfileCtx(ctx, acctA, profA), &messagingv1.GetMessagesRequest{
		Chat:           chatDMRef(chatID),
		AfterMessageId: &firstID,
		Page:           &commonv1.CursorPageRequest{PageSize: 10},
	})
	require.NoError(t, err)
	require.NotEmpty(t, afterPage.GetMessageList().GetMessages())
	for i := 1; i < len(afterPage.GetMessageList().GetMessages()); i++ {
		require.Less(t, afterPage.GetMessageList().GetMessages()[i-1].GetId(), afterPage.GetMessageList().GetMessages()[i].GetId(),
			"after_message_id page must be ascending by id")
	}

	newestID := list.GetMessageList().GetMessages()[0].GetId()
	_, err = client.MarkRead(withProfileCtx(ctx, acctA, profA), &messagingv1.MarkReadRequest{
		Chat:              chatDMRef(chatID),
		LastReadMessageId: newestID,
	})
	require.NoError(t, err)

	rs, err := client.GetReadState(withProfileCtx(ctx, acctA, profA), &messagingv1.GetReadStateRequest{Chat: chatDMRef(chatID)})
	require.NoError(t, err)
	require.Equal(t, newestID, rs.GetReadState().GetLastReadMessageId())

	_, err = client.MarkRead(withProfileCtx(ctx, acctA, profA), &messagingv1.MarkReadRequest{
		Chat:              chatDMRef(chatID),
		LastReadMessageId: uuid.New().String(),
	})
	require.Error(t, err)
	require.Equal(t, codes.NotFound, status.Code(err))
}

func TestMessagingSendAttachmentOnlyMessageValidatesReadyFile(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000002_client_message_id.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000003_attachment_only_messages.up.sql"))

	chatID := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	acctA := uuid.New()
	fileID := uuid.New().String()
	seedDMChat(t, ctx, pool, chatID, profA, profB)
	client, _ := startMessagingServerWired(t, pool, messagingWire{
		Files: fileMetadataMap{
			fileID: {
				Id:                fileID,
				UploaderProfileId: profA.String(),
				OriginalName:      "cat.png",
				MimeType:          "image/png",
				SizeBytes:         2048,
				Status:            "ready",
				FileType:          "image",
				ScanResult:        "clean",
				Chat:              chatDMRef(chatID),
			},
		},
	})
	attachments := mustAttachmentJSON(t, []map[string]any{{
		"file_id":     fileID,
		"type":        "image",
		"url":         "voice-file://" + fileID,
		"preview_url": "voice-file://" + fileID + "/preview",
	}})

	resp, err := client.SendMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.SendMessageRequest{
		Chat:            chatDMRef(chatID),
		Content:         "",
		AttachmentsJson: attachments,
		MentionsJson:    "[]",
	})
	require.NoError(t, err)
	require.Empty(t, resp.GetMessage().GetContent())
	require.JSONEq(t, attachments, resp.GetMessage().GetAttachmentsJson())

	missingAttachment := mustAttachmentJSON(t, []map[string]any{{"file_id": uuid.New().String(), "type": "image"}})
	_, err = client.SendMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.SendMessageRequest{
		Chat:            chatDMRef(chatID),
		AttachmentsJson: missingAttachment,
		MentionsJson:    "[]",
	})
	require.Error(t, err)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))

	infectedID := uuid.New().String()
	infectedClient, _ := startMessagingServerWired(t, pool, messagingWire{
		Files: fileMetadataMap{
			infectedID: {
				Id:         infectedID,
				Status:     "failed",
				FileType:   "image",
				ScanResult: "infected",
				Chat:       chatDMRef(chatID),
			},
		},
	})
	infectedAttachment := mustAttachmentJSON(t, []map[string]any{{"file_id": infectedID, "type": "image"}})
	_, err = infectedClient.SendMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.SendMessageRequest{
		Chat:            chatDMRef(chatID),
		AttachmentsJson: infectedAttachment,
		MentionsJson:    "[]",
	})
	require.Error(t, err)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}

func mustAttachmentJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return string(b)
}

func TestMessagingGetChatListMetadata_PreviewUnreadAndMarkRead(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000002_client_message_id.up.sql"))

	chatID := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	acctA := uuid.New()
	acctB := uuid.New()
	seedDMChat(t, ctx, pool, chatID, profA, profB)

	client, _ := startMessagingServer(t, pool)
	mk := messagingv1.MessageKind_MESSAGE_KIND_REGULAR
	send := func(profile, account uuid.UUID, content string) string {
		t.Helper()
		resp, err := client.SendMessage(withProfileCtx(ctx, account, profile), &messagingv1.SendMessageRequest{
			Chat:            chatDMRef(chatID),
			Content:         content,
			AttachmentsJson: "[]",
			MentionsJson:    "[]",
			MessageKind:     &mk,
		})
		require.NoError(t, err)
		return resp.GetMessage().GetId()
	}

	send(profA, acctA, "self first")
	peerSecond := send(profB, acctB, "peer second")
	peerThird := send(profB, acctB, "peer third")

	meta, err := client.GetChatListMetadata(withProfileCtx(ctx, acctA, profA), &messagingv1.GetChatListMetadataRequest{
		Chats: []*chatv1.ChatRef{chatDMRef(chatID)},
	})
	require.NoError(t, err)
	item := meta.GetByChatId()[chatID.String()]
	require.NotNil(t, item)
	require.Equal(t, "peer third", item.GetLastMessagePreview())
	require.Equal(t, int64(2), item.GetUnreadCount(), "only peer messages unread by viewer count")
	require.NotNil(t, item.GetLastMessageAt())

	_, err = client.MarkRead(withProfileCtx(ctx, acctA, profA), &messagingv1.MarkReadRequest{
		Chat:              chatDMRef(chatID),
		LastReadMessageId: peerSecond,
	})
	require.NoError(t, err)
	meta, err = client.GetChatListMetadata(withProfileCtx(ctx, acctA, profA), &messagingv1.GetChatListMetadataRequest{
		Chats: []*chatv1.ChatRef{chatDMRef(chatID)},
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), meta.GetByChatId()[chatID.String()].GetUnreadCount())

	_, err = client.MarkRead(withProfileCtx(ctx, acctA, profA), &messagingv1.MarkReadRequest{
		Chat:              chatDMRef(chatID),
		LastReadMessageId: peerThird,
	})
	require.NoError(t, err)
	meta, err = client.GetChatListMetadata(withProfileCtx(ctx, acctA, profA), &messagingv1.GetChatListMetadataRequest{
		Chats: []*chatv1.ChatRef{chatDMRef(chatID)},
	})
	require.NoError(t, err)
	require.Equal(t, int64(0), meta.GetByChatId()[chatID.String()].GetUnreadCount())
}

func TestMessagingMarkdownPreview_stripInChatListMetadata(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000002_client_message_id.up.sql"))

	chatID := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	acctA := uuid.New()
	acctB := uuid.New()
	seedDMChat(t, ctx, pool, chatID, profA, profB)

	client, _ := startMessagingServer(t, pool)
	mk := messagingv1.MessageKind_MESSAGE_KIND_REGULAR

	send := func(profile, account uuid.UUID, content string) {
		t.Helper()
		_, err := client.SendMessage(withProfileCtx(ctx, account, profile), &messagingv1.SendMessageRequest{
			Chat:            chatDMRef(chatID),
			Content:         content,
			AttachmentsJson: "[]",
			MentionsJson:    "[]",
			MessageKind:     &mk,
		})
		require.NoError(t, err)
	}

	send(profB, acctB, "**bold** preview")
	meta, err := client.GetChatListMetadata(withProfileCtx(ctx, acctA, profA), &messagingv1.GetChatListMetadataRequest{
		Chats: []*chatv1.ChatRef{chatDMRef(chatID)},
	})
	require.NoError(t, err)
	item := meta.GetByChatId()[chatID.String()]
	require.NotNil(t, item)
	require.Equal(t, "bold preview", item.GetLastMessagePreview())

	hist, err := client.GetMessages(withProfileCtx(ctx, acctA, profA), &messagingv1.GetMessagesRequest{
		Chat: chatDMRef(chatID),
	})
	require.NoError(t, err)
	require.NotEmpty(t, hist.GetMessageList().GetMessages())
	lastID := hist.GetMessageList().GetMessages()[0].GetId()
	_, err = client.EditMessage(withProfileCtx(ctx, acctB, profB), &messagingv1.EditMessageRequest{
		MessageId: lastID,
		Content:   "*italic* edit",
	})
	require.NoError(t, err)

	meta, err = client.GetChatListMetadata(withProfileCtx(ctx, acctA, profA), &messagingv1.GetChatListMetadataRequest{
		Chats: []*chatv1.ChatRef{chatDMRef(chatID)},
	})
	require.NoError(t, err)
	require.Equal(t, "italic edit", meta.GetByChatId()[chatID.String()].GetLastMessagePreview())

	got, err := client.GetMessages(withProfileCtx(ctx, acctA, profA), &messagingv1.GetMessagesRequest{
		Chat: chatDMRef(chatID),
	})
	require.NoError(t, err)
	require.Equal(t, "*italic* edit", got.GetMessageList().GetMessages()[0].GetContent(), "API stores markdown source unchanged")
}

func TestMessagingEditDeleteSenderOnlyPolicy(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000002_client_message_id.up.sql"))

	chatID := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	acctA := uuid.New()
	acctB := uuid.New()
	seedDMChat(t, ctx, pool, chatID, profA, profB)

	client, _ := startMessagingServer(t, pool)
	mk := messagingv1.MessageKind_MESSAGE_KIND_REGULAR

	sendA, err := client.SendMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.SendMessageRequest{
		Chat:            chatDMRef(chatID),
		Content:         "original",
		AttachmentsJson: "[]",
		MentionsJson:    "[]",
		MessageKind:     &mk,
	})
	require.NoError(t, err)
	msgID := sendA.GetMessage().GetId()

	// Sender may edit.
	edited, err := client.EditMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.EditMessageRequest{
		MessageId: msgID,
		Content:   "revised",
	})
	require.NoError(t, err)
	require.Equal(t, "revised", edited.GetMessage().GetContent())
	require.NotNil(t, edited.GetMessage().GetEditedAt())

	_, err = client.EditMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.EditMessageRequest{
		MessageId: msgID,
		Content:   "   ",
	})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))

	// Other DM member cannot edit sender's message.
	_, err = client.EditMessage(withProfileCtx(ctx, acctB, profB), &messagingv1.EditMessageRequest{
		MessageId: msgID,
		Content:   "hijack",
	})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))

	// Sender may delete; response is empty.
	_, err = client.DeleteMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.DeleteMessageRequest{MessageId: msgID})
	require.NoError(t, err)

	// Deleted message no longer in history.
	list, err := client.GetMessages(withProfileCtx(ctx, acctA, profA), &messagingv1.GetMessagesRequest{
		Chat: chatDMRef(chatID),
		Page: &commonv1.CursorPageRequest{PageSize: 20},
	})
	require.NoError(t, err)
	for _, m := range list.GetMessageList().GetMessages() {
		require.NotEqual(t, msgID, m.GetId(), "soft-deleted message must not appear in GetMessages")
	}

	// Cannot edit after delete.
	_, err = client.EditMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.EditMessageRequest{
		MessageId: msgID,
		Content:   "again",
	})
	require.Error(t, err)
	require.Equal(t, codes.NotFound, status.Code(err))

	// Other member cannot delete (even if we had not deleted — use a fresh message).
	sendB, err := client.SendMessage(withProfileCtx(ctx, acctB, profB), &messagingv1.SendMessageRequest{
		Chat:            chatDMRef(chatID),
		Content:         "from-b",
		AttachmentsJson: "[]",
		MentionsJson:    "[]",
	})
	require.NoError(t, err)
	bID := sendB.GetMessage().GetId()
	_, err = client.DeleteMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.DeleteMessageRequest{MessageId: bID})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))

	scopeMe := messagingv1.DeleteScope_DELETE_SCOPE_FOR_ME
	_, err = client.DeleteMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.DeleteMessageRequest{MessageId: bID, Scope: &scopeMe})
	require.NoError(t, err)
	hiddenForA, err := client.GetMessages(withProfileCtx(ctx, acctA, profA), &messagingv1.GetMessagesRequest{
		Chat: chatDMRef(chatID),
		Page: &commonv1.CursorPageRequest{PageSize: 20},
	})
	require.NoError(t, err)
	for _, m := range hiddenForA.GetMessageList().GetMessages() {
		require.NotEqual(t, bID, m.GetId(), "delete for me hides only for caller")
	}
	visibleForB, err := client.GetMessages(withProfileCtx(ctx, acctB, profB), &messagingv1.GetMessagesRequest{
		Chat: chatDMRef(chatID),
		Page: &commonv1.CursorPageRequest{PageSize: 20},
	})
	require.NoError(t, err)
	require.True(t, slices.ContainsFunc(visibleForB.GetMessageList().GetMessages(), func(m *messagingv1.Message) bool {
		return m.GetId() == bID
	}), "delete for me must not hide from peer")

	// Non-member cannot edit or delete.
	otherChat := uuid.New()
	profX := uuid.New()
	profY := uuid.New()
	acctX := uuid.New()
	seedDMChat(t, ctx, pool, otherChat, profX, profY)
	sendX, err := client.SendMessage(withProfileCtx(ctx, acctX, profX), &messagingv1.SendMessageRequest{
		Chat:            chatDMRef(otherChat),
		Content:         "secret",
		AttachmentsJson: "[]",
		MentionsJson:    "[]",
	})
	require.NoError(t, err)
	xID := sendX.GetMessage().GetId()
	_, err = client.EditMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.EditMessageRequest{
		MessageId: xID,
		Content:   "nope",
	})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	_, err = client.DeleteMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.DeleteMessageRequest{MessageId: xID})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestMessagingNonMemberDenied(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000002_client_message_id.up.sql"))

	chatID := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	profStranger := uuid.New()
	acct := uuid.New()
	seedDMChat(t, ctx, pool, chatID, profA, profB)

	client, _ := startMessagingServer(t, pool)
	_, err := client.SendMessage(withProfileCtx(ctx, acct, profStranger), &messagingv1.SendMessageRequest{
		Chat:            chatDMRef(chatID),
		Content:         "nope",
		AttachmentsJson: "[]",
		MentionsJson:    "[]",
	})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestMessagingChatMembershipViaGRPC(t *testing.T) {
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

	chatCli, chatCleanup := testchat.NewBufconnChatClient(t, pool)
	t.Cleanup(chatCleanup)

	client, _ := startMessagingServerWired(t, pool, messagingWire{
		ChatGuard: s2s.NewGRPCChatGuard(chatCli),
	})

	mk := messagingv1.MessageKind_MESSAGE_KIND_REGULAR
	_, err := client.SendMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.SendMessageRequest{
		Chat:            chatDMRef(chatID),
		Content:         "from-a",
		AttachmentsJson: "[]",
		MentionsJson:    "[]",
		MessageKind:     &mk,
	})
	require.NoError(t, err)
}

// TestChatMessagingIntegration_CreateDM_SendGetMessagesCursor wires Chat CreateDM (no friendship) with
// Messaging over a shared chat_db: S2S membership via ListMembers, then SendMessage + cursor GetMessages.
func TestChatMessagingIntegration_CreateDM_SendGetMessagesCursor(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000002_client_message_id.up.sql"))

	acctA := uuid.New()
	acctB := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	pmap := profileAcctMap{profA: acctA, profB: acctB}

	chatCli, chatCleanup := testchat.NewBufconnChatClientWith(t, pool, testchat.ChatDeps{Profiles: pmap})
	t.Cleanup(chatCleanup)

	ctxA := withProfileCtx(ctx, acctA, profA)
	dm, err := chatCli.CreateDM(ctxA, &chatv1.CreateDMRequest{OtherProfileId: profB.String()})
	require.NoError(t, err)
	chatID, err := uuid.Parse(dm.GetChat().GetId())
	require.NoError(t, err)

	client, _ := startMessagingServerWired(t, pool, messagingWire{
		ChatGuard:    s2s.NewGRPCChatGuard(chatCli),
		Blocks:       boolBlocks(false),
		UserProfiles: pmap,
	})

	mk := messagingv1.MessageKind_MESSAGE_KIND_REGULAR
	for i := 0; i < 4; i++ {
		_, err := client.SendMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.SendMessageRequest{
			Chat:            chatDMRef(chatID),
			Content:         t.Name() + string(rune('a'+i)),
			AttachmentsJson: "[]",
			MentionsJson:    "[]",
			MessageKind:     &mk,
		})
		require.NoError(t, err)
	}

	p1, err := client.GetMessages(withProfileCtx(ctx, acctA, profA), &messagingv1.GetMessagesRequest{
		Chat: chatDMRef(chatID),
		Page: &commonv1.CursorPageRequest{PageSize: 2},
	})
	require.NoError(t, err)
	ml1 := p1.GetMessageList()
	require.Len(t, ml1.GetMessages(), 2)
	require.True(t, ml1.GetHasMore(), "first page must signal more history")
	require.NotEmpty(t, ml1.GetNextCursor())

	p2, err := client.GetMessages(withProfileCtx(ctx, acctA, profA), &messagingv1.GetMessagesRequest{
		Chat: chatDMRef(chatID),
		Page: &commonv1.CursorPageRequest{Cursor: ml1.GetNextCursor(), PageSize: 2},
	})
	require.NoError(t, err)
	ml2 := p2.GetMessageList()
	require.Len(t, ml2.GetMessages(), 2)
	require.False(t, ml2.GetHasMore())

	secondNewest, err := uuid.Parse(ml1.GetMessages()[1].GetId())
	require.NoError(t, err)
	for _, m := range ml2.GetMessages() {
		mid, perr := uuid.Parse(m.GetId())
		require.NoError(t, perr)
		require.Less(t, bytes.Compare(mid[:], secondNewest[:]), 0,
			"second page must be strictly older than second item of first page (UUID byte order)")
	}
}

// TestChatMessagingIntegration_SendDeniedWhenSocialBlocks uses the same Chat-created DM; Messaging Blocks
// (Social IsBlocked wiring in production) must reject SendMessage with PermissionDenied.
func TestChatMessagingIntegration_SendDeniedWhenSocialBlocks(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000002_client_message_id.up.sql"))

	acctA := uuid.New()
	acctB := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	pmap := profileAcctMap{profA: acctA, profB: acctB}

	chatCli, chatCleanup := testchat.NewBufconnChatClientWith(t, pool, testchat.ChatDeps{Profiles: pmap})
	t.Cleanup(chatCleanup)

	ctxA := withProfileCtx(ctx, acctA, profA)
	dm, err := chatCli.CreateDM(ctxA, &chatv1.CreateDMRequest{OtherProfileId: profB.String()})
	require.NoError(t, err)
	chatID, err := uuid.Parse(dm.GetChat().GetId())
	require.NoError(t, err)

	client, _ := startMessagingServerWired(t, pool, messagingWire{
		ChatGuard:    s2s.NewGRPCChatGuard(chatCli),
		Blocks:       boolBlocks(true),
		UserProfiles: pmap,
	})
	mk := messagingv1.MessageKind_MESSAGE_KIND_REGULAR
	_, err = client.SendMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.SendMessageRequest{
		Chat:            chatDMRef(chatID),
		Content:         "should not send",
		AttachmentsJson: "[]",
		MentionsJson:    "[]",
		MessageKind:     &mk,
	})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestMessagingSendMessage_BlockedPairDenied(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000002_client_message_id.up.sql"))

	chatID := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	acctA := uuid.New()
	acctB := uuid.New()
	seedDMChat(t, ctx, pool, chatID, profA, profB)

	client, _ := startMessagingServerWired(t, pool, messagingWire{
		Blocks:       boolBlocks(true),
		UserProfiles: profileAcctMap{profB: acctB},
	})
	mk := messagingv1.MessageKind_MESSAGE_KIND_REGULAR
	_, err := client.SendMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.SendMessageRequest{
		Chat:            chatDMRef(chatID),
		Content:         "blocked",
		AttachmentsJson: "[]",
		MentionsJson:    "[]",
		MessageKind:     &mk,
	})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestMessagingSendMessage_WhenNotBlockedSucceeds(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000002_client_message_id.up.sql"))

	chatID := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	acctA := uuid.New()
	acctB := uuid.New()
	seedDMChat(t, ctx, pool, chatID, profA, profB)

	client, _ := startMessagingServerWired(t, pool, messagingWire{
		Blocks:       boolBlocks(false),
		UserProfiles: profileAcctMap{profB: acctB},
	})
	mk := messagingv1.MessageKind_MESSAGE_KIND_REGULAR
	_, err := client.SendMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.SendMessageRequest{
		Chat:            chatDMRef(chatID),
		Content:         "hi",
		AttachmentsJson: "[]",
		MentionsJson:    "[]",
		MessageKind:     &mk,
	})
	require.NoError(t, err)
}

func TestMessagingGetMessages_cursorPaginationInvalidAndExclusiveCursors(t *testing.T) {
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
	mk := messagingv1.MessageKind_MESSAGE_KIND_REGULAR
	send := func(body string) string {
		t.Helper()
		resp, err := client.SendMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.SendMessageRequest{
			Chat:            chatDMRef(chatID),
			Content:         body,
			AttachmentsJson: "[]",
			MentionsJson:    "[]",
			MessageKind:     &mk,
		})
		require.NoError(t, err)
		return resp.GetMessage().GetId()
	}
	for i := 0; i < 4; i++ {
		send(t.Name() + string(rune('a'+i)))
	}

	p1, err := client.GetMessages(withProfileCtx(ctx, acctA, profA), &messagingv1.GetMessagesRequest{
		Chat: chatDMRef(chatID),
		Page: &commonv1.CursorPageRequest{PageSize: 2},
	})
	require.NoError(t, err)
	ml1 := p1.GetMessageList()
	require.Len(t, ml1.GetMessages(), 2)
	require.True(t, ml1.GetHasMore(), "first page must signal more history")
	require.NotEmpty(t, ml1.GetNextCursor())
	require.Equal(t, ml1.GetNextCursor(), ml1.GetPage().GetNextCursor())

	p2, err := client.GetMessages(withProfileCtx(ctx, acctA, profA), &messagingv1.GetMessagesRequest{
		Chat: chatDMRef(chatID),
		Page: &commonv1.CursorPageRequest{Cursor: ml1.GetNextCursor(), PageSize: 2},
	})
	require.NoError(t, err)
	ml2 := p2.GetMessageList()
	require.Len(t, ml2.GetMessages(), 2)
	require.False(t, ml2.GetHasMore())

	secondNewest, err := uuid.Parse(ml1.GetMessages()[1].GetId())
	require.NoError(t, err)
	for _, m := range ml2.GetMessages() {
		mid, perr := uuid.Parse(m.GetId())
		require.NoError(t, perr)
		require.Less(t, bytes.Compare(mid[:], secondNewest[:]), 0,
			"second page must be strictly older than second item of first page (UUID byte order)")
	}

	_, err = client.GetMessages(withProfileCtx(ctx, acctA, profA), &messagingv1.GetMessagesRequest{
		Chat: chatDMRef(chatID),
		Page: &commonv1.CursorPageRequest{Cursor: "not-a-valid-cursor", PageSize: 2},
	})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))

	a := ml1.GetMessages()[0].GetId()
	b := ml1.GetMessages()[1].GetId()
	_, err = client.GetMessages(withProfileCtx(ctx, acctA, profA), &messagingv1.GetMessagesRequest{
		Chat:            chatDMRef(chatID),
		AfterMessageId:  &a,
		BeforeMessageId: &b,
		Page:            &commonv1.CursorPageRequest{PageSize: 2},
	})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestMessagingSendMessage_idempotentKeyScopedPerSender(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000002_client_message_id.up.sql"))

	chatID := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	acctA := uuid.New()
	acctB := uuid.New()
	seedDMChat(t, ctx, pool, chatID, profA, profB)

	client, _ := startMessagingServer(t, pool)
	mk := messagingv1.MessageKind_MESSAGE_KIND_REGULAR
	sharedKey := uuid.New().String()

	rA, err := client.SendMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.SendMessageRequest{
		Chat:            chatDMRef(chatID),
		Content:         "from-a",
		AttachmentsJson: "[]",
		MentionsJson:    "[]",
		MessageKind:     &mk,
		ClientMessageId: &sharedKey,
	})
	require.NoError(t, err)
	rB, err := client.SendMessage(withProfileCtx(ctx, acctB, profB), &messagingv1.SendMessageRequest{
		Chat:            chatDMRef(chatID),
		Content:         "from-b",
		AttachmentsJson: "[]",
		MentionsJson:    "[]",
		MessageKind:     &mk,
		ClientMessageId: &sharedKey,
	})
	require.NoError(t, err)
	require.NotEqual(t, rA.GetMessage().GetId(), rB.GetMessage().GetId(),
		"same client_message_id must not dedupe across different senders")
}

func TestMessagingNonMemberDenied_GetMessagesMarkReadGetReadState(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000002_client_message_id.up.sql"))

	chatID := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	profStranger := uuid.New()
	acctA := uuid.New()
	acctS := uuid.New()
	seedDMChat(t, ctx, pool, chatID, profA, profB)

	client, _ := startMessagingServer(t, pool)
	mk := messagingv1.MessageKind_MESSAGE_KIND_REGULAR
	sendA, err := client.SendMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.SendMessageRequest{
		Chat:            chatDMRef(chatID),
		Content:         "member-only",
		AttachmentsJson: "[]",
		MentionsJson:    "[]",
		MessageKind:     &mk,
	})
	require.NoError(t, err)
	msgID := sendA.GetMessage().GetId()

	strCtx := withProfileCtx(ctx, acctS, profStranger)
	_, err = client.GetMessages(strCtx, &messagingv1.GetMessagesRequest{
		Chat: chatDMRef(chatID),
		Page: &commonv1.CursorPageRequest{PageSize: 10},
	})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))

	_, err = client.MarkRead(strCtx, &messagingv1.MarkReadRequest{
		Chat:              chatDMRef(chatID),
		LastReadMessageId: msgID,
	})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))

	_, err = client.GetReadState(strCtx, &messagingv1.GetReadStateRequest{Chat: chatDMRef(chatID)})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestMessagingMarkRead_monotonicDoesNotRegress(t *testing.T) {
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
	mk := messagingv1.MessageKind_MESSAGE_KIND_REGULAR
	var ids []string
	for i := 0; i < 3; i++ {
		resp, err := client.SendMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.SendMessageRequest{
			Chat:            chatDMRef(chatID),
			Content:         t.Name() + string(rune('0'+i)),
			AttachmentsJson: "[]",
			MentionsJson:    "[]",
			MessageKind:     &mk,
		})
		require.NoError(t, err)
		ids = append(ids, resp.GetMessage().GetId())
	}
	oldest, mid, newest := ids[0], ids[1], ids[2]

	_, err := client.MarkRead(withProfileCtx(ctx, acctA, profA), &messagingv1.MarkReadRequest{
		Chat:              chatDMRef(chatID),
		LastReadMessageId: newest,
	})
	require.NoError(t, err)
	_, err = client.MarkRead(withProfileCtx(ctx, acctA, profA), &messagingv1.MarkReadRequest{
		Chat:              chatDMRef(chatID),
		LastReadMessageId: oldest,
	})
	require.NoError(t, err)
	_, err = client.MarkRead(withProfileCtx(ctx, acctA, profA), &messagingv1.MarkReadRequest{
		Chat:              chatDMRef(chatID),
		LastReadMessageId: mid,
	})
	require.NoError(t, err)

	rs, err := client.GetReadState(withProfileCtx(ctx, acctA, profA), &messagingv1.GetReadStateRequest{Chat: chatDMRef(chatID)})
	require.NoError(t, err)
	require.Equal(t, newest, rs.GetReadState().GetLastReadMessageId(),
		"read cursor must stay at newest UUID when older MarkRead is applied (monotonic)")
}

func TestMessagingGetBulkReadState(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000002_client_message_id.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000001_init.up.sql"))

	chatA := uuid.New()
	chatB := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	acctA := uuid.New()
	seedDMChat(t, ctx, pool, chatA, profA, profB)
	seedDMChat(t, ctx, pool, chatB, profA, uuid.New())

	client, cleanup := startMessagingServerWired(t, pool, messagingWire{})
	t.Cleanup(cleanup)

	sendA, err := client.SendMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.SendMessageRequest{
		Chat:            chatDMRef(chatA),
		Content:         "hello A",
		AttachmentsJson: "[]",
		MentionsJson:    "[]",
	})
	require.NoError(t, err)
	sendB, err := client.SendMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.SendMessageRequest{
		Chat:            chatDMRef(chatB),
		Content:         "hello B",
		AttachmentsJson: "[]",
		MentionsJson:    "[]",
	})
	require.NoError(t, err)

	_, err = client.MarkRead(withProfileCtx(ctx, acctA, profA), &messagingv1.MarkReadRequest{
		Chat:              chatDMRef(chatA),
		LastReadMessageId: sendA.GetMessage().GetId(),
	})
	require.NoError(t, err)

	bulk, err := client.GetBulkReadState(withProfileCtx(ctx, acctA, profA), &messagingv1.GetBulkReadStateRequest{
		Chats: []*chatv1.ChatRef{chatDMRef(chatA), chatDMRef(chatB)},
	})
	require.NoError(t, err)
	require.Len(t, bulk.GetByChatId(), 1)
	require.Equal(t, sendA.GetMessage().GetId(), bulk.GetByChatId()[chatA.String()].GetLastReadMessageId())
	_, hasB := bulk.GetByChatId()[chatB.String()]
	require.False(t, hasB)
	_ = sendB
}

func TestMessagingGetReadState_beforeMarkRead(t *testing.T) {
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

	rs, err := client.GetReadState(withProfileCtx(ctx, acctA, profA), &messagingv1.GetReadStateRequest{Chat: chatDMRef(chatID)})
	require.NoError(t, err)
	require.Nil(t, rs.GetReadState())
}

func TestMessagingGetMessages_beforeMessageID(t *testing.T) {
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
	mk := messagingv1.MessageKind_MESSAGE_KIND_REGULAR
	var ids []string
	for i := 0; i < 3; i++ {
		resp, err := client.SendMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.SendMessageRequest{
			Chat: chatDMRef(chatID), Content: "m", AttachmentsJson: "[]", MentionsJson: "[]", MessageKind: &mk,
		})
		require.NoError(t, err)
		ids = append(ids, resp.GetMessage().GetId())
	}
	anchor := ids[2]
	page, err := client.GetMessages(withProfileCtx(ctx, acctA, profA), &messagingv1.GetMessagesRequest{
		Chat:             chatDMRef(chatID),
		BeforeMessageId:  &anchor,
		Page:             &commonv1.CursorPageRequest{PageSize: 10},
	})
	require.NoError(t, err)
	require.NotEmpty(t, page.GetMessageList().GetMessages())
	for _, m := range page.GetMessageList().GetMessages() {
		require.Less(t, m.GetId(), anchor)
	}
}

func TestMessagingSendMessage_kindsAndValidation(t *testing.T) {
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

	sys := messagingv1.MessageKind_MESSAGE_KIND_SYSTEM
	sysResp, err := client.SendMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.SendMessageRequest{
		Chat: chatDMRef(chatID), Content: "sys", AttachmentsJson: "[]", MentionsJson: "[]", MessageKind: &sys,
	})
	require.NoError(t, err)
	require.Equal(t, messagingv1.MessageKind_MESSAGE_KIND_SYSTEM, sysResp.GetMessage().GetMessageKind())

	fwd := messagingv1.MessageKind_MESSAGE_KIND_FORWARD
	fwdResp, err := client.SendMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.SendMessageRequest{
		Chat: chatDMRef(chatID), Content: "fwd", AttachmentsJson: "[]", MentionsJson: "[]", MessageKind: &fwd,
	})
	require.NoError(t, err)
	require.Equal(t, messagingv1.MessageKind_MESSAGE_KIND_FORWARD, fwdResp.GetMessage().GetMessageKind())

	long := strings.Repeat("x", 4001)
	_, err = client.SendMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.SendMessageRequest{
		Chat: chatDMRef(chatID), Content: long, AttachmentsJson: "[]", MentionsJson: "[]",
	})
	require.Equal(t, codes.InvalidArgument, status.Code(err))

	_, err = client.SendMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.SendMessageRequest{
		Chat: chatDMRef(chatID), Content: "   ", AttachmentsJson: "[]", MentionsJson: "[]",
	})
	require.Equal(t, codes.InvalidArgument, status.Code(err))

	badThread := "not-uuid"
	_, err = client.SendMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.SendMessageRequest{
		Chat: chatDMRef(chatID), Content: "t", ThreadParentId: &badThread, AttachmentsJson: "[]", MentionsJson: "[]",
	})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestMessagingGetBulkReadState_nonMemberDenied(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000002_client_message_id.up.sql"))

	chatID := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	profStranger := uuid.New()
	acctS := uuid.New()
	seedDMChat(t, ctx, pool, chatID, profA, profB)
	client, _ := startMessagingServer(t, pool)

	_, err := client.GetBulkReadState(withProfileCtx(ctx, acctS, profStranger), &messagingv1.GetBulkReadStateRequest{
		Chats: []*chatv1.ChatRef{chatDMRef(chatID)},
	})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestMessagingGetBulkReadState_dedupesChatRefs(t *testing.T) {
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

	send, err := client.SendMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.SendMessageRequest{
		Chat: chatDMRef(chatID), Content: "hi", AttachmentsJson: "[]", MentionsJson: "[]",
	})
	require.NoError(t, err)
	_, err = client.MarkRead(withProfileCtx(ctx, acctA, profA), &messagingv1.MarkReadRequest{
		Chat: chatDMRef(chatID), LastReadMessageId: send.GetMessage().GetId(),
	})
	require.NoError(t, err)

	ref := chatDMRef(chatID)
	bulk, err := client.GetBulkReadState(withProfileCtx(ctx, acctA, profA), &messagingv1.GetBulkReadStateRequest{
		Chats: []*chatv1.ChatRef{ref, ref},
	})
	require.NoError(t, err)
	require.Len(t, bulk.GetByChatId(), 1)
}

func TestMessagingGetMessages_afterPageNextCursor(t *testing.T) {
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
	mk := messagingv1.MessageKind_MESSAGE_KIND_REGULAR
	var ids []string
	for i := 0; i < 4; i++ {
		resp, err := client.SendMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.SendMessageRequest{
			Chat: chatDMRef(chatID), Content: "m", AttachmentsJson: "[]", MentionsJson: "[]", MessageKind: &mk,
		})
		require.NoError(t, err)
		ids = append(ids, resp.GetMessage().GetId())
	}
	page, err := client.GetMessages(withProfileCtx(ctx, acctA, profA), &messagingv1.GetMessagesRequest{
		Chat:           chatDMRef(chatID),
		AfterMessageId: &ids[0],
		Page:           &commonv1.CursorPageRequest{PageSize: 2},
	})
	require.NoError(t, err)
	ml := page.GetMessageList()
	require.True(t, ml.GetHasMore())
	thirdID, err := uuid.Parse(ids[2])
	require.NoError(t, err)
	require.Equal(t, store.EncodeAfterCursor(thirdID), ml.GetNextCursor())
}

func TestMessagingGetMessages_cursorFromPageAfterDirection(t *testing.T) {
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
	mk := messagingv1.MessageKind_MESSAGE_KIND_REGULAR
	var ids []string
	for i := 0; i < 4; i++ {
		resp, err := client.SendMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.SendMessageRequest{
			Chat: chatDMRef(chatID), Content: "m", AttachmentsJson: "[]", MentionsJson: "[]", MessageKind: &mk,
		})
		require.NoError(t, err)
		ids = append(ids, resp.GetMessage().GetId())
	}
	first, err := client.GetMessages(withProfileCtx(ctx, acctA, profA), &messagingv1.GetMessagesRequest{
		Chat:           chatDMRef(chatID),
		AfterMessageId: &ids[0],
		Page:           &commonv1.CursorPageRequest{PageSize: 2},
	})
	require.NoError(t, err)
	cursor := first.GetMessageList().GetNextCursor()
	require.NotEmpty(t, cursor)

	second, err := client.GetMessages(withProfileCtx(ctx, acctA, profA), &messagingv1.GetMessagesRequest{
		Chat: chatDMRef(chatID),
		Page: &commonv1.CursorPageRequest{Cursor: cursor, PageSize: 2},
	})
	require.NoError(t, err)
	require.NotEmpty(t, second.GetMessageList().GetMessages())
}

func TestMessagingGetMessages_lastAndAfterDisagree(t *testing.T) {
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
	a := uuid.New().String()
	b := uuid.New().String()
	_, err := client.GetMessages(withProfileCtx(ctx, acctA, profA), &messagingv1.GetMessagesRequest{
		Chat:           chatDMRef(chatID),
		AfterMessageId: &a,
		LastMessageId:  &b,
		Page:           &commonv1.CursorPageRequest{PageSize: 2},
	})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestMessagingSendMessage_chatGuardInternal(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000002_client_message_id.up.sql"))

	client, _ := startMessagingServerWired(t, pool, messagingWire{
		ChatGuard: faultGuard{memberErr: errors.New("chat down")},
	})
	_, err := client.SendMessage(withProfileCtx(ctx, uuid.New(), uuid.New()), &messagingv1.SendMessageRequest{
		Chat: chatDMRef(uuid.New()), Content: "x", AttachmentsJson: "[]", MentionsJson: "[]",
	})
	require.Equal(t, codes.Internal, status.Code(err))
}

func TestMessagingSendMessage_withThreadParent(t *testing.T) {
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

	parent, err := client.SendMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.SendMessageRequest{
		Chat: chatDMRef(chatID), Content: "parent", AttachmentsJson: "[]", MentionsJson: "[]",
	})
	require.NoError(t, err)
	parentID := parent.GetMessage().GetId()
	reply, err := client.SendMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.SendMessageRequest{
		Chat: chatDMRef(chatID), Content: "reply", ThreadParentId: &parentID, AttachmentsJson: "[]", MentionsJson: "[]",
	})
	require.NoError(t, err)
	require.Equal(t, parentID, reply.GetMessage().GetThreadParentId())
}
