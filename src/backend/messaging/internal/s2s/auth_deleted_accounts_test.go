package s2s

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	authv1 "voice.app/voice/auth/v1"
)

type stubDeletedAccountsAuth struct {
	authv1.UnimplementedAuthServiceServer
	deleted  []string
	request  []string
	internal []string
	deadline time.Time
}

func (s *stubDeletedAccountsAuth) FilterDeletedAccountIDs(ctx context.Context, req *authv1.FilterDeletedAccountIDsRequest) (*authv1.FilterDeletedAccountIDsResponse, error) {
	s.request = append([]string(nil), req.GetAccountIds()...)
	s.internal = metadata.ValueFromIncomingContext(ctx, "x-voice-internal")
	s.deadline, _ = ctx.Deadline()
	return &authv1.FilterDeletedAccountIDsResponse{DeletedAccountIds: s.deleted}, nil
}

func startBufconnAuthDeletedAccounts(t *testing.T, impl authv1.AuthServiceServer) (grpc.ClientConnInterface, func()) {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer()
	authv1.RegisterAuthServiceServer(srv, impl)
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

func TestAuthGRPCDeletedAccounts_DeletedAmong(t *testing.T) {
	accountA, accountB := uuid.New(), uuid.New()
	stub := &stubDeletedAccountsAuth{deleted: []string{accountB.String()}}
	conn, cleanup := startBufconnAuthDeletedAccounts(t, stub)
	t.Cleanup(cleanup)

	before := time.Now()
	got, err := NewAuthGRPCDeletedAccounts(authv1.NewAuthServiceClient(conn)).DeletedAmong(context.Background(), []uuid.UUID{accountA, accountB})
	require.NoError(t, err)
	require.Equal(t, []string{accountA.String(), accountB.String()}, stub.request)
	require.Equal(t, []string{"true"}, stub.internal)
	require.True(t, stub.deadline.After(before))
	require.LessOrEqual(t, stub.deadline.Sub(before), authDeletedAccountsTimeout+time.Second)
	require.Equal(t, map[uuid.UUID]struct{}{accountB: {}}, got)
}

func TestAuthGRPCDeletedAccounts_MalformedResponseFailsClosed(t *testing.T) {
	stub := &stubDeletedAccountsAuth{deleted: []string{"not-a-uuid"}}
	conn, cleanup := startBufconnAuthDeletedAccounts(t, stub)
	t.Cleanup(cleanup)

	_, err := NewAuthGRPCDeletedAccounts(authv1.NewAuthServiceClient(conn)).DeletedAmong(context.Background(), []uuid.UUID{uuid.New()})
	require.Equal(t, codes.Unavailable, status.Code(err))
}
