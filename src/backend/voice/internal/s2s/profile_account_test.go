package s2s

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	userv1 "voice.app/voice/user/v1"
)

type profileAccountClient struct {
	userv1.UserServiceClient
	profile *userv1.Profile
	request *userv1.GetProfileRequest
	ctx     context.Context
}

func (c *profileAccountClient) GetProfile(ctx context.Context, req *userv1.GetProfileRequest, _ ...grpc.CallOption) (*userv1.GetProfileResponse, error) {
	c.ctx, c.request = ctx, req
	return &userv1.GetProfileResponse{Profile: c.profile}, nil
}

func TestGRPCProfileAccountResolverUsesUserOwnedExactProfile(t *testing.T) {
	profileID, accountID := uuid.New(), uuid.New()
	client := &profileAccountClient{profile: &userv1.Profile{Id: profileID.String(), AccountId: accountID.String()}}
	got, err := (&GRPCProfileAccountResolver{Client: client}).AccountIDByProfileID(context.Background(), profileID)
	require.NoError(t, err)
	require.Equal(t, accountID, got)
	require.Equal(t, profileID.String(), client.request.GetProfileId())
	md, ok := metadata.FromOutgoingContext(client.ctx)
	require.True(t, ok)
	require.Equal(t, []string{"voice"}, md.Get("x-voice-internal-caller"))
}

func TestGRPCProfileAccountResolverRejectsMismatchedOrMissingIdentity(t *testing.T) {
	profileID := uuid.New()
	client := &profileAccountClient{profile: &userv1.Profile{Id: uuid.NewString(), AccountId: uuid.NewString()}}
	_, err := (&GRPCProfileAccountResolver{Client: client}).AccountIDByProfileID(context.Background(), profileID)
	require.Error(t, err)

	client.profile = &userv1.Profile{Id: profileID.String(), AccountId: "not-a-uuid"}
	_, err = (&GRPCProfileAccountResolver{Client: client}).AccountIDByProfileID(context.Background(), profileID)
	require.Error(t, err)
}
