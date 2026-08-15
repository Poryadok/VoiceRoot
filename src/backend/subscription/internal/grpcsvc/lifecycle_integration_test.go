package grpcsvc

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/metadata"

	"voice/backend/subscription/internal/store"
	"voice/backend/subscription/internal/sweeper"

	subscriptionv1 "voice.app/voice/subscription/v1"
)

func TestCancelSubscription_setsCancelledAt(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSubscriptionPostgres(t, ctx)
	client, cleanup := startSubscriptionGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	accountID := uuid.New()
	body, _ := premiumActivatedWebhookBody(t, accountID)
	_, err := client.HandlePaddleWebhook(ctx, &subscriptionv1.HandlePaddleWebhookRequest{
		RawBody: body, Signature: signedWebhook(t, body),
	})
	require.NoError(t, err)

	sub, err := client.GetSubscription(ctx, &subscriptionv1.GetSubscriptionRequest{AccountId: accountID.String()})
	require.NoError(t, err)
	subID := sub.GetSubscription().GetId()

	mdCtx := metadata.AppendToOutgoingContext(ctx, "x-voice-user-id", accountID.String())
	resp, err := client.CancelSubscription(mdCtx, &subscriptionv1.CancelSubscriptionRequest{SubscriptionId: subID})
	require.NoError(t, err)
	require.NotNil(t, resp.GetSubscription().GetCancelledAt())
	require.Equal(t, "active", resp.GetSubscription().GetStatus())
}

func TestResumeSubscription_clearsCancelledAt(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSubscriptionPostgres(t, ctx)
	client, cleanup := startSubscriptionGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	accountID := uuid.New()
	body, _ := premiumActivatedWebhookBody(t, accountID)
	_, err := client.HandlePaddleWebhook(ctx, &subscriptionv1.HandlePaddleWebhookRequest{
		RawBody: body, Signature: signedWebhook(t, body),
	})
	require.NoError(t, err)

	sub, err := client.GetSubscription(ctx, &subscriptionv1.GetSubscriptionRequest{AccountId: accountID.String()})
	require.NoError(t, err)
	subID := sub.GetSubscription().GetId()

	mdCtx := metadata.AppendToOutgoingContext(ctx, "x-voice-user-id", accountID.String())
	_, err = client.CancelSubscription(mdCtx, &subscriptionv1.CancelSubscriptionRequest{SubscriptionId: subID})
	require.NoError(t, err)

	resp, err := client.ResumeSubscription(mdCtx, &subscriptionv1.ResumeSubscriptionRequest{SubscriptionId: subID})
	require.NoError(t, err)
	require.Nil(t, resp.GetSubscription().GetCancelledAt())
}

func TestHandlePaddleWebhook_renewedExtendsPeriod(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSubscriptionPostgres(t, ctx)
	client, cleanup := startSubscriptionGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	accountID := uuid.New()
	activateBody, _ := premiumActivatedWebhookBody(t, accountID)
	_, err := client.HandlePaddleWebhook(ctx, &subscriptionv1.HandlePaddleWebhookRequest{
		RawBody: activateBody, Signature: signedWebhook(t, activateBody),
	})
	require.NoError(t, err)

	renewBody := renewedWebhookBody(t, accountID)
	_, err = client.HandlePaddleWebhook(ctx, &subscriptionv1.HandlePaddleWebhookRequest{
		RawBody: renewBody, Signature: signedWebhook(t, renewBody),
	})
	require.NoError(t, err)

	sub, err := client.GetSubscription(ctx, &subscriptionv1.GetSubscriptionRequest{AccountId: accountID.String()})
	require.NoError(t, err)
	require.Equal(t, "active", sub.GetSubscription().GetStatus())
}

func TestGraceSweeper_expiresGracePeriod(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSubscriptionPostgres(t, ctx)
	st := &store.SubscriptionStore{Pool: pool}
	client, cleanup := startSubscriptionGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	accountID := uuid.New()
	activateBody, _ := premiumActivatedWebhookBody(t, accountID)
	_, err := client.HandlePaddleWebhook(ctx, &subscriptionv1.HandlePaddleWebhookRequest{
		RawBody: activateBody, Signature: signedWebhook(t, activateBody),
	})
	require.NoError(t, err)
	failBody := paymentFailedWebhookBody(t, accountID)
	_, err = client.HandlePaddleWebhook(ctx, &subscriptionv1.HandlePaddleWebhookRequest{
		RawBody: failBody, Signature: signedWebhook(t, failBody),
	})
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `UPDATE subscriptions SET grace_period_end = $2 WHERE account_id = $1`, accountID, time.Now().UTC().Add(-time.Hour))
	require.NoError(t, err)

	runner := &sweeper.Runner{Store: st, Now: func() time.Time { return time.Now().UTC() }}
	require.NoError(t, runner.RunOnce(ctx))

	sub, err := client.GetSubscription(ctx, &subscriptionv1.GetSubscriptionRequest{AccountId: accountID.String()})
	require.NoError(t, err)
	require.Equal(t, "cancelled", sub.GetSubscription().GetStatus())
}

func TestGetBillingHistory_returnsWebhookEvents(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSubscriptionPostgres(t, ctx)
	client, cleanup := startSubscriptionGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	accountID := uuid.New()
	body, _ := premiumActivatedWebhookBody(t, accountID)
	_, err := client.HandlePaddleWebhook(ctx, &subscriptionv1.HandlePaddleWebhookRequest{
		RawBody: body, Signature: signedWebhook(t, body),
	})
	require.NoError(t, err)

	resp, err := client.GetBillingHistory(ctx, &subscriptionv1.GetBillingHistoryRequest{AccountId: accountID.String()})
	require.NoError(t, err)
	require.NotEmpty(t, resp.GetBillingHistoryList().GetEvents())
}

func TestCheckLimit_spaceMemberCountRequiresScopeSpace(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSubscriptionPostgres(t, ctx)
	client, cleanup := startSubscriptionGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	_, err := client.CheckLimit(ctx, &subscriptionv1.CheckLimitRequest{
		AccountId: uuid.NewString(),
		LimitName: "space_member_count",
	})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func renewedWebhookBody(t *testing.T, accountID uuid.UUID) string {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		"event_id":   "evt_renew_" + uuid.New().String(),
		"event_type": "subscription.renewed",
		"data": map[string]any{
			"custom_data": map[string]string{
				"account_id": accountID.String(),
				"plan":       "premium",
			},
			"current_billing_period_ends_at": time.Now().UTC().AddDate(0, 1, 0).Format(time.RFC3339),
		},
	})
	require.NoError(t, err)
	return string(body)
}
