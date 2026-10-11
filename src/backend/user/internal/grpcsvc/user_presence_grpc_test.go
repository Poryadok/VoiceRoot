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
	"voice/backend/user/internal/store"

	userv1 "voice.app/voice/user/v1"
)

func TestHasNotificationRoutingSessionUsesOnlyActiveVisibleStates(t *testing.T) {
	tests := []struct {
		name     string
		snapshot *store.PresenceSnapshot
		want     bool
		wantErr  bool
	}{
		{name: "online", snapshot: &store.PresenceSnapshot{Live: true, Status: "online"}, want: true},
		{name: "idle", snapshot: &store.PresenceSnapshot{Live: true, Status: "idle"}, want: true},
		{name: "dnd", snapshot: &store.PresenceSnapshot{Live: true, Status: "dnd"}, want: true},
		{name: "invisible remains push eligible", snapshot: &store.PresenceSnapshot{Live: true, Status: "invisible", StatusEnum: int32(userv1.PresenceOnlineStatus_PRESENCE_ONLINE_STATUS_INVISIBLE)}, want: false},
		{name: "offline", snapshot: &store.PresenceSnapshot{Live: false, Status: "offline"}, want: false},
		{name: "unknown live status is unavailable", snapshot: &store.PresenceSnapshot{Live: true, Status: "unknown"}, wantErr: true},
		{name: "unknown live enum is unavailable", snapshot: &store.PresenceSnapshot{Live: true, StatusEnum: 99}, wantErr: true},
		{name: "missing presence", snapshot: nil, want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := hasNotificationRoutingSession(tc.snapshot)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			if got != tc.want {
				t.Fatalf("hasNotificationRoutingSession() = %v, want %v", got, tc.want)
			}
		})
	}
}

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
