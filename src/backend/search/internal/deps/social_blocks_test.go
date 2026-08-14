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

	socialv1 "voice.app/voice/social/v1"
)

type stubSocialIsBlocked struct {
	socialv1.UnimplementedSocialServiceServer
	blocked map[string]bool
}

func (s *stubSocialIsBlocked) IsBlocked(_ context.Context, req *socialv1.IsBlockedRequest) (*socialv1.IsBlockedResponse, error) {
	key := req.GetAccountIdA() + ":" + req.GetAccountIdB()
	return &socialv1.IsBlockedResponse{Blocked: s.blocked[key]}, nil
}

func startSocialBufconn(t *testing.T, srv socialv1.SocialServiceServer) socialv1.SocialServiceClient {
	t.Helper()
	grpcSrv := grpc.NewServer()
	socialv1.RegisterSocialServiceServer(grpcSrv, srv)
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
	return socialv1.NewSocialServiceClient(conn)
}

func TestSocialBlocks_AccountPairBlocked_Bidirectional(t *testing.T) {
	t.Parallel()
	viewer := uuid.New()
	other := uuid.New()
	stub := &stubSocialIsBlocked{blocked: map[string]bool{
		other.String() + ":" + viewer.String(): true,
	}}
	client := startSocialBufconn(t, stub)
	blocks := &SocialBlocks{Client: client}
	ctx := context.Background()

	blocked, err := blocks.AccountPairBlocked(ctx, viewer, other)
	require.NoError(t, err)
	require.True(t, blocked, "reverse block must hide profile from viewer")
}

func TestSocialBlocks_AccountPairBlocked_Outgoing(t *testing.T) {
	t.Parallel()
	viewer := uuid.New()
	other := uuid.New()
	stub := &stubSocialIsBlocked{blocked: map[string]bool{
		viewer.String() + ":" + other.String(): true,
	}}
	client := startSocialBufconn(t, stub)
	blocks := &SocialBlocks{Client: client}

	blocked, err := blocks.AccountPairBlocked(context.Background(), viewer, other)
	require.NoError(t, err)
	require.True(t, blocked)
}

func TestSocialBlocks_AccountPairBlocked_NoBlock(t *testing.T) {
	t.Parallel()
	viewer := uuid.New()
	other := uuid.New()
	client := startSocialBufconn(t, &stubSocialIsBlocked{blocked: map[string]bool{}})
	blocks := &SocialBlocks{Client: client}

	blocked, err := blocks.AccountPairBlocked(context.Background(), viewer, other)
	require.NoError(t, err)
	require.False(t, blocked)
}

func TestSocialBlocks_AccountPairBlocked_ForwardsMetadata(t *testing.T) {
	t.Parallel()
	viewer := uuid.New()
	other := uuid.New()
	recording := &recordingSocialIsBlocked{}
	client := startSocialBufconn(t, recording)
	blocks := &SocialBlocks{Client: client}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		"x-voice-profile-id", uuid.New().String(),
		"x-voice-user-id", viewer.String(),
	))
	_, err := blocks.AccountPairBlocked(ctx, viewer, other)
	require.NoError(t, err)
	require.NotEmpty(t, recording.lastMD.Get("x-voice-user-id"))
}

type recordingSocialIsBlocked struct {
	socialv1.UnimplementedSocialServiceServer
	lastMD metadata.MD
}

func (s *recordingSocialIsBlocked) IsBlocked(ctx context.Context, _ *socialv1.IsBlockedRequest) (*socialv1.IsBlockedResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	s.lastMD = md
	return &socialv1.IsBlockedResponse{Blocked: false}, nil
}
