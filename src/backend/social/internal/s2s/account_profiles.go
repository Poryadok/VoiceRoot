package s2s

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/pkg/principal"

	userv1 "voice.app/voice/user/v1"
)

// GRPCAccountProfiles resolves account → profile ids via User Service S2S.
type GRPCAccountProfiles struct {
	Client userv1.UserServiceClient
	Issuer *principal.Issuer
}

func NewGRPCAccountProfiles(client userv1.UserServiceClient, issuer *principal.Issuer) *GRPCAccountProfiles {
	return &GRPCAccountProfiles{Client: client, Issuer: issuer}
}

func (c *GRPCAccountProfiles) ProfileIDsForAccount(ctx context.Context, accountID uuid.UUID) ([]uuid.UUID, error) {
	if c == nil || c.Client == nil {
		return nil, status.Error(codes.Unavailable, "user account profiles client unavailable")
	}
	req := &userv1.ListProfileIDsForAccountRequest{
		AccountId: accountID.String(),
	}
	ctx, err := privacyS2SContext(ctx, c.Issuer, "user", userv1.UserService_ListProfileIDsForAccount_FullMethodName, req)
	if err != nil {
		return nil, err
	}
	resp, err := c.Client.ListProfileIDsForAccount(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, status.Error(codes.Internal, "user account profiles response missing")
	}
	out := make([]uuid.UUID, 0, len(resp.GetProfileIds()))
	for _, raw := range resp.GetProfileIds() {
		id, err := uuid.Parse(strings.TrimSpace(raw))
		if err != nil || id == uuid.Nil {
			return nil, status.Error(codes.Internal, "user account profiles response invalid")
		}
		out = append(out, id)
	}
	return out, nil
}
