package s2s

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	userv1 "voice.app/voice/user/v1"
)

// GRPCUserPresence resolves online/idle profiles via UserService.GetBulkPresence.
type GRPCUserPresence struct {
	Client userv1.UserServiceClient
}

func isOnlinePresence(status string, statusEnum userv1.PresenceOnlineStatus) bool {
	if statusEnum == userv1.PresenceOnlineStatus_PRESENCE_ONLINE_STATUS_ONLINE ||
		statusEnum == userv1.PresenceOnlineStatus_PRESENCE_ONLINE_STATUS_IDLE {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "online", "idle":
		return true
	default:
		return false
	}
}

func (g *GRPCUserPresence) FilterOnlineProfileIDs(ctx context.Context, profileIDs []uuid.UUID) ([]uuid.UUID, error) {
	if g == nil || g.Client == nil {
		return nil, status.Error(codes.FailedPrecondition, "user service not configured")
	}
	if len(profileIDs) == 0 {
		return nil, nil
	}
	ids := make([]string, 0, len(profileIDs))
	for _, pid := range profileIDs {
		ids = append(ids, pid.String())
	}
	ctx = ForwardIncomingMetadata(ctx)
	resp, err := g.Client.GetBulkPresence(ctx, &userv1.GetBulkPresenceRequest{ProfileIds: ids})
	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.Unavailable {
			return nil, status.Error(codes.Unavailable, "user service unavailable")
		}
		return nil, err
	}
	byID := resp.GetByProfileId()
	var online []uuid.UUID
	for _, pid := range profileIDs {
		ps, ok := byID[pid.String()]
		if !ok || ps == nil {
			continue
		}
		if isOnlinePresence(ps.GetStatus(), ps.GetStatusEnum()) {
			online = append(online, pid)
		}
	}
	return online, nil
}
