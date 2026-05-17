package grpcsvc

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	"voice/backend/user/internal/authctx"

	userv1 "voice.app/voice/user/v1"
)

func TestUserGRPC_presenceNil_returnsUnavailable(t *testing.T) {
	ctx := context.Background()
	lis := bufconn.Listen(1024 * 1024)
	t.Cleanup(func() { _ = lis.Close() })
	srv := grpc.NewServer()
	userv1.RegisterUserServiceServer(srv, &UserGRPC{Presence: nil})
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	cli := userv1.NewUserServiceClient(conn)

	md := metadata.AppendToOutgoingContext(ctx, authctx.HeaderUserID, "00000000-0000-0000-0000-000000000001")
	_, err = cli.UpdatePresence(md, &userv1.UpdatePresenceRequest{Status: "online"})
	require.Error(t, err)
	require.Equal(t, codes.Unavailable, status.Code(err))

	_, err = cli.GetPresence(ctx, &userv1.GetPresenceRequest{ProfileId: "00000000-0000-0000-0000-000000000002"})
	require.Error(t, err)
	require.Equal(t, codes.Unavailable, status.Code(err))

	_, err = cli.GetBulkPresence(ctx, &userv1.GetBulkPresenceRequest{ProfileIds: []string{"00000000-0000-0000-0000-000000000002"}})
	require.Error(t, err)
	require.Equal(t, codes.Unavailable, status.Code(err))
}
