package s2s

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	userv1 "voice.app/voice/user/v1"
)

// GRPCAccountProfiles resolves account → profile ids via User Service S2S.
type GRPCAccountProfiles struct {
	Client userv1.UserServiceClient
}

func NewGRPCAccountProfiles(conn *grpc.ClientConn) *GRPCAccountProfiles {
	return &GRPCAccountProfiles{Client: userv1.NewUserServiceClient(conn)}
}

func (c *GRPCAccountProfiles) ProfileIDsForAccount(ctx context.Context, accountID uuid.UUID) ([]uuid.UUID, error) {
	if c == nil || c.Client == nil {
		return nil, nil
	}
	ctx = metadata.AppendToOutgoingContext(ctx, "x-voice-internal-caller", "social")
	resp, err := c.Client.ListProfileIDsForAccount(ctx, &userv1.ListProfileIDsForAccountRequest{
		AccountId: accountID.String(),
	})
	if err != nil {
		return nil, err
	}
	out := make([]uuid.UUID, 0, len(resp.GetProfileIds()))
	for _, raw := range resp.GetProfileIds() {
		id, err := uuid.Parse(strings.TrimSpace(raw))
		if err != nil {
			continue
		}
		out = append(out, id)
	}
	return out, nil
}
