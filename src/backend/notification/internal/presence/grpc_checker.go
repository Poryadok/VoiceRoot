package presence

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	userv1 "voice.app/voice/user/v1"
)

// GRPCChecker calls User Service GetBulkPresence.
type GRPCChecker struct {
	client userv1.UserServiceClient
}

func NewGRPCChecker(addr string) (*GRPCChecker, error) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return nil, fmt.Errorf("presence: empty user grpc addr")
	}
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return &GRPCChecker{client: userv1.NewUserServiceClient(conn)}, nil
}

func (c *GRPCChecker) IsOnline(ctx context.Context, profileID uuid.UUID) (bool, error) {
	if c == nil || c.client == nil {
		return false, nil
	}
	resp, err := c.client.GetBulkPresence(ctx, &userv1.GetBulkPresenceRequest{
		ProfileIds: []string{profileID.String()},
	})
	if err != nil {
		return false, err
	}
	if resp == nil || resp.GetByProfileId() == nil {
		return false, nil
	}
	st, ok := resp.GetByProfileId()[profileID.String()]
	if !ok || st == nil {
		return false, nil
	}
	if st.GetStatusEnum() == userv1.PresenceOnlineStatus_PRESENCE_ONLINE_STATUS_ONLINE {
		return true, nil
	}
	return strings.EqualFold(st.GetStatus(), "online"), nil
}
