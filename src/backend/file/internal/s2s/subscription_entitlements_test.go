package s2s

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"

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
	now := time.Date(2026, time.October, 25, 12, 0, 0, 0, time.UTC)
	resolver := NewSubscriptionEntitlements(subscriptionEntitlementsClientStub{
		response: &subscriptionv1.GetSubscriptionResponse{Subscription: &subscriptionv1.Subscription{
			Plan: "premium", Status: "active", CurrentPeriodEnd: timestamppb.New(now.Add(time.Hour)),
		}},
	})

	premium, err := resolver.Premium(context.Background(), uuid.New(), now)
	require.NoError(t, err)
	require.True(t, premium)
}

func TestSubscriptionEntitlementsPremiumExpiresAtStatusDeadline(t *testing.T) {
	deadline := time.Date(2026, time.October, 25, 12, 0, 0, 0, time.UTC)
	for _, status := range []string{"active", "grace_period"} {
		for _, tc := range []struct {
			name string
			now  time.Time
			want bool
		}{
			{"before", deadline.Add(-time.Nanosecond), true},
			{"equal", deadline, false},
			{"after", deadline.Add(time.Nanosecond), false},
		} {
			t.Run(status+"/"+tc.name, func(t *testing.T) {
				sub := &subscriptionv1.Subscription{Plan: "premium", Status: status}
				if status == "active" {
					sub.CurrentPeriodEnd = timestamppb.New(deadline)
					// A future grace deadline cannot extend an active period.
					sub.GracePeriodEnd = timestamppb.New(deadline.Add(time.Hour))
				} else {
					sub.CurrentPeriodEnd = timestamppb.New(deadline.Add(-time.Hour))
					sub.GracePeriodEnd = timestamppb.New(deadline)
				}
				resolver := NewSubscriptionEntitlements(subscriptionEntitlementsClientStub{
					response: &subscriptionv1.GetSubscriptionResponse{Subscription: sub},
				})
				premium, err := resolver.Premium(context.Background(), uuid.New(), tc.now)
				require.NoError(t, err)
				require.Equal(t, tc.want, premium)
			})
		}
	}
}

func TestSubscriptionEntitlementsPremiumRejectsMissingOrInvalidStatusDeadline(t *testing.T) {
	now := time.Date(2026, time.October, 25, 12, 0, 0, 0, time.UTC)
	for _, status := range []string{"active", "grace_period"} {
		for _, tc := range []struct {
			name     string
			deadline *timestamppb.Timestamp
		}{
			{"missing", nil},
			{"invalid seconds", &timestamppb.Timestamp{Seconds: 253402300800}},
			{"invalid nanos", &timestamppb.Timestamp{Seconds: now.Unix() + 3600, Nanos: 1000000000}},
		} {
			t.Run(status+"/"+tc.name, func(t *testing.T) {
				future := timestamppb.New(now.Add(time.Hour))
				sub := &subscriptionv1.Subscription{Plan: "premium", Status: status, CurrentPeriodEnd: future, GracePeriodEnd: future}
				if status == "active" {
					sub.CurrentPeriodEnd = tc.deadline
				} else {
					sub.GracePeriodEnd = tc.deadline
				}
				resolver := NewSubscriptionEntitlements(subscriptionEntitlementsClientStub{
					response: &subscriptionv1.GetSubscriptionResponse{Subscription: sub},
				})
				premium, err := resolver.Premium(context.Background(), uuid.New(), now)
				require.NoError(t, err)
				require.False(t, premium)
			})
		}
	}
}

func TestSubscriptionEntitlementsPremiumFailsClosedForOtherStatesAndErrors(t *testing.T) {
	future := timestamppb.New(time.Now().Add(time.Hour))
	for _, tc := range []struct {
		name string
		stub subscriptionEntitlementsClientStub
	}{
		{"space pro", subscriptionEntitlementsClientStub{response: &subscriptionv1.GetSubscriptionResponse{Subscription: &subscriptionv1.Subscription{Plan: "space_pro", Status: "active", CurrentPeriodEnd: future}}}},
		{"cancelled", subscriptionEntitlementsClientStub{response: &subscriptionv1.GetSubscriptionResponse{Subscription: &subscriptionv1.Subscription{Plan: "premium", Status: "cancelled", CurrentPeriodEnd: future, GracePeriodEnd: future}}}},
		{"inactive", subscriptionEntitlementsClientStub{response: &subscriptionv1.GetSubscriptionResponse{Subscription: &subscriptionv1.Subscription{Plan: "premium", Status: "inactive", CurrentPeriodEnd: future, GracePeriodEnd: future}}}},
		{"missing response", subscriptionEntitlementsClientStub{}},
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
