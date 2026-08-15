package main

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestComposeAdminAPI_live documents staff moderation + analytics admin REST on live compose gateway.
func TestComposeAdminAPI_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()

	adminBase := liveAdminBaseURL()
	adminHealth, err := client.Get(adminBase + "/health")
	require.NoError(t, err)
	adminHealth.Body.Close()
	if adminHealth.StatusCode != http.StatusOK {
		t.Skipf("admin SPA unavailable at %s (status=%d)", adminBase, adminHealth.StatusCode)
	}

	staffToken := composeStaffToken(t, client, base)
	if staffToken == "" {
		t.Skip("no staff token; set GATEWAY_STATIC_TOKENS_JSON or ADMIN_STAFF_TOKEN")
	}

	queueReq, err := http.NewRequest(http.MethodGet, base+"/api/v1/admin/moderation/reports?status=pending&queue=content", nil)
	require.NoError(t, err)
	queueReq.Header.Set("Authorization", "Bearer "+staffToken)
	queueResp, err := client.Do(queueReq)
	require.NoError(t, err)
	defer queueResp.Body.Close()
	require.Equal(t, http.StatusOK, queueResp.StatusCode)

	dashReq, err := http.NewRequest(http.MethodGet, base+"/api/v1/analytics/dashboard/product", nil)
	require.NoError(t, err)
	dashReq.Header.Set("Authorization", "Bearer "+staffToken)
	dashResp, err := client.Do(dashReq)
	require.NoError(t, err)
	defer dashResp.Body.Close()
	require.Equal(t, http.StatusOK, dashResp.StatusCode)

	auditReq, err := http.NewRequest(http.MethodGet, base+"/api/v1/admin/moderation/audit/export", nil)
	require.NoError(t, err)
	auditReq.Header.Set("Authorization", "Bearer "+staffToken)
	auditResp, err := client.Do(auditReq)
	require.NoError(t, err)
	defer auditResp.Body.Close()
	require.Equal(t, http.StatusOK, auditResp.StatusCode)
}
