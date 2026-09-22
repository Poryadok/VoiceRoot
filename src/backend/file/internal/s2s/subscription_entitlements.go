package s2s

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"

	subscriptionv1 "voice.app/voice/subscription/v1"
)

type subscriptionEntitlementsClient interface {
	GetSubscription(context.Context, *subscriptionv1.GetSubscriptionRequest, ...grpc.CallOption) (*subscriptionv1.GetSubscriptionResponse, error)
}

// SubscriptionEntitlements obtains File's verified entitlement decision from
// the Subscription service instead of trusting client-controlled metadata.
type SubscriptionEntitlements struct {
	client subscriptionEntitlementsClient
}

func NewSubscriptionEntitlements(client subscriptionEntitlementsClient) *SubscriptionEntitlements {
	return &SubscriptionEntitlements{client: client}
}

func (s *SubscriptionEntitlements) Premium(ctx context.Context, accountID uuid.UUID, _ time.Time) (bool, error) {
	if s == nil || s.client == nil || accountID == uuid.Nil {
		return false, nil
	}
	resp, err := s.client.GetSubscription(ForwardIncomingMetadata(ctx), &subscriptionv1.GetSubscriptionRequest{AccountId: accountID.String()})
	if err != nil {
		return false, err
	}
	sub := resp.GetSubscription()
	if sub == nil || strings.TrimSpace(sub.GetPlan()) != "premium" {
		return false, nil
	}
	switch strings.TrimSpace(sub.GetStatus()) {
	case "active", "grace_period":
		return true, nil
	default:
		return false, nil
	}
}
