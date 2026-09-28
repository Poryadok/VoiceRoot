package s2s

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/pkg/principal"
	"voice/backend/social/internal/store"

	userv1 "voice.app/voice/user/v1"
)

// GRPCProfileAccounts resolves profile_id → account_id via User Service S2S.
type GRPCProfileAccounts struct {
	Client userv1.UserServiceClient
	Issuer *principal.Issuer
}

func NewGRPCProfileAccounts(client userv1.UserServiceClient, issuer *principal.Issuer) *GRPCProfileAccounts {
	return &GRPCProfileAccounts{Client: client, Issuer: issuer}
}

// Profile fetches the exact public profile through the protected User RPC.
func (c *GRPCProfileAccounts) Profile(ctx context.Context, profileID uuid.UUID) (store.BlockedProfile, error) {
	if c == nil || c.Client == nil {
		return store.BlockedProfile{}, status.Error(codes.FailedPrecondition, "user service not configured")
	}
	req := &userv1.GetProfileRequest{By: &userv1.GetProfileRequest_ProfileId{ProfileId: profileID.String()}}
	ctx, err := privacyS2SContext(ctx, c.Issuer, "user", userv1.UserService_GetProfile_FullMethodName, req)
	if err != nil {
		return store.BlockedProfile{}, err
	}
	resp, err := c.Client.GetProfile(ctx, req)
	if err != nil {
		return store.BlockedProfile{}, err
	}
	p := resp.GetProfile()
	if p == nil {
		return store.BlockedProfile{}, status.Error(codes.NotFound, "profile not found")
	}
	returnedID, err := uuid.Parse(strings.TrimSpace(p.GetId()))
	if err != nil || returnedID != profileID {
		return store.BlockedProfile{}, status.Error(codes.Internal, "invalid profile_id on profile")
	}
	accountID, err := uuid.Parse(strings.TrimSpace(p.GetAccountId()))
	if err != nil || accountID == uuid.Nil {
		return store.BlockedProfile{}, status.Error(codes.Internal, "invalid account_id on profile")
	}
	return store.BlockedProfile{
		AccountID:     accountID,
		DisplayName:   p.GetDisplayName(),
		Username:      p.GetUsername(),
		Discriminator: p.GetDiscriminator(),
	}, nil
}

func (c *GRPCProfileAccounts) AccountIDByProfileID(ctx context.Context, profileID uuid.UUID) (uuid.UUID, error) {
	if c == nil || c.Client == nil {
		return uuid.Nil, status.Error(codes.FailedPrecondition, "user service not configured")
	}
	req := &userv1.GetProfileRequest{
		By: &userv1.GetProfileRequest_ProfileId{ProfileId: profileID.String()},
	}
	ctx, err := privacyS2SContext(ctx, c.Issuer, "user", userv1.UserService_GetProfile_FullMethodName, req)
	if err != nil {
		return uuid.Nil, err
	}
	resp, err := c.Client.GetProfile(ctx, req)
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
