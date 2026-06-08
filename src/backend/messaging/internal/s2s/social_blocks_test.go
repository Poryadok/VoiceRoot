package s2s

import (
	"context"
	"net"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	socialv1 "voice.app/voice/social/v1"
)

type stubSocialIsBlocked struct {
	socialv1.UnimplementedSocialServiceServer
	blocked map[string]bool
	err     error
}

func (s *stubSocialIsBlocked) IsBlocked(_ context.Context, req *socialv1.IsBlockedRequest) (*socialv1.IsBlockedResponse, error) {
	if s.err != nil {
		return nil, s.err
	}
	key := req.GetAccountIdA() + ">" + req.GetAccountIdB()
	return &socialv1.IsBlockedResponse{Blocked: s.blocked[key]}, nil
}

func startBufconnSocial(t *testing.T, impl socialv1.SocialServiceServer) (grpc.ClientConnInterface, func()) {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer()
	socialv1.RegisterSocialServiceServer(srv, impl)
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

func TestSocialGRPCBlocks_AccountPairBlocked(t *testing.T) {
	t.Parallel()

	viewer := uuid.MustParse("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")
	other := uuid.MustParse("bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb")

	t.Run("nil client", func(t *testing.T) {
		t.Parallel()
		var b *SocialGRPCBlocks
		ok, err := b.AccountPairBlocked(context.Background(), viewer, other)
		require.NoError(t, err)
		require.False(t, ok)
	})

	t.Run("same account", func(t *testing.T) {
		t.Parallel()
		conn, cleanup := startBufconnSocial(t, &stubSocialIsBlocked{})
		t.Cleanup(cleanup)
		b := NewSocialGRPCBlocks(conn)
		ok, err := b.AccountPairBlocked(context.Background(), viewer, viewer)
		require.NoError(t, err)
		require.False(t, ok)
	})

	t.Run("blocked viewer to other", func(t *testing.T) {
		t.Parallel()
		stub := &stubSocialIsBlocked{blocked: map[string]bool{
			viewer.String() + ">" + other.String(): true,
		}}
		conn, cleanup := startBufconnSocial(t, stub)
		t.Cleanup(cleanup)
		b := NewSocialGRPCBlocks(conn)
		ok, err := b.AccountPairBlocked(context.Background(), viewer, other)
		require.NoError(t, err)
		require.True(t, ok)
	})

	t.Run("blocked other to viewer", func(t *testing.T) {
		t.Parallel()
		stub := &stubSocialIsBlocked{blocked: map[string]bool{
			other.String() + ">" + viewer.String(): true,
		}}
		conn, cleanup := startBufconnSocial(t, stub)
		t.Cleanup(cleanup)
		b := NewSocialGRPCBlocks(conn)
		ok, err := b.AccountPairBlocked(context.Background(), viewer, other)
		require.NoError(t, err)
		require.True(t, ok)
	})

	t.Run("grpc error", func(t *testing.T) {
		t.Parallel()
		stub := &stubSocialIsBlocked{err: status.Error(codes.Unavailable, "social down")}
		conn, cleanup := startBufconnSocial(t, stub)
		t.Cleanup(cleanup)
		b := NewSocialGRPCBlocks(conn)
		_, err := b.AccountPairBlocked(context.Background(), viewer, other)
		require.Error(t, err)
		require.Equal(t, codes.Unavailable, status.Code(err))
	})
}
