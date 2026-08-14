package subscriptionconsume

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	eventsv1 "voice.app/voice/events/v1"
)

func TestApplySubscriptionEvent_PlanStartedPremium(t *testing.T) {
	cache := NewTierCache()
	ApplySubscriptionEvent(cache, &eventsv1.SubscriptionStreamEvent{
		OccurredAt: timestamppb.Now(),
		Payload: &eventsv1.SubscriptionStreamEvent_PlanStarted{
			PlanStarted: &eventsv1.PlanStarted{
				AccountId: "acc-1",
				Plan:      "premium",
			},
		},
	})
	require.Equal(t, "premium", cache.Tier("acc-1"))
}

func TestApplySubscriptionEvent_PlanCancelledResetsFree(t *testing.T) {
	cache := NewTierCache()
	cache.SetTier("acc-1", "premium")
	ApplySubscriptionEvent(cache, &eventsv1.SubscriptionStreamEvent{
		Payload: &eventsv1.SubscriptionStreamEvent_PlanCancelled{
			PlanCancelled: &eventsv1.PlanCancelled{AccountId: "acc-1", Plan: "premium"},
		},
	})
	require.Equal(t, "free", cache.Tier("acc-1"))
}
