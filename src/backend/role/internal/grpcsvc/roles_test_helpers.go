package grpcsvc

import (
	"context"
	"net"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	"voice/backend/role/internal/store"

	rolev1 "voice.app/voice/role/v1"
)

func startRoleGRPCTestServer(t *testing.T, pool *pgxpool.Pool, opts ...func(*RoleGRPC)) (rolev1.RoleServiceClient, func()) {
	t.Helper()
	const bufSize = 1 << 20
	lis := bufconn.Listen(bufSize)
	srv := grpc.NewServer()
	svc := &RoleGRPC{Store: &store.RoleStore{Pool: pool}}
	for _, o := range opts {
		o(svc)
	}
	rolev1.RegisterRoleServiceServer(srv, svc)
	go func() {
		if err := srv.Serve(lis); err != nil {
			t.Logf("grpc serve: %v", err)
		}
	}()
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	cleanup := func() {
		_ = conn.Close()
		srv.Stop()
	}
	return rolev1.NewRoleServiceClient(conn), cleanup
}
