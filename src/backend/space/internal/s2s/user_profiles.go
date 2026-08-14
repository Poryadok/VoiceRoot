package s2s

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc"

	userv1 "voice.app/voice/user/v1"
)

// GRPCUserProfiles resolves profile_id to account_id via User Service.
type GRPCUserProfiles struct {
	Client userv1.UserServiceClient
}

func NewGRPCUserProfiles(cc grpc.ClientConnInterface) *GRPCUserProfiles {
	return &GRPCUserProfiles{Client: userv1.NewUserServiceClient(cc)}
}

func (u *GRPCUserProfiles) AccountIDByProfileID(ctx context.Context, profileID uuid.UUID) (uuid.UUID, error) {
	if u == nil || u.Client == nil {
		return uuid.Nil, nil
	}
	ctx = ForwardIncomingMetadata(ctx)
	resp, err := u.Client.GetProfile(ctx, &userv1.GetProfileRequest{
		By: &userv1.GetProfileRequest_ProfileId{ProfileId: profileID.String()},
	})
	if err != nil {
		return uuid.Nil, err
	}
	return uuid.Parse(resp.GetProfile().GetAccountId())
}
