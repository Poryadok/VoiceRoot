package s2s

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	subscriptionv1 "voice.app/voice/subscription/v1"
	spacev1 "voice.app/voice/space/v1"
)

// GRPCSpacePro reports Space Pro entitlement via SubscriptionService.GetSpaceSubscription.
type GRPCSpacePro struct {
	Client subscriptionv1.SubscriptionServiceClient
}

func NewGRPCSpacePro(c subscriptionv1.SubscriptionServiceClient) *GRPCSpacePro {
	return &GRPCSpacePro{Client: c}
}

func (g *GRPCSpacePro) HasSpacePro(ctx context.Context, spaceID string) (bool, error) {
	if g == nil || g.Client == nil {
		return false, status.Error(codes.FailedPrecondition, "subscription service not configured")
	}
	sid, err := uuid.Parse(strings.TrimSpace(spaceID))
	if err != nil {
		return false, status.Error(codes.InvalidArgument, "invalid space id")
	}
	ctx = ForwardIncomingMetadata(ctx)
	resp, err := g.Client.GetSpaceSubscription(ctx, &subscriptionv1.GetSpaceSubscriptionRequest{
		Space: &spacev1.SpaceRef{Id: sid.String()},
	})
	if err != nil {
		return false, err
	}
	sub := resp.GetSpaceSubscription()
	if sub == nil {
		return false, nil
	}
	switch strings.TrimSpace(sub.GetStatus()) {
	case "active", "grace_period", "pending_cancel":
		return true, nil
	default:
		return false, nil
	}
}
