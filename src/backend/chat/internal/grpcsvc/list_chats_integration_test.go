package grpcsvc

import (
	"context"
	"net"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	commonv1 "voice.app/voice/common/v1"
	messagingv1 "voice.app/voice/messaging/v1"

	chatv1 "voice.app/voice/chat/v1"
)

type mapListEnricher map[uuid.UUID]ListChatExtra

func (m mapListEnricher) EnrichListChats(_ context.Context, _ uuid.UUID, chatIDs []uuid.UUID) (map[uuid.UUID]ListChatExtra, error) {
	out := make(map[uuid.UUID]ListChatExtra)
	for _, id := range chatIDs {
		if x, ok := m[id]; ok {
			out[id] = x
		}
	}
	return out, nil
}

// TestListChats_SortsByLastMessageAt documents ordering: COALESCE(last_message_at, created_at) DESC.
func TestListChats_SortsByLastMessageAt(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatPostgresForTest(t, ctx)
	applyChatMigration(t, ctx, pool)

	accA := uuid.New()
	accB := uuid.New()
	accC := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	profC := uuid.New()
	profiles := mapProfileAccounts{profA: accA, profB: accB, profC: accC}

	client, cleanup := startChatGRPCTestServer(t, pool, profiles, nil, nil)
	t.Cleanup(cleanup)

	ctxA := withAccountProfileCtx(ctx, accA, profA)
	rB, err := client.CreateDM(ctxA, &chatv1.CreateDMRequest{OtherProfileId: profB.String()})
	require.NoError(t, err)
	rC, err := client.CreateDM(ctxA, &chatv1.CreateDMRequest{OtherProfileId: profC.String()})
	require.NoError(t, err)
	chatB := rB.GetChat().GetId()
	chatC := rC.GetChat().GetId()

	_, err = pool.Exec(ctx, `UPDATE chats SET last_message_at = now() + interval '2 hours' WHERE id = $1::uuid`, chatB)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE chats SET last_message_at = now() WHERE id = $1::uuid`, chatC)
	require.NoError(t, err)

	list, err := client.ListChats(ctxA, &chatv1.ListChatsRequest{
		Page: &commonv1.CursorPageRequest{PageSize: 10},
	})
	require.NoError(t, err)
	items := list.GetChatList().GetItems()
	require.Len(t, items, 2)
	require.Equal(t, chatB, items[0].GetChat().GetId(), "newer last_message_at must sort first")
	require.Equal(t, chatC, items[1].GetChat().GetId())
}

func TestListChats_Pagination(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatPostgresForTest(t, ctx)
	applyChatMigration(t, ctx, pool)

	accA := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	profC := uuid.New()
	profD := uuid.New()
	profiles := mapProfileAccounts{profA: accA, profB: accA, profC: accA, profD: accA}

	client, cleanup := startChatGRPCTestServer(t, pool, profiles, nil, nil)
	t.Cleanup(cleanup)
	ctxA := withAccountProfileCtx(ctx, accA, profA)

	var ids []string
	for _, other := range []uuid.UUID{profB, profC, profD} {
		r, err := client.CreateDM(ctxA, &chatv1.CreateDMRequest{OtherProfileId: other.String()})
		require.NoError(t, err)
		ids = append(ids, r.GetChat().GetId())
	}
	for i, id := range ids {
		_, err := pool.Exec(ctx, `UPDATE chats SET last_message_at = now() + ($1::int * interval '1 minute') WHERE id = $2::uuid`, i, id)
		require.NoError(t, err)
	}

	r1, err := client.ListChats(ctxA, &chatv1.ListChatsRequest{
		Page: &commonv1.CursorPageRequest{PageSize: 1},
	})
	require.NoError(t, err)
	require.Len(t, r1.GetChatList().GetItems(), 1)
	require.Equal(t, ids[2], r1.GetChatList().GetItems()[0].GetChat().GetId())
	cur := r1.GetChatList().GetNextCursor()
	require.NotEmpty(t, cur)

	r2, err := client.ListChats(ctxA, &chatv1.ListChatsRequest{
		Page: &commonv1.CursorPageRequest{Cursor: cur, PageSize: 1},
	})
	require.NoError(t, err)
	require.Len(t, r2.GetChatList().GetItems(), 1)
	require.Equal(t, ids[1], r2.GetChatList().GetItems()[0].GetChat().GetId())
}

func TestListChats_EnrichmentFromMessagingHook(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatPostgresForTest(t, ctx)
	applyChatMigration(t, ctx, pool)

	accA := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	profiles := mapProfileAccounts{profA: accA, profB: accA}

	enrich := make(mapListEnricher)
	client, cleanup := startChatGRPCTestServer(t, pool, profiles, nil, enrich)
	t.Cleanup(cleanup)
	ctxA := withAccountProfileCtx(ctx, accA, profA)
	r, err := client.CreateDM(ctxA, &chatv1.CreateDMRequest{OtherProfileId: profB.String()})
	require.NoError(t, err)
	cid, err := uuid.Parse(r.GetChat().GetId())
	require.NoError(t, err)
	enrich[cid] = ListChatExtra{LastMessagePreview: "hello from messaging", UnreadCount: 4}

	list, err := client.ListChats(ctxA, &chatv1.ListChatsRequest{
		Page: &commonv1.CursorPageRequest{PageSize: 10},
	})
	require.NoError(t, err)
	require.Len(t, list.GetChatList().GetItems(), 1)
	it := list.GetChatList().GetItems()[0]
	require.Equal(t, "hello from messaging", it.GetLastMessagePreview())
	require.Equal(t, int64(4), it.GetUnreadCount())
}

type recordingMessagingMetadata struct {
	messagingv1.UnimplementedMessagingServiceServer
	lastMD metadata.MD
	byID   map[string]*messagingv1.ChatListMetadata
}

func (s *recordingMessagingMetadata) GetChatListMetadata(ctx context.Context, req *messagingv1.GetChatListMetadataRequest) (*messagingv1.GetChatListMetadataResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	s.lastMD = md
	out := make(map[string]*messagingv1.ChatListMetadata)
	for _, ref := range req.GetChats() {
		if item, ok := s.byID[ref.GetId()]; ok {
			out[ref.GetId()] = item
		}
	}
	return &messagingv1.GetChatListMetadataResponse{ByChatId: out}, nil
}

func startMessagingMetadataTestClient(t *testing.T, srv messagingv1.MessagingServiceServer) (messagingv1.MessagingServiceClient, func()) {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	grpcSrv := grpc.NewServer()
	messagingv1.RegisterMessagingServiceServer(grpcSrv, srv)
	go func() {
		if err := grpcSrv.Serve(lis); err != nil {
			t.Logf("messaging grpc serve: %v", err)
		}
	}()
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	return messagingv1.NewMessagingServiceClient(conn), func() {
		_ = conn.Close()
		grpcSrv.Stop()
		_ = lis.Close()
	}
}

func TestListChats_EnrichmentFromMessagingGRPC(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatPostgresForTest(t, ctx)
	applyChatMigration(t, ctx, pool)

	accA := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	profiles := mapProfileAccounts{profA: accA, profB: uuid.New()}

	now := timestamppb.Now()
	rec := &recordingMessagingMetadata{byID: map[string]*messagingv1.ChatListMetadata{}}
	msgClient, cleanupMsg := startMessagingMetadataTestClient(t, rec)
	t.Cleanup(cleanupMsg)

	enrich := NewMessagingListEnricher(msgClient)
	client, cleanup := startChatGRPCTestServer(t, pool, profiles, nil, enrich)
	t.Cleanup(cleanup)

	ctxA := withAccountProfileCtx(ctx, accA, profA)
	dm, err := client.CreateDM(ctxA, &chatv1.CreateDMRequest{OtherProfileId: profB.String()})
	require.NoError(t, err)
	chatID := dm.GetChat().GetId()
	rec.byID[chatID] = &messagingv1.ChatListMetadata{
		Chat:               &chatv1.ChatRef{Id: chatID},
		LastMessagePreview: proto.String("from grpc metadata"),
		UnreadCount:        7,
		LastMessageAt:      now,
	}

	list, err := client.ListChats(ctxA, &chatv1.ListChatsRequest{
		Page: &commonv1.CursorPageRequest{PageSize: 10},
	})
	require.NoError(t, err)
	require.Len(t, list.GetChatList().GetItems(), 1)
	item := list.GetChatList().GetItems()[0]
	require.Equal(t, "from grpc metadata", item.GetLastMessagePreview())
	require.Equal(t, int64(7), item.GetUnreadCount())
	require.Equal(t, profA.String(), rec.lastMD.Get("x-voice-profile-id")[0], "Chat must forward caller metadata to Messaging")
}

func TestListChats_InvalidCursor(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatPostgresForTest(t, ctx)
	applyChatMigration(t, ctx, pool)
	accA := uuid.New()
	profA := uuid.New()
	profiles := mapProfileAccounts{profA: accA}
	client, cleanup := startChatGRPCTestServer(t, pool, profiles, nil, nil)
	t.Cleanup(cleanup)
	ctxA := withAccountProfileCtx(ctx, accA, profA)

	_, err := client.ListChats(ctxA, &chatv1.ListChatsRequest{
		Page: &commonv1.CursorPageRequest{Cursor: "not-a-valid-cursor", PageSize: 10},
	})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestListChats_MissingAuth_Unauthenticated(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatPostgresForTest(t, ctx)
	applyChatMigration(t, ctx, pool)
	profA := uuid.New()
	profiles := mapProfileAccounts{profA: uuid.New()}
	client, cleanup := startChatGRPCTestServer(t, pool, profiles, nil, nil)
	t.Cleanup(cleanup)

	_, err := client.ListChats(ctx, &chatv1.ListChatsRequest{
		Page: &commonv1.CursorPageRequest{PageSize: 10},
	})
	require.Error(t, err)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestListChats_DefaultPageSize(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatPostgresForTest(t, ctx)
	applyChatMigration(t, ctx, pool)
	accA := uuid.New()
	profA := uuid.New()
	profiles := mapProfileAccounts{profA: accA}
	client, cleanup := startChatGRPCTestServer(t, pool, profiles, nil, nil)
	t.Cleanup(cleanup)
	ctxA := withAccountProfileCtx(ctx, accA, profA)
	// 51 DMs — default page size 50 + non-empty next_cursor
	for i := 0; i < 51; i++ {
		other := uuid.New()
		profiles[other] = accA
		_, err := client.CreateDM(ctxA, &chatv1.CreateDMRequest{OtherProfileId: other.String()})
		require.NoError(t, err)
	}
	list, err := client.ListChats(ctxA, &chatv1.ListChatsRequest{Page: nil})
	require.NoError(t, err)
	require.Len(t, list.GetChatList().GetItems(), 50)
	require.NotEmpty(t, list.GetChatList().GetNextCursor())
}
