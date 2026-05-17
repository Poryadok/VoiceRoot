package grpcsvc

import (
	"bytes"
	"context"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	"voice/backend/chat/testchat"
	"voice/backend/messaging/internal/authctx"
	"voice/backend/messaging/internal/s2s"
	"voice/backend/messaging/internal/store"

	chatv1 "voice.app/voice/chat/v1"
	commonv1 "voice.app/voice/common/v1"
	messagingv1 "voice.app/voice/messaging/v1"
)

func init() {
	if runtime.GOOS == "windows" && os.Getenv("TESTCONTAINERS_RYUK_DISABLED") == "" {
		_ = os.Setenv("TESTCONTAINERS_RYUK_DISABLED", "true")
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", ".."))
}

func startPostgresForTest(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()
	pgC, err := postgres.Run(ctx, "postgres:16-bookworm",
		postgres.BasicWaitStrategies(),
		postgres.WithDatabase("db"),
		postgres.WithUsername("u"),
		postgres.WithPassword("p"),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pgC.Terminate(ctx) })

	connStr, err := pgC.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	connStr = strings.Replace(connStr, "localhost", "127.0.0.1", 1)
	connStr = strings.Replace(connStr, "[::1]", "127.0.0.1", 1)

	var pool *pgxpool.Pool
	for i := 0; i < 60; i++ {
		p, err := pgxpool.New(ctx, connStr)
		if err == nil {
			if pingErr := p.Ping(ctx); pingErr == nil {
				pool = p
				break
			}
			p.Close()
		}
		time.Sleep(500 * time.Millisecond)
	}
	require.NotNil(t, pool, "postgres did not become ready in time")
	t.Cleanup(pool.Close)
	return pool
}

func applySQLFile(t *testing.T, ctx context.Context, pool *pgxpool.Pool, relPath string) {
	t.Helper()
	p := filepath.Join(repoRoot(t), relPath)
	b, err := os.ReadFile(p)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(b))
	require.NoError(t, err)
}

func withProfileCtx(ctx context.Context, accountID, profileID uuid.UUID) context.Context {
	ctx = metadata.AppendToOutgoingContext(ctx, authctx.HeaderUserID, accountID.String())
	return metadata.AppendToOutgoingContext(ctx, authctx.HeaderProfileID, profileID.String())
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
	messagingv1.RegisterMessagingServiceServer(srv, &MessagingGRPC{
		Messages:     &store.MessagesStore{Pool: pool},
		ChatGuard:    guard,
		Blocks:       w.Blocks,
		UserProfiles: w.UserProfiles,
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
	ChatGuard    ChatGuard
	UserProfiles ProfileAccountLookup
	Blocks       AccountPairBlockChecker
}

func startMessagingServer(t *testing.T, pool *pgxpool.Pool) (messagingv1.MessagingServiceClient, func()) {
	t.Helper()
	return startMessagingServerWired(t, pool, messagingWire{})
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
