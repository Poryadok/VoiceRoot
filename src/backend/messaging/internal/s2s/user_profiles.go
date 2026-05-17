package s2s

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	userv1 "voice.app/voice/user/v1"
)

// UserGRPCProfiles resolves profile_id → account_id via UserService.GetProfile.
type UserGRPCProfiles struct {
	Client userv1.UserServiceClient
}

func (u *UserGRPCProfiles) AccountIDByProfileID(ctx context.Context, profileID uuid.UUID) (uuid.UUID, error) {
	if u == nil || u.Client == nil {
		return uuid.Nil, status.Error(codes.FailedPrecondition, "user service not configured")
	}
	ctx = ForwardIncomingMetadata(ctx)
	resp, err := u.Client.GetProfile(ctx, &userv1.GetProfileRequest{
		By: &userv1.GetProfileRequest_ProfileId{ProfileId: profileID.String()},
	})
	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.NotFound {
			return uuid.Nil, status.Error(codes.NotFound, "profile not found")
		}
		return uuid.Nil, err
	}
	p := resp.GetProfile()
	if p == nil {
		return uuid.Nil, status.Error(codes.NotFound, "profile not found")
	}
	aid := strings.TrimSpace(p.GetAccountId())
	if aid == "" {
		return uuid.Nil, status.Error(codes.Internal, "profile missing account_id")
	}
	out, err := uuid.Parse(aid)
	if err != nil {
		return uuid.Nil, status.Error(codes.Internal, "invalid account_id on profile")
	}
	return out, nil
}
