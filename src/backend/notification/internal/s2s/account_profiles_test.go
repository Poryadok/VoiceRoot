package s2s_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"voice/backend/notification/internal/s2s"
	userv1 "voice.app/voice/user/v1"
)

type stubUserProfilesClient struct {
	userv1.UserServiceClient
	accountID  string
	profileIDs []string
	lastMD     metadata.MD
}

func (s *stubUserProfilesClient) ListProfileIDsForAccount(
	ctx context.Context,
	in *userv1.ListProfileIDsForAccountRequest,
	_ ...grpc.CallOption,
) (*userv1.ListProfileIDsForAccountResponse, error) {
	s.accountID = in.GetAccountId()
	if md, ok := metadata.FromOutgoingContext(ctx); ok {
		s.lastMD = md
	}
	return &userv1.ListProfileIDsForAccountResponse{ProfileIds: s.profileIDs}, nil
}

func TestGRPCAccountProfiles_ProfileIDsForAccount(t *testing.T) {
	accountID := uuid.New()
	profileID := uuid.New()
	stub := &stubUserProfilesClient{profileIDs: []string{profileID.String(), "not-a-uuid"}}
	resolver := &s2s.GRPCAccountProfiles{Client: stub}

	got, err := resolver.ProfileIDsForAccount(context.Background(), accountID)
	require.NoError(t, err)
	require.Equal(t, accountID.String(), stub.accountID)
	require.Equal(t, []string{"notification"}, stub.lastMD.Get("x-voice-internal-caller"))
	require.Equal(t, []uuid.UUID{profileID}, got)
}

func TestGRPCAccountProfiles_NilClient(t *testing.T) {
	resolver := &s2s.GRPCAccountProfiles{}
	got, err := resolver.ProfileIDsForAccount(context.Background(), uuid.New())
	require.NoError(t, err)
	require.Nil(t, got)
}
