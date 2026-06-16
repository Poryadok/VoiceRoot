package deps

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

	chatv1 "voice.app/voice/chat/v1"
)

type recordingChatGRPC struct {
	chatv1.UnimplementedChatServiceServer
	lastMD        metadata.MD
	lastGetChatID string
	getChatErr    error
}

func (s *recordingChatGRPC) GetChat(ctx context.Context, req *chatv1.GetChatRequest) (*chatv1.GetChatResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	s.lastMD = md
	s.lastGetChatID = req.GetChatId()
	if s.getChatErr != nil {
		return nil, s.getChatErr
	}
	return &chatv1.GetChatResponse{Chat: &chatv1.Chat{Id: req.GetChatId()}}, nil
}

func (s *recordingChatGRPC) ListChats(ctx context.Context, _ *chatv1.ListChatsRequest) (*chatv1.ListChatsResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	s.lastMD = md
	return &chatv1.ListChatsResponse{ChatList: &chatv1.ChatList{}}, nil
}

func TestChatReadAccess_UsesGetChatForMembership(t *testing.T) {
	t.Parallel()
	profileID := uuid.New()
	chatID := uuid.New()
	srv := &recordingChatGRPC{getChatErr: nil}
	grpcSrv := grpc.NewServer()
	chatv1.RegisterChatServiceServer(grpcSrv, srv)
	lis := bufconn.Listen(1 << 20)
	go func() { _ = grpcSrv.Serve(lis) }()
	t.Cleanup(func() {
		grpcSrv.Stop()
		_ = lis.Close()
	})

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	access := &ChatReadAccess{Client: chatv1.NewChatServiceClient(conn)}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		"x-voice-profile-id", profileID.String(),
	))
	ok, err := access.CanReadMessages(ctx, profileID, chatID)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, chatID.String(), srv.lastGetChatID)
}

func TestChatReadAccess_DeniesWhenNotMember(t *testing.T) {
	t.Parallel()
	profileID := uuid.New()
	chatID := uuid.New()
	srv := &recordingChatGRPC{
		getChatErr: status.Error(codes.PermissionDenied, "not a chat member"),
	}
	grpcSrv := grpc.NewServer()
	chatv1.RegisterChatServiceServer(grpcSrv, srv)
	lis := bufconn.Listen(1 << 20)
	go func() { _ = grpcSrv.Serve(lis) }()
	t.Cleanup(func() {
		grpcSrv.Stop()
		_ = lis.Close()
	})

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	access := &ChatReadAccess{Client: chatv1.NewChatServiceClient(conn)}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		"x-voice-profile-id", profileID.String(),
	))
	ok, err := access.CanReadMessages(ctx, profileID, chatID)
	require.NoError(t, err)
	require.False(t, ok)
}

func TestChatMembership_ForwardsProfileMetadata(t *testing.T) {
	t.Parallel()
	profileID := uuid.New()
	srv := &recordingChatGRPC{}
	grpcSrv := grpc.NewServer()
	chatv1.RegisterChatServiceServer(grpcSrv, srv)
	lis := bufconn.Listen(1 << 20)
	go func() { _ = grpcSrv.Serve(lis) }()
	t.Cleanup(func() {
		grpcSrv.Stop()
		_ = lis.Close()
	})

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	membership := &ChatMembership{Client: chatv1.NewChatServiceClient(conn)}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		"x-voice-profile-id", profileID.String(),
		"x-voice-user-id", uuid.New().String(),
	))
	_, err = membership.AccessibleChatIDs(ctx, profileID)
	require.NoError(t, err)
	got := srv.lastMD.Get("x-voice-profile-id")
	require.Len(t, got, 1)
	require.Equal(t, profileID.String(), got[0])
}
