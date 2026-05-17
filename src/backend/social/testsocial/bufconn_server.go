// Package testsocial hosts Social gRPC over bufconn for cross-service tests.
// It lives outside social/internal so other modules (e.g. voice/backend/user) may import it.
package testsocial

import (
	"context"
	"net"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	socgrpc "voice/backend/social/internal/grpcsvc"
	socialstore "voice/backend/social/internal/store"

	socialv1 "voice.app/voice/social/v1"
)

const defaultBufSize = 1 << 20

// NewBufconnClient returns a gRPC client connection to an in-process SocialService backed by pool.
// Caller must run migrations on pool before use. cleanup closes the client and stops the server.
func NewBufconnClient(t *testing.T, pool *pgxpool.Pool) (grpc.ClientConnInterface, func()) {
	t.Helper()
	lis := bufconn.Listen(defaultBufSize)
	srv := grpc.NewServer()
	socialv1.RegisterSocialServiceServer(srv, &socgrpc.SocialGRPC{
		Friends: &socialstore.FriendshipStore{Pool: pool},
		Blocks:  &socialstore.BlockStore{Pool: pool},
	})
	go func() {
		if err := srv.Serve(lis); err != nil {
			t.Logf("social grpc serve: %v", err)
		}
	}()
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	cleanup := func() {
		_ = conn.Close()
		srv.Stop()
		_ = lis.Close()
	}
	return conn, cleanup
}
