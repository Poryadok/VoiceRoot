package main

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestComposeMultiProfileDelete_live documents soft-delete removes profile and blocks switch (docs/features/multi-profile.md).
func TestComposeMultiProfileDelete_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 60 * time.Second}
	base := liveGatewayBaseURL()
	n := time.Now().UnixNano()

	sess := registerComposeUser(t, client, base, formatComposeEmail("mp-delete", n), "VoiceQaTest1!")
	_, altProfileID := composeCreateAltProfile(t, client, base, sess.AccessToken, "Delete Me", "personal")

	require.Equal(t, http.StatusNoContent, composeDeleteProfileStatus(t, client, base, sess.AccessToken, altProfileID))

	ids := composeListProfileIDs(t, client, base, sess.AccessToken)
	require.NotContains(t, ids, altProfileID, "deleted profile must not appear in list")

	status := composeSwitchProfileStatus(t, client, base, sess.AccessToken, altProfileID)
	require.Equal(t, http.StatusPreconditionFailed, status, "switch to deleted profile must fail")
}

// TestComposeMultiProfileDowngrade_live documents downgrade freezes excess profiles and blocks switch.
func TestComposeMultiProfileDowngrade_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 90 * time.Second}
	base := liveGatewayBaseURL()
	n := time.Now().UnixNano()

	sess := registerComposeUser(t, client, base, formatComposeEmail("mp-downgrade", n), "VoiceQaTest1!")
	composeActivatePremiumWebhook(t, client, base, sess.AccountID)

	altToken1, altProfile1 := composeCreateAltProfile(t, client, base, sess.AccessToken, "Alt One", "personal")
	_, altProfile2 := composeCreateAltProfile(t, client, base, altToken1, "Alt Two", "work")

	require.Equal(t, http.StatusOK, composePostDowngradeProfiles(t, client, base, sess.AccessToken, []string{sess.ProfileID}))

	status := composeSwitchProfileStatus(t, client, base, sess.AccessToken, altProfile2)
	require.Equal(t, http.StatusPreconditionFailed, status, "switch to frozen profile must fail after downgrade")

	// Primary profile switch-back still works.
	require.Equal(t, http.StatusOK, composeSwitchProfileStatus(t, client, base, sess.AccessToken, sess.ProfileID))

	_ = altProfile1
}
