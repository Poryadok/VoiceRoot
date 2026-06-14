package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	subscriptionv1 "voice.app/voice/subscription/v1"
	spacev1 "voice.app/voice/space/v1"
)

func TestGetSubscription_emptyWhenNoRow(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSubscriptionPostgres(t, ctx)
	client, cleanup := startSubscriptionGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	resp, err := client.GetSubscription(ctx, &subscriptionv1.GetSubscriptionRequest{
		AccountId: uuid.New().String(),
	})
	require.NoError(t, err)
	require.Nil(t, resp.GetSubscription())
}

func TestCreateCheckoutSession_knownPlan(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSubscriptionPostgres(t, ctx)
	client, cleanup := startSubscriptionGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	resp, err := client.CreateCheckoutSession(ctx, &subscriptionv1.CreateCheckoutSessionRequest{
		Plan:          "premium",
		BillingPeriod: "monthly",
		SuccessUrl:    "https://voice.test/success",
	})
	require.NoError(t, err)
	require.NotEmpty(t, resp.GetCheckoutResponse().GetCheckoutUrl())
	require.NotEmpty(t, resp.GetCheckoutResponse().GetSessionId())
}

func TestCreateCheckoutSession_unknownPlanRejected(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSubscriptionPostgres(t, ctx)
	client, cleanup := startSubscriptionGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	_, err := client.CreateCheckoutSession(ctx, &subscriptionv1.CreateCheckoutSessionRequest{
		Plan:          "enterprise",
		BillingPeriod: "monthly",
	})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestCancelSubscription_unimplemented(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSubscriptionPostgres(t, ctx)
	client, cleanup := startSubscriptionGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	_, err := client.CancelSubscription(ctx, &subscriptionv1.CancelSubscriptionRequest{
		SubscriptionId: uuid.NewString(),
	})
	require.Error(t, err)
	require.Equal(t, codes.Unimplemented, status.Code(err))
}

func TestGetSpaceSubscription_emptyWhenMissing(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSubscriptionPostgres(t, ctx)
	client, cleanup := startSubscriptionGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	resp, err := client.GetSpaceSubscription(ctx, &subscriptionv1.GetSpaceSubscriptionRequest{
		Space: &spacev1.SpaceRef{Id: uuid.NewString()},
	})
	require.NoError(t, err)
	require.Nil(t, resp.GetSpaceSubscription())
}

func TestCheckLimit_unknownLimitRejected(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSubscriptionPostgres(t, ctx)
	client, cleanup := startSubscriptionGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	_, err := client.CheckLimit(ctx, &subscriptionv1.CheckLimitRequest{
		AccountId: uuid.NewString(),
		LimitName: "unknown_limit",
	})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestHandlePaddleWebhook_unknownPlanRejected(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSubscriptionPostgres(t, ctx)
	client, cleanup := startSubscriptionGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	body := `{"event_id":"evt_bad_plan","event_type":"subscription.activated","data":{"custom_data":{"plan":"unknown"}}}`
	_, err := client.HandlePaddleWebhook(ctx, &subscriptionv1.HandlePaddleWebhookRequest{
		RawBody:   body,
		Signature: signedWebhook(t, body),
	})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestResumeSubscription_unimplemented(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSubscriptionPostgres(t, ctx)
	client, cleanup := startSubscriptionGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	_, err := client.ResumeSubscription(ctx, &subscriptionv1.ResumeSubscriptionRequest{
		SubscriptionId: uuid.NewString(),
	})
	require.Error(t, err)
	require.Equal(t, codes.Unimplemented, status.Code(err))
}

func TestCreateSpaceCheckoutSession_returnsURL(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSubscriptionPostgres(t, ctx)
	client, cleanup := startSubscriptionGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	resp, err := client.CreateSpaceCheckoutSession(ctx, &subscriptionv1.CreateSpaceCheckoutSessionRequest{
		Space:      &spacev1.SpaceRef{Id: uuid.NewString()},
		SuccessUrl: "https://voice.test/space-success",
	})
	require.NoError(t, err)
	require.Contains(t, resp.GetCheckoutResponse().GetCheckoutUrl(), "https://voice.test/space-success")
}

func TestGetBillingHistory_emptyList(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSubscriptionPostgres(t, ctx)
	client, cleanup := startSubscriptionGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	resp, err := client.GetBillingHistory(ctx, &subscriptionv1.GetBillingHistoryRequest{
		AccountId: uuid.NewString(),
	})
	require.NoError(t, err)
	require.NotNil(t, resp.GetBillingHistoryList())
}

func TestHandleCloudPaymentsWebhook_unimplemented(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSubscriptionPostgres(t, ctx)
	client, cleanup := startSubscriptionGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	_, err := client.HandleCloudPaymentsWebhook(ctx, &subscriptionv1.HandleCloudPaymentsWebhookRequest{})
	require.Error(t, err)
	require.Equal(t, codes.Unimplemented, status.Code(err))
}

func TestHandlePaddleWebhook_paymentFailedNotFound(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSubscriptionPostgres(t, ctx)
	client, cleanup := startSubscriptionGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	body := paymentFailedWebhookBody(t, uuid.New())
	_, err := client.HandlePaddleWebhook(ctx, &subscriptionv1.HandlePaddleWebhookRequest{
		RawBody:   body,
		Signature: signedWebhook(t, body),
	})
	require.Error(t, err)
	require.Equal(t, codes.NotFound, status.Code(err))
}

func TestGetSpaceSubscription_returnsActiveSpacePro(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSubscriptionPostgres(t, ctx)
	client, cleanup := startSubscriptionGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	spaceID := uuid.New()
	purchaserID := uuid.New()
	body, _ := spaceProActivatedWebhookBody(t, spaceID, purchaserID)
	_, err := client.HandlePaddleWebhook(ctx, &subscriptionv1.HandlePaddleWebhookRequest{
		RawBody:   body,
		Signature: signedWebhook(t, body),
	})
	require.NoError(t, err)

	resp, err := client.GetSpaceSubscription(ctx, &subscriptionv1.GetSpaceSubscriptionRequest{
		Space: &spacev1.SpaceRef{Id: spaceID.String()},
	})
	require.NoError(t, err)
	require.Equal(t, "space_pro", resp.GetSpaceSubscription().GetPlan())
	require.Equal(t, purchaserID.String(), resp.GetSpaceSubscription().GetPurchaserAccountId())
}

func TestHandlePaddleWebhook_unknownEventTypeIsNoOp(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSubscriptionPostgres(t, ctx)
	client, cleanup := startSubscriptionGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	body := `{"event_id":"evt_unknown","event_type":"customer.created","data":{}}`
	_, err := client.HandlePaddleWebhook(ctx, &subscriptionv1.HandlePaddleWebhookRequest{
		RawBody:   body,
		Signature: signedWebhook(t, body),
	})
	require.NoError(t, err)
}

func TestGetSubscription_invalidAccountRejected(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSubscriptionPostgres(t, ctx)
	client, cleanup := startSubscriptionGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	_, err := client.GetSubscription(ctx, &subscriptionv1.GetSubscriptionRequest{AccountId: "not-a-uuid"})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestGetLimits_freeTierDefaults(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSubscriptionPostgres(t, ctx)
	client, cleanup := startSubscriptionGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	limits, err := client.GetLimits(ctx, &subscriptionv1.GetLimitsRequest{AccountId: uuid.NewString()})
	require.NoError(t, err)
	require.Contains(t, limits.GetLimits().GetLimitsJson(), "file_upload_bytes")
}

func TestCreateSpaceCheckoutSession_requiresSpace(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSubscriptionPostgres(t, ctx)
	client, cleanup := startSubscriptionGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	_, err := client.CreateSpaceCheckoutSession(ctx, &subscriptionv1.CreateSpaceCheckoutSessionRequest{})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestHandlePaddleWebhook_spaceProMissingCustomDataRejected(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSubscriptionPostgres(t, ctx)
	client, cleanup := startSubscriptionGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	body := `{"event_id":"evt_space_bad","event_type":"subscription.activated","data":{"custom_data":{"plan":"space_pro"}}}`
	_, err := client.HandlePaddleWebhook(ctx, &subscriptionv1.HandlePaddleWebhookRequest{
		RawBody:   body,
		Signature: signedWebhook(t, body),
	})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}
