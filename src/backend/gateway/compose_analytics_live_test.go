package main

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestComposeAnalytics_live documents staff analytics dashboard after compose stack is up.
func TestComposeAnalytics_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()
	staffToken := composeStaffToken(t, client, base)
	if staffToken == "" {
		t.Skip("no staff token; set GATEWAY_STATIC_TOKENS_JSON")
	}

	req, err := http.NewRequest(http.MethodGet, base+"/api/v1/analytics/dashboard/product", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+staffToken)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	userSess := registerComposeUser(t, client, base, formatComposeEmail("analytics-deny", time.Now().UnixNano()), "VoiceQaTest1!")
	denyReq, err := http.NewRequest(http.MethodGet, base+"/api/v1/analytics/dashboard/product", nil)
	require.NoError(t, err)
	denyReq.Header.Set("Authorization", "Bearer "+userSess.AccessToken)
	denyResp, err := client.Do(denyReq)
	require.NoError(t, err)
	defer denyResp.Body.Close()
	require.Equal(t, http.StatusForbidden, denyResp.StatusCode)
}
