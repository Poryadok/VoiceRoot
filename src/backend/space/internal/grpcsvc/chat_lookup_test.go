package grpcsvc

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
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

	"voice/backend/space/internal/store"

	chatv1 "voice.app/voice/chat/v1"
)

type chatLookupServer struct {
	chatv1.UnimplementedChatServiceServer
	chat  *chatv1.Chat
	chats map[string]*chatv1.Chat
	err   error
	errs  map[string]error
	md    metadata.MD
}

func (s *chatLookupServer) GetChat(ctx context.Context, req *chatv1.GetChatRequest) (*chatv1.GetChatResponse, error) {
	s.md, _ = metadata.FromIncomingContext(ctx)
	if err := s.errs[req.GetChatId()]; err != nil {
		return nil, err
	}
	if s.err != nil {
		return nil, s.err
	}
	if chat, ok := s.chats[req.GetChatId()]; ok {
		return &chatv1.GetChatResponse{Chat: chat}, nil
	}
	return &chatv1.GetChatResponse{Chat: s.chat}, nil
}

func startChatLookupServer(t *testing.T, impl chatv1.ChatServiceServer) (grpc.ClientConnInterface, func()) {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer()
	chatv1.RegisterChatServiceServer(srv, impl)
	go func() { _ = srv.Serve(lis) }()
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	return conn, func() {
		_ = conn.Close()
		srv.Stop()
		_ = lis.Close()
	}
}

func TestGRPCChatLookup_UsesSpaceCallerWithoutForwardingCallerContext(t *testing.T) {
	t.Parallel()

	chatID := uuid.New()
	server := &chatLookupServer{chat: &chatv1.Chat{Id: chatID.String(), Name: stringPtr("Raid planning")}}
	conn, cleanup := startChatLookupServer(t, server)
	t.Cleanup(cleanup)

	lookup := NewGRPCChatLookup(conn)
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		"authorization", "Bearer viewer-token",
		"x-voice-user-id", uuid.NewString(),
		"x-voice-profile-id", uuid.NewString(),
		"x-voice-internal-caller", "untrusted-forwarded-caller",
		"x-unrelated", "must-not-forward",
	))
	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs("x-voice-internal-caller", "stale-outgoing-caller"))

	info, err := lookup.GetChatNames(ctx, []uuid.UUID{chatID})
	require.NoError(t, err)
	require.Equal(t, ChatInfo{Name: "Raid planning"}, info[chatID])
	require.Equal(t, []string{"space"}, server.md.Get("x-voice-internal-caller"))
	require.Empty(t, server.md.Get("authorization"))
	require.Empty(t, server.md.Get("x-voice-user-id"))
	require.Empty(t, server.md.Get("x-voice-profile-id"))
	require.Empty(t, server.md.Get("x-unrelated"))
}

func TestSpaceTreeDataToProto_WarnsAndSkipsEnrichmentWhenChatLookupFails(t *testing.T) {
	t.Parallel()

	chatID := uuid.New()
	var logs bytes.Buffer
	svc := &SpaceGRPC{
		Chats:  failingChatLookup{err: errors.New("caller-context-must-not-leak")},
		Logger: slog.New(slog.NewTextHandler(&logs, nil)),
	}

	tree := svc.spaceTreeDataToProto(context.Background(), &store.SpaceTreeData{Nodes: []*store.TreeNodeRow{{
		ID:      uuid.New(),
		SpaceID: uuid.New(),
		Kind:    "text_chat",
		ChatID:  &chatID,
	}}})

	require.Len(t, tree.GetNodes(), 1)
	require.Empty(t, tree.GetNodes()[0].GetDisplayName())
	require.Contains(t, logs.String(), "space chat lookup failed")
	require.NotContains(t, logs.String(), "caller-context-must-not-leak")
}

func TestGRPCChatLookup_ReturnsSanitizedErrorForFailedLookup(t *testing.T) {
	t.Parallel()

	conn, cleanup := startChatLookupServer(t, &chatLookupServer{
		err: status.Error(codes.Unavailable, "caller-context-must-not-leak"),
	})
	t.Cleanup(cleanup)

	_, err := NewGRPCChatLookup(conn).GetChatNames(context.Background(), []uuid.UUID{uuid.New()})
	require.EqualError(t, err, "chat lookup failed")
}

func TestGRPCChatLookup_PreservesPartialEnrichmentWhenOneLookupFails(t *testing.T) {
	t.Parallel()

	firstID := uuid.New()
	failedID := uuid.New()
	lastID := uuid.New()
	conn, cleanup := startChatLookupServer(t, &chatLookupServer{
		chats: map[string]*chatv1.Chat{
			firstID.String(): {Id: firstID.String(), Name: stringPtr("First")},
			lastID.String():  {Id: lastID.String(), Name: stringPtr("Last")},
		},
		errs: map[string]error{failedID.String(): status.Error(codes.Unavailable, "chat unavailable")},
	})
	t.Cleanup(cleanup)

	lookup := NewGRPCChatLookup(conn)
	info, err := lookup.GetChatNames(context.Background(), []uuid.UUID{firstID, failedID, lastID})
	require.EqualError(t, err, "chat lookup failed")
	require.Equal(t, ChatInfo{Name: "First"}, info[firstID])
	require.Equal(t, ChatInfo{Name: "Last"}, info[lastID])
	require.NotContains(t, info, failedID)

	tree := (&SpaceGRPC{Chats: lookup}).spaceTreeDataToProto(context.Background(), &store.SpaceTreeData{Nodes: []*store.TreeNodeRow{
		{ID: uuid.New(), SpaceID: uuid.New(), Kind: "text_chat", ChatID: &firstID},
		{ID: uuid.New(), SpaceID: uuid.New(), Kind: "text_chat", ChatID: &failedID},
		{ID: uuid.New(), SpaceID: uuid.New(), Kind: "text_chat", ChatID: &lastID},
	}})
	require.Equal(t, "First", tree.GetNodes()[0].GetDisplayName())
	require.Empty(t, tree.GetNodes()[1].GetDisplayName())
	require.Equal(t, "Last", tree.GetNodes()[2].GetDisplayName())
}

type failingChatLookup struct{ err error }

func (l failingChatLookup) GetChatNames(context.Context, []uuid.UUID) (map[uuid.UUID]ChatInfo, error) {
	return nil, l.err
}

func stringPtr(v string) *string { return &v }
