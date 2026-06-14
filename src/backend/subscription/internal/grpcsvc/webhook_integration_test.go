package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/subscription/internal/testfixtures"

	subscriptionv1 "voice.app/voice/subscription/v1"
)

// TestHandlePaddleWebhook_IdempotentDuplicateProviderEvent documents duplicate provider_event_id is no-op.
func TestHandlePaddleWebhook_IdempotentDuplicateProviderEvent(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSubscriptionPostgres(t, ctx)
	client, cleanup := startSubscriptionGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	accountID := uuid.New()
	body, eventID := premiumActivatedWebhookBody(t, accountID)
	sig := signedWebhook(t, body)

	req := &subscriptionv1.HandlePaddleWebhookRequest{RawBody: body, Signature: sig}
	_, err := client.HandlePaddleWebhook(ctx, req)
	require.NoError(t, err)

	_, err = client.HandlePaddleWebhook(ctx, req)
	require.NoError(t, err)

	var count int
	err = pool.QueryRow(ctx, `
SELECT COUNT(*) FROM billing_events WHERE provider = 'paddle' AND provider_event_id = $1
`, eventID).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

// TestHandlePaddleWebhook_InvalidSignatureRejected documents bad webhook signatures are rejected.
func TestHandlePaddleWebhook_InvalidSignatureRejected(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSubscriptionPostgres(t, ctx)
	client, cleanup := startSubscriptionGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	body, _ := premiumActivatedWebhookBody(t, uuid.New())
	_, err := client.HandlePaddleWebhook(ctx, &subscriptionv1.HandlePaddleWebhookRequest{
		RawBody:   body,
		Signature: "ts=1,h1=deadbeef",
	})
	require.Error(t, err)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
}

// TestPremiumWebhook_GetLimits200MB5Profiles documents Premium activation exposes entitlement limits.
func TestPremiumWebhook_GetLimits200MB5Profiles(t *testing.T) {
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
		RawBody:   body,
		Signature: signedWebhook(t, body),
	})
	require.NoError(t, err)

	limits, err := client.GetLimits(ctx, &subscriptionv1.GetLimitsRequest{AccountId: accountID.String()})
	require.NoError(t, err)
	require.NotNil(t, limits.GetLimits())
	json := limits.GetLimits().GetLimitsJson()
	require.Equal(t, testfixtures.FileUploadBytesPremium, limitsJSONInt(t, json, "file_upload_bytes"))
	require.EqualValues(t, testfixtures.ProfileCountPremium, limitsJSONInt(t, json, "profile_count"))
}

// TestGracePeriod_FailedPaymentKeepsPremiumLimits7Days documents grace_period retains premium limits.
func TestGracePeriod_FailedPaymentKeepsPremiumLimits7Days(t *testing.T) {
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
		RawBody:   activateBody,
		Signature: signedWebhook(t, activateBody),
	})
	require.NoError(t, err)

	failBody := paymentFailedWebhookBody(t, accountID)
	_, err = client.HandlePaddleWebhook(ctx, &subscriptionv1.HandlePaddleWebhookRequest{
		RawBody:   failBody,
		Signature: signedWebhook(t, failBody),
	})
	require.NoError(t, err)

	sub, err := client.GetSubscription(ctx, &subscriptionv1.GetSubscriptionRequest{AccountId: accountID.String()})
	require.NoError(t, err)
	require.Equal(t, "grace_period", sub.GetSubscription().GetStatus())
	require.NotNil(t, sub.GetSubscription().GetGracePeriodEnd())

	limits, err := client.GetLimits(ctx, &subscriptionv1.GetLimitsRequest{AccountId: accountID.String()})
	require.NoError(t, err)
	json := limits.GetLimits().GetLimitsJson()
	require.Equal(t, testfixtures.FileUploadBytesPremium, limitsJSONInt(t, json, "file_upload_bytes"))
	require.EqualValues(t, testfixtures.ProfileCountPremium, limitsJSONInt(t, json, "profile_count"))
}

// TestSpaceProWebhook_CheckLimitMemberCap5000 documents Space Pro raises member cap.
func TestSpaceProWebhook_CheckLimitMemberCap5000(t *testing.T) {
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

	resp, err := client.CheckLimit(ctx, &subscriptionv1.CheckLimitRequest{
		AccountId: purchaserID.String(),
		LimitName: "space_member_count",
		Delta:     1,
	})
	require.NoError(t, err)
	require.True(t, resp.GetAllowed())
	require.EqualValues(t, testfixtures.SpaceMemberCountSpacePro, resp.GetRemaining())
}
