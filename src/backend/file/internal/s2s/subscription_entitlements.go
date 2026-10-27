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

func (s *SubscriptionEntitlements) Premium(ctx context.Context, accountID uuid.UUID, now time.Time) (bool, error) {
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
	// The legacy subscription response carries the entitled-until boundary in
	// the period field corresponding to its status. Never extend it locally.
	deadline := sub.GetCurrentPeriodEnd()
	switch strings.TrimSpace(sub.GetStatus()) {
	case "active":
	case "grace_period":
		deadline = sub.GetGracePeriodEnd()
	default:
		return false, nil
	}
	if deadline == nil || deadline.CheckValid() != nil {
		return false, nil
	}
	return now.Before(deadline.AsTime()), nil
}
