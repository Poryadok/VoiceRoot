package main

import (
	"context"
	"net/http"
	"strings"

	"google.golang.org/grpc/metadata"

	subscriptionv1 "voice.app/voice/subscription/v1"
)

// effectiveSubscriptionTier resolves entitlements from Subscription Service (source of truth).
// Used for File upload caps and User cosmetics (GIF avatar, banner, profile limit).
// When the upstream is unavailable, free-tier limits apply (Tier-2 degradation).
func (t *transcoder) effectiveSubscriptionTier(ctx context.Context, r *http.Request) string {
	accountID := strings.TrimSpace(r.Header.Get("X-Voice-User-Id"))
	if accountID == "" {
		return normalizeSubscriptionTier(r.Header.Get("X-Voice-Subscription-Tier"))
	}
	if t.clients.subscription == nil {
		return "free"
	}
	resp, err := t.clients.subscription.GetSubscription(ctx, &subscriptionv1.GetSubscriptionRequest{
		AccountId: accountID,
	})
	if err != nil || resp == nil || resp.GetSubscription() == nil {
		return "free"
	}
	sub := resp.GetSubscription()
	if sub.GetPlan() == "premium" && (sub.GetStatus() == "active" || sub.GetStatus() == "grace_period") {
		return "premium"
	}
	return "free"
}

// effectiveSubscriptionTierForFiles is kept for call sites/tests; same live entitlement resolve.
func (t *transcoder) effectiveSubscriptionTierForFiles(ctx context.Context, r *http.Request) string {
	return t.effectiveSubscriptionTier(ctx, r)
}

func (t *transcoder) withLiveSubscriptionTierMetadata(ctx context.Context, r *http.Request) context.Context {
	md := grpcMetadataFromRequest(r)
	md.Set("x-voice-subscription-tier", t.effectiveSubscriptionTier(ctx, r))
	return metadata.NewOutgoingContext(ctx, md)
}

func (t *transcoder) withFileGRPCMetadata(ctx context.Context, r *http.Request) context.Context {
	return t.withLiveSubscriptionTierMetadata(ctx, r)
}

func normalizeSubscriptionTier(raw string) string {
	tier := strings.TrimSpace(strings.ToLower(raw))
	if tier == "" {
		return "free"
	}
	return tier
}
