package main

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestComposeAnalyticsExport_live documents staff CSV export and audit path.
func TestComposeAnalyticsExport_live(t *testing.T) {
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

	req, err := http.NewRequest(http.MethodGet, base+"/api/v1/analytics/export?format=csv", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+staffToken)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
}
