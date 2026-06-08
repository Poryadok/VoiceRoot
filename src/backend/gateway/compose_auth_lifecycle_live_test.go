package main

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestComposeAuthLifecycle_live: refresh rotates access token; logout blacklists JWT on protected routes.
func TestComposeAuthLifecycle_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()

	n := time.Now().UnixNano()
	sess := registerComposeUser(t, client, base, formatComposeEmail("auth-life", n), "VoiceQaTest1!")
	require.Equal(t, http.StatusOK, composeProtectedRouteStatus(t, client, base, sess.AccessToken))

	oldAccess := sess.AccessToken
	sess = refreshComposeSession(t, client, base, sess.RefreshToken)
	require.NotEqual(t, oldAccess, sess.AccessToken)
	require.Equal(t, http.StatusOK, composeProtectedRouteStatus(t, client, base, sess.AccessToken))

	logoutComposeSession(t, client, base, sess.AccessToken, sess.RefreshToken)
	status := composeProtectedRouteStatus(t, client, base, sess.AccessToken)
	require.Equal(t, http.StatusUnauthorized, status, "logged-out access token must be rejected by gateway blacklist/JWKS")
}
