package s2s

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	userv1 "voice.app/voice/user/v1"
)

// AccountProfiles resolves Auth account_id → User profile_ids.
type AccountProfiles interface {
	ProfileIDsForAccount(ctx context.Context, accountID uuid.UUID) ([]uuid.UUID, error)
}

// GRPCAccountProfiles calls User Service ListProfileIDsForAccount.
type GRPCAccountProfiles struct {
	Client userv1.UserServiceClient
}

// NewGRPCAccountProfiles dials User gRPC at addr.
func NewGRPCAccountProfiles(addr string) (*GRPCAccountProfiles, error) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return nil, fmt.Errorf("account profiles: empty user grpc addr")
	}
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return &GRPCAccountProfiles{Client: userv1.NewUserServiceClient(conn)}, nil
}

// ProfileIDsForAccount returns all profile IDs owned by accountID.
func (c *GRPCAccountProfiles) ProfileIDsForAccount(ctx context.Context, accountID uuid.UUID) ([]uuid.UUID, error) {
	if c == nil || c.Client == nil {
		return nil, nil
	}
	if accountID == uuid.Nil {
		return nil, nil
	}
	ctx = Context(ctx)
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
