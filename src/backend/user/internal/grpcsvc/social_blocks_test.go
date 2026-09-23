package grpcsvc

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	socialv1 "voice.app/voice/social/v1"
)

type socialBlocksTestServer struct {
	socialv1.UnimplementedSocialServiceServer
	blocked map[string]bool
	err     error
}

func (s socialBlocksTestServer) IsBlocked(_ context.Context, req *socialv1.IsBlockedRequest) (*socialv1.IsBlockedResponse, error) {
	if s.err != nil {
		return nil, s.err
	}
	return &socialv1.IsBlockedResponse{Blocked: s.blocked[req.GetAccountIdA()+":"+req.GetAccountIdB()]}, nil
}

func startSocialBlocksTestClient(t *testing.T, impl socialv1.SocialServiceServer) *SocialGRPCBlocks {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer()
	socialv1.RegisterSocialServiceServer(srv, impl)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(func() {
		srv.Stop()
		_ = lis.Close()
	})
	conn, err := grpc.NewClient("passthrough:///social-blocks",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	return NewSocialGRPCBlocks(conn)
}

func TestSocialGRPCBlocksAccountPairBlockedDeniesReverseBlock(t *testing.T) {
	t.Parallel()
	viewer, other := uuid.New(), uuid.New()
	blocks := startSocialBlocksTestClient(t, socialBlocksTestServer{blocked: map[string]bool{
		other.String() + ":" + viewer.String(): true,
	}})

	blocked, err := blocks.AccountPairBlocked(context.Background(), viewer, other)

	require.NoError(t, err)
	require.True(t, blocked, "a target block must hide the target from the viewer")
}

func TestSocialGRPCBlocksAccountPairBlockedPropagatesBackendFailure(t *testing.T) {
	t.Parallel()
	blocks := startSocialBlocksTestClient(t, socialBlocksTestServer{err: errors.New("social unavailable")})

	blocked, err := blocks.AccountPairBlocked(context.Background(), uuid.New(), uuid.New())

	require.Error(t, err)
	require.False(t, blocked)
}

func TestSocialGRPCBlocksAccountPairBlockedFailsClosedWithoutClient(t *testing.T) {
	t.Parallel()

	for _, blocks := range []*SocialGRPCBlocks{nil, {}} {
		blocked, err := blocks.AccountPairBlocked(context.Background(), uuid.New(), uuid.New())

		require.Error(t, err)
		require.False(t, blocked)
	}
}
