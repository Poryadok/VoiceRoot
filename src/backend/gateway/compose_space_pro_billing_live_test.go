package main

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestComposeSpaceProBilling_live documents Space Pro webhook activation and tree cap lift (docs/features/subscription.md).
func TestComposeSpaceProBilling_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 120 * time.Second}
	base := liveGatewayBaseURL()
	n := time.Now().UnixNano()

	sess := registerComposeUser(t, client, base, formatComposeEmail("space-pro-owner", n), "VoiceQaTest1!")
	spaceID := createComposeSpace(t, client, base, sess.AccessToken, "Space Pro QA", "billing e2e")

	for i := 0; i < 49; i++ {
		status := createComposeSpaceChatStatus(t, client, base, sess.AccessToken, spaceID, fmt.Sprintf("node-%d", i))
		require.Equal(t, http.StatusOK, status, "tree node %d must succeed before cap", i)
	}
	require.Equal(t, http.StatusOK, createComposeSpaceChatStatus(t, client, base, sess.AccessToken, spaceID, "node-49"))
	require.NotEqual(t, http.StatusOK, createComposeSpaceChatStatus(t, client, base, sess.AccessToken, spaceID, "node-50-cap"),
		"51st tree node must fail on free tier")

	composeActivateSpaceProWebhook(t, client, base, spaceID, sess.AccountID)

	row := composeQueryPostgres(t, "subscription_db",
		fmt.Sprintf("SELECT plan,status FROM space_subscriptions WHERE space_id = '%s'", spaceID))
	require.Contains(t, row, "space_pro")
	require.Contains(t, row, "active")

	require.Equal(t, http.StatusOK, createComposeSpaceChatStatus(t, client, base, sess.AccessToken, spaceID, "node-50-pro"),
		"tree node beyond free cap must succeed after Space Pro webhook")
}
