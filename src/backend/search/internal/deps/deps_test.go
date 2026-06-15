package deps

import (
	"context"
	"net"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"

	chatv1 "voice.app/voice/chat/v1"
)

type recordingChatGRPC struct {
	chatv1.UnimplementedChatServiceServer
	lastMD metadata.MD
}

func (s *recordingChatGRPC) ListChats(ctx context.Context, _ *chatv1.ListChatsRequest) (*chatv1.ListChatsResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	s.lastMD = md
	return &chatv1.ListChatsResponse{ChatList: &chatv1.ChatList{}}, nil
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
