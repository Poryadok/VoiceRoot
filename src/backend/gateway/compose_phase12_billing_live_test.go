package main

import (
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestComposePhase12Billing_live documents PLAN Phase 12 acceptance: webhook premium + upload boundaries.
func TestComposePhase12Billing_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 60 * time.Second}
	base := liveGatewayBaseURL()
	n := time.Now().UnixNano()

	sess := registerComposeUser(t, client, base, formatComposeEmail("p12-billing", n), "VoiceQaTest1!")
	require.NotEmpty(t, sess.AccountID, "register must return account_id for webhook custom_data")

	composeActivatePremiumWebhook(t, client, base, sess.AccountID)
	require.Equal(t, "premium", composeGetSubscriptionPlan(t, client, base, sess.AccessToken))

	if !composeFileUploadAvailable(t, client, base, sess.AccessToken) {
		t.Skip("object storage not configured (MinIO/R2); set FILE_R2_* in .env for compose app profile")
	}

	require.Equal(t, http.StatusOK, composeRequestUploadStatus(t, client, base, sess.AccessToken, composeUpload100MiB))
	require.Equal(t, http.StatusBadRequest, composeRequestUploadStatus(t, client, base, sess.AccessToken, composeUpload250MiB))
}

// TestComposeSubscriptionDowngradeFileLimits_live verifies file uploads use free tier when subscription upstream is absent.
func TestComposeSubscriptionDowngradeFileLimits_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	if os.Getenv("VOICE_SUBSCRIPTION_DEGRADATION_TEST") != "true" {
		t.Skip("set VOICE_SUBSCRIPTION_DEGRADATION_TEST=true with subscription container stopped")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 30 * time.Second}
	base := liveGatewayBaseURL()
	sess := registerComposeUser(t, client, base, formatComposeEmail("p12-deg", time.Now().UnixNano()), "VoiceQaTest1!")

	if !composeFileUploadAvailable(t, client, base, sess.AccessToken) {
		t.Skip("object storage not configured (MinIO/R2); set FILE_R2_* in .env for compose app profile")
	}

	require.Equal(t, http.StatusBadRequest, composeRequestUploadStatus(t, client, base, sess.AccessToken, composeUpload51MiB))
	require.Equal(t, http.StatusOK, composeRequestUploadStatus(t, client, base, sess.AccessToken, 1024))
}
