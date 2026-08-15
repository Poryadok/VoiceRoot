package s2s

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

	"voice/backend/pkg/privacy"

	userv1 "voice.app/voice/user/v1"
)

type stubUserPrivacy struct {
	userv1.UnimplementedUserServiceServer
	lastMD metadata.MD
}

func (s *stubUserPrivacy) GetPrivacySettings(ctx context.Context, req *userv1.GetPrivacySettingsRequest) (*userv1.GetPrivacySettingsResponse, error) {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		s.lastMD = md.Copy()
	}
	aud := privacy.ToProto(privacy.EveryoneWithGuests())
	return &userv1.GetPrivacySettingsResponse{
		PrivacySettings: &userv1.PrivacySettings{
			ProfileId:           req.GetProfileId(),
			AllowFriendRequests: aud,
			AllowPhoneSearch:    aud,
		},
	}, nil
}

func startBufconnUser(t *testing.T, impl userv1.UserServiceServer) (grpc.ClientConnInterface, func()) {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer()
	userv1.RegisterUserServiceServer(srv, impl)
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

func TestGRPCUserPrivacy_SendsInternalCaller(t *testing.T) {
	t.Parallel()
	stub := &stubUserPrivacy{}
	conn, cleanup := startBufconnUser(t, stub)
	t.Cleanup(cleanup)

	client := &GRPCUserPrivacy{Client: userv1.NewUserServiceClient(conn)}
	target := uuid.New()

	// Incoming end-user MD must not be forwarded (would trip ownership checks).
	inCtx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		"x-voice-user-id", uuid.New().String(),
		"x-voice-profile-id", uuid.New().String(),
	))

	aud, err := client.AllowFriendRequestsAudience(inCtx, target)
	require.NoError(t, err)
	require.True(t, aud.IsEveryoneShortcut())
	require.Equal(t, []string{"social"}, stub.lastMD.Get("x-voice-internal-caller"))
	require.Empty(t, stub.lastMD.Get("x-voice-user-id"))
	require.Empty(t, stub.lastMD.Get("x-voice-profile-id"))
}
