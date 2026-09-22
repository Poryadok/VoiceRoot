package s2s

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	subscriptionv1 "voice.app/voice/subscription/v1"
)

type subscriptionEntitlementsClientStub struct {
	response *subscriptionv1.GetSubscriptionResponse
	err      error
}

func (s subscriptionEntitlementsClientStub) GetSubscription(context.Context, *subscriptionv1.GetSubscriptionRequest, ...grpc.CallOption) (*subscriptionv1.GetSubscriptionResponse, error) {
	return s.response, s.err
}

func TestSubscriptionEntitlementsPremiumUsesActivePremiumSubscription(t *testing.T) {
	resolver := NewSubscriptionEntitlements(subscriptionEntitlementsClientStub{
		response: &subscriptionv1.GetSubscriptionResponse{Subscription: &subscriptionv1.Subscription{
			Plan: "premium", Status: "active",
		}},
	})

	premium, err := resolver.Premium(context.Background(), uuid.New(), time.Now())
	require.NoError(t, err)
	require.True(t, premium)
}

func TestSubscriptionEntitlementsPremiumFailsClosedForOtherStatesAndErrors(t *testing.T) {
	for _, tc := range []struct {
		name string
		stub subscriptionEntitlementsClientStub
	}{
		{"space pro", subscriptionEntitlementsClientStub{response: &subscriptionv1.GetSubscriptionResponse{Subscription: &subscriptionv1.Subscription{Plan: "space_pro", Status: "active"}}}},
		{"cancelled", subscriptionEntitlementsClientStub{response: &subscriptionv1.GetSubscriptionResponse{Subscription: &subscriptionv1.Subscription{Plan: "premium", Status: "cancelled"}}}},
		{"unavailable", subscriptionEntitlementsClientStub{err: errors.New("subscription unavailable")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			premium, err := NewSubscriptionEntitlements(tc.stub).Premium(context.Background(), uuid.New(), time.Now())
			if tc.stub.err != nil {
				require.ErrorIs(t, err, tc.stub.err)
			}
			require.False(t, premium)
		})
	}
}
