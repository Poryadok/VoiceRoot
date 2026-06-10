package testutil

import (
	"context"
	"net"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	"voice/backend/role/internal/grpcsvc"
	"voice/backend/role/internal/store"

	rolev1 "voice.app/voice/role/v1"
)

// StartRoleGRPC boots role_db (when pool is nil) and an in-process RoleService for cross-service tests.
func StartRoleGRPC(t *testing.T, pool *pgxpool.Pool) (rolev1.RoleServiceClient, func()) {
	t.Helper()
	var cleanupPool func()
	if pool == nil {
		ctx := context.Background()
		pool = StartRoleDB(t, ctx)
		ApplyRoleMigrations(t, ctx, pool)
		cleanupPool = func() { pool.Close() }
	}
	const bufSize = 1 << 20
	lis := bufconn.Listen(bufSize)
	srv := grpc.NewServer()
	rolev1.RegisterRoleServiceServer(srv, &grpcsvc.RoleGRPC{Store: &store.RoleStore{Pool: pool}})
	go func() { _ = srv.Serve(lis) }()
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	return rolev1.NewRoleServiceClient(conn), func() {
		_ = conn.Close()
		srv.Stop()
		if cleanupPool != nil {
			cleanupPool()
		}
	}
}
