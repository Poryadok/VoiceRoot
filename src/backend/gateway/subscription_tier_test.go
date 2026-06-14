package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	subscriptionv1 "voice.app/voice/subscription/v1"
)

type stubSubscriptionTierBackend struct {
	subscriptionv1.UnimplementedSubscriptionServiceServer
	resp *subscriptionv1.GetSubscriptionResponse
	err  error
}

func (s *stubSubscriptionTierBackend) GetSubscription(_ context.Context, _ *subscriptionv1.GetSubscriptionRequest) (*subscriptionv1.GetSubscriptionResponse, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.resp, nil
}

func TestEffectiveSubscriptionTierForFiles_degradesToFreeWhenUpstreamUnavailable(t *testing.T) {
	client, cleanup := startBufconnSubscriptionClient(t, &stubSubscriptionTierBackend{
		err: status.Error(codes.Unavailable, "subscription down"),
	})
	t.Cleanup(cleanup)

	tr := newTranscoderWithSubscription(client)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/files/upload", nil)
	req.Header.Set("X-Voice-User-Id", "11111111-1111-4111-8111-111111111111")
	req.Header.Set("X-Voice-Subscription-Tier", "premium")

	require.Equal(t, "free", tr.effectiveSubscriptionTierForFiles(req.Context(), req))
}

func TestEffectiveSubscriptionTierForFiles_premiumFromSubscriptionService(t *testing.T) {
	accountID := "22222222-2222-4222-8222-222222222222"
	client, cleanup := startBufconnSubscriptionClient(t, &stubSubscriptionTierBackend{
		resp: &subscriptionv1.GetSubscriptionResponse{
			Subscription: &subscriptionv1.Subscription{
				AccountId: accountID,
				Plan:      "premium",
				Status:    "active",
			},
		},
	})
	t.Cleanup(cleanup)

	tr := newTranscoderWithSubscription(client)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/files/upload", nil)
	req.Header.Set("X-Voice-User-Id", accountID)
	req.Header.Set("X-Voice-Subscription-Tier", "free")

	require.Equal(t, "premium", tr.effectiveSubscriptionTierForFiles(req.Context(), req))
}
