package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	subscriptionv1 "voice.app/voice/subscription/v1"
	userv1 "voice.app/voice/user/v1"
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

func TestEffectiveSubscriptionTier_userCosmeticsUsesLiveSubscription(t *testing.T) {
	accountID := "33333333-3333-4333-8333-333333333333"
	client, cleanup := startBufconnSubscriptionClient(t, &stubSubscriptionTierBackend{
		resp: &subscriptionv1.GetSubscriptionResponse{
			Subscription: &subscriptionv1.Subscription{
				AccountId: accountID,
				Plan:      "premium",
				Status:    "grace_period",
			},
		},
	})
	t.Cleanup(cleanup)

	tr := newTranscoderWithSubscription(client)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/me/avatar/presigned-upload", nil)
	req.Header.Set("X-Voice-User-Id", accountID)
	req.Header.Set("X-Voice-Subscription-Tier", "free")

	require.Equal(t, "premium", tr.effectiveSubscriptionTier(req.Context(), req))
	mdCtx := tr.withLiveSubscriptionTierMetadata(req.Context(), req)
	md, ok := metadata.FromOutgoingContext(mdCtx)
	require.True(t, ok)
	require.Equal(t, []string{"premium"}, md.Get("x-voice-subscription-tier"))
}

// Both custom-status write routes must replace caller/JWT metadata with the
// live Subscription result. These are route contracts, not helper-only tests:
// a route that forgets the wrapper would otherwise accept a forged tier.
func TestCustomStatusWriteRoutes_UseLiveSubscriptionTier(t *testing.T) {
	accountID := "44444444-4444-4444-8444-444444444444"
	for _, tc := range []struct {
		name string
		path string
		body string
		resp *subscriptionv1.GetSubscriptionResponse
		err  error
		want string
	}{
		{name: "profile unavailable fails closed", path: "/api/v1/users/me", body: `{"custom_status":"x"}`, err: status.Error(codes.Unavailable, "down"), want: "free"},
		{name: "presence cancelled fails closed", path: "/api/v1/users/me/presence", body: `{"status":"online","custom_status":"x"}`, resp: &subscriptionv1.GetSubscriptionResponse{Subscription: &subscriptionv1.Subscription{AccountId: accountID, Plan: "premium", Status: "cancelled"}}, want: "free"},
		{name: "profile grace permits premium", path: "/api/v1/users/me", body: `{"custom_status":"x"}`, resp: &subscriptionv1.GetSubscriptionResponse{Subscription: &subscriptionv1.Subscription{AccountId: accountID, Plan: "premium", Status: "grace_period"}}, want: "premium"},
		{name: "presence active permits premium", path: "/api/v1/users/me/presence", body: `{"status":"online","custom_status":"x"}`, resp: &subscriptionv1.GetSubscriptionResponse{Subscription: &subscriptionv1.Subscription{AccountId: accountID, Plan: "premium", Status: "active"}}, want: "premium"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			subscription, closeSubscription := startBufconnSubscriptionClient(t, &stubSubscriptionTierBackend{resp: tc.resp, err: tc.err})
			t.Cleanup(closeSubscription)
			recorder := &recordingUserGRPC{}
			userConn, closeUser := startBufconnUserConn(t, recorder)
			t.Cleanup(closeUser)
			tr := newTranscoderWithSubscription(subscription)
			tr.clients.user = userv1.NewUserServiceClient(userConn)
			h := newGatewayForContract(t, gatewayTestOptions{tokenClaims: map[string]tokenClaims{
				"forged-premium": {UserID: accountID, ProfileID: "profile-1", SubscriptionTier: "premium"},
			}, transcoder: tr})
			resp := performRequest(h, http.MethodPatch, tc.path, tc.body, map[string]string{"Authorization": "Bearer forged-premium"})
			require.Equal(t, http.StatusOK, resp.Code, "body=%s", resp.Body.String())
			require.Equal(t, []string{tc.want}, recorder.lastMD.Get("x-voice-subscription-tier"))
		})
	}
}
