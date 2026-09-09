package grpcsvc

import (
	"context"
	"net"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	userv1 "voice.app/voice/user/v1"
)

func TestUserGRPCProfileDowngrade_forwardsVerifiedAccountID(t *testing.T) {
	t.Parallel()

	listener := bufconn.Listen(1 << 20)
	server := grpc.NewServer()
	recorder := &downgradeMetadataRecorder{}
	userv1.RegisterUserServiceServer(server, recorder)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)

	conn, err := grpc.NewClient(
		"passthrough:///user-profile-downgrade-test",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	accountID := uuid.New()
	client := &UserGRPCProfileDowngrade{Client: userv1.NewUserServiceClient(conn)}
	require.NoError(t, client.ApplyDowngradeProfiles(context.Background(), accountID, []uuid.UUID{uuid.New(), uuid.New()}))
	require.Equal(t, accountID.String(), recorder.accountID)
}

func TestUserGRPCProfileDowngrade_rejectsNilClient(t *testing.T) {
	t.Parallel()

	var typedNil *UserGRPCProfileDowngrade
	err := typedNil.ApplyDowngradeProfiles(context.Background(), uuid.New(), []uuid.UUID{uuid.New(), uuid.New()})

	require.Error(t, err)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}

type downgradeMetadataRecorder struct {
	userv1.UnimplementedUserServiceServer
	accountID string
}

func (s *downgradeMetadataRecorder) ApplyDowngradeProfiles(ctx context.Context, _ *userv1.ApplyDowngradeProfilesRequest) (*userv1.ApplyDowngradeProfilesResponse, error) {
	metadata, ok := metadata.FromIncomingContext(ctx)
	if ok {
		values := metadata.Get("x-voice-user-id")
		if len(values) > 0 {
			s.accountID = values[0]
		}
	}
	return &userv1.ApplyDowngradeProfilesResponse{}, nil
}
