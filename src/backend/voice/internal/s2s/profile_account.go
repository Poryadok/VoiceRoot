package s2s

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	userv1 "voice.app/voice/user/v1"
)

// ProfileAccountResolver resolves the durable Auth/User account owning a profile.
type ProfileAccountResolver interface {
	AccountIDByProfileID(context.Context, uuid.UUID) (uuid.UUID, error)
}

// GRPCProfileAccountResolver uses the protected User profile lookup as the
// source of truth; callers must never supply the account identity being bound.
type GRPCProfileAccountResolver struct {
	Client userv1.UserServiceClient
}

func (r *GRPCProfileAccountResolver) AccountIDByProfileID(ctx context.Context, profileID uuid.UUID) (uuid.UUID, error) {
	if r == nil || r.Client == nil || profileID == uuid.Nil {
		return uuid.Nil, status.Error(codes.FailedPrecondition, "user profile resolver not configured")
	}
	req := &userv1.GetProfileRequest{By: &userv1.GetProfileRequest_ProfileId{ProfileId: profileID.String()}}
	resp, err := r.Client.GetProfile(privacyS2SContext(ctx), req)
	if err != nil {
		return uuid.Nil, err
	}
	profile := resp.GetProfile()
	if profile == nil {
		return uuid.Nil, status.Error(codes.NotFound, "profile not found")
	}
	if returnedID, err := uuid.Parse(strings.TrimSpace(profile.GetId())); err != nil || returnedID != profileID {
		return uuid.Nil, status.Error(codes.Internal, "User returned a mismatched profile")
	}
	accountID, err := uuid.Parse(strings.TrimSpace(profile.GetAccountId()))
	if err != nil || accountID == uuid.Nil {
		return uuid.Nil, status.Error(codes.Internal, "User returned an invalid profile account")
	}
	return accountID, nil
}
