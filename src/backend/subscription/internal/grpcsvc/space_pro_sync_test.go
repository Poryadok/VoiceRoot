package grpcsvc

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"voice/backend/subscription/internal/store"

	spacev1 "voice.app/voice/space/v1"
	subscriptionv1 "voice.app/voice/subscription/v1"
)

type captureSpaceSync struct {
	reqs []*spacev1.SyncSpaceProSubscriptionRequest
}

func (c *captureSpaceSync) SyncSpaceProSubscription(
	_ context.Context,
	req *spacev1.SyncSpaceProSubscriptionRequest,
	_ ...grpc.CallOption,
) (*spacev1.SyncSpaceProSubscriptionResponse, error) {
	cp := protoCloneSyncReq(req)
	c.reqs = append(c.reqs, cp)
	return &spacev1.SyncSpaceProSubscriptionResponse{}, nil
}

func protoCloneSyncReq(req *spacev1.SyncSpaceProSubscriptionRequest) *spacev1.SyncSpaceProSubscriptionRequest {
	if req == nil {
		return nil
	}
	return &spacev1.SyncSpaceProSubscriptionRequest{
		SpaceId:            req.GetSpaceId(),
		PurchaserAccountId: req.GetPurchaserAccountId(),
		Status:             req.GetStatus(),
	}
}

// TestSpaceProWebhook_SyncsActiveEntitlementToSpace documents webhook → S2S SyncSpaceProSubscription(active)
// so Space can raise member cap without SeedSpaceProActive.
func TestSpaceProWebhook_SyncsActiveEntitlementToSpace(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSubscriptionPostgres(t, ctx)
	st := &store.SubscriptionStore{Pool: pool}
	sync := &captureSpaceSync{}
	svc := NewSubscriptionGRPC(st)
	svc.SpaceEntitlements = sync

	spaceID := uuid.New()
	purchaserID := uuid.New()
	body, _ := spaceProActivatedWebhookBody(t, spaceID, purchaserID)
	_, err := svc.HandlePaddleWebhook(ctx, &subscriptionv1.HandlePaddleWebhookRequest{
		RawBody:   body,
		Signature: signedWebhook(t, body),
	})
	require.NoError(t, err)
	require.Len(t, sync.reqs, 1)
	require.Equal(t, spaceID.String(), sync.reqs[0].GetSpaceId())
	require.Equal(t, purchaserID.String(), sync.reqs[0].GetPurchaserAccountId())
	require.Equal(t, "active", sync.reqs[0].GetStatus())
}

// TestSpaceProWebhook_CancelSyncsCancelledWhenPeriodEnded documents cancel → SyncSpaceProSubscription(cancelled)
// so new joins use free member cap while existing members are not kicked.
func TestSpaceProWebhook_CancelSyncsCancelledWhenPeriodEnded(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSubscriptionPostgres(t, ctx)
	st := &store.SubscriptionStore{Pool: pool}
	sync := &captureSpaceSync{}
	svc := NewSubscriptionGRPC(st)
	svc.SpaceEntitlements = sync

	spaceID := uuid.New()
	purchaserID := uuid.New()
	activateBody, _ := spaceProActivatedWebhookBody(t, spaceID, purchaserID)
	_, err := svc.HandlePaddleWebhook(ctx, &subscriptionv1.HandlePaddleWebhookRequest{
		RawBody:   activateBody,
		Signature: signedWebhook(t, activateBody),
	})
	require.NoError(t, err)
	require.Len(t, sync.reqs, 1)

	_, err = pool.Exec(ctx, `
UPDATE space_subscriptions
SET current_period_end = now() - interval '1 hour'
WHERE space_id = $1`, spaceID)
	require.NoError(t, err)

	cancelBody, err := json.Marshal(map[string]any{
		"event_id":   "evt_space_cancel_" + uuid.New().String(),
		"event_type": "subscription.cancelled",
		"data": map[string]any{
			"custom_data": map[string]string{
				"space_id":     spaceID.String(),
				"purchaser_id": purchaserID.String(),
				"plan":         "space_pro",
			},
		},
	})
	require.NoError(t, err)
	_, err = svc.HandlePaddleWebhook(ctx, &subscriptionv1.HandlePaddleWebhookRequest{
		RawBody:   string(cancelBody),
		Signature: signedWebhook(t, string(cancelBody)),
	})
	require.NoError(t, err)
	require.Len(t, sync.reqs, 2)
	require.Equal(t, "cancelled", sync.reqs[1].GetStatus())
	require.Equal(t, spaceID.String(), sync.reqs[1].GetSpaceId())
}
