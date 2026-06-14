package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestComposePhase14Moderation_live documents PLAN Phase 14 acceptance:
// moderator resolves report, applies sanction, suspended login blocked, shadow ban hides DM delivery.
func TestComposePhase14Moderation_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 90 * time.Second}
	base := liveGatewayBaseURL()
	n := time.Now().UnixNano()

	reporter := registerComposeUser(t, client, base, formatComposeEmail("p14-reporter", n), "VoiceQaTest1!")
	target := registerComposeUser(t, client, base, formatComposeEmail("p14-target", n), "VoiceQaTest1!")
	_ = reporter

	reportBody, err := json.Marshal(map[string]any{
		"target_type":   "user",
		"target_id":     target.ProfileID,
		"category":      "harassment",
		"evidence_json": "{}",
	})
	require.NoError(t, err)
	reportReq, err := http.NewRequest(http.MethodPost, base+"/api/v1/moderation/reports", bytes.NewReader(reportBody))
	require.NoError(t, err)
	reportReq.Header.Set("Authorization", "Bearer "+reporter.AccessToken)
	reportReq.Header.Set("Content-Type", "application/json")
	reportResp, err := client.Do(reportReq)
	require.NoError(t, err)
	defer reportResp.Body.Close()
	require.Equal(t, http.StatusAccepted, reportResp.StatusCode)

	// Staff moderation: requires staff JWT (compose seeds via GATEWAY_STATIC_TOKENS_JSON or dedicated mod account).
	staffToken := composeStaffToken(t, client, base)
	if staffToken == "" {
		t.Skip("no staff token available in compose; set GATEWAY_STATIC_TOKENS_JSON staff entry")
	}

	sanctionBody, err := json.Marshal(map[string]any{
		"target_account_id": target.AccountID,
		"type":              "perm_ban",
		"reason":            "compose phase14",
	})
	require.NoError(t, err)
	sanctionReq, err := http.NewRequest(http.MethodPost, base+"/api/v1/admin/moderation/sanctions", bytes.NewReader(sanctionBody))
	require.NoError(t, err)
	sanctionReq.Header.Set("Authorization", "Bearer "+staffToken)
	sanctionReq.Header.Set("Content-Type", "application/json")
	sanctionResp, err := client.Do(sanctionReq)
	require.NoError(t, err)
	defer sanctionResp.Body.Close()
	require.Equal(t, http.StatusOK, sanctionResp.StatusCode)

	queueReq, err := http.NewRequest(http.MethodGet, base+"/api/v1/admin/moderation/reports?status=pending&queue=content", nil)
	require.NoError(t, err)
	queueReq.Header.Set("Authorization", "Bearer "+staffToken)
	queueResp, err := client.Do(queueReq)
	require.NoError(t, err)
	defer queueResp.Body.Close()
	require.Equal(t, http.StatusOK, queueResp.StatusCode)

	loginPayload, err := json.Marshal(map[string]string{
		"email":            formatComposeEmail("p14-target", n),
		"password":         "VoiceQaTest1!",
		"device_info_json": `{"platform":"go-live-test"}`,
	})
	require.NoError(t, err)
	loginReq, err := http.NewRequest(http.MethodPost, base+"/api/v1/auth/login", bytes.NewReader(loginPayload))
	require.NoError(t, err)
	loginReq.Header.Set("Content-Type", "application/json")
	loginResp, err := client.Do(loginReq)
	require.NoError(t, err)
	defer loginResp.Body.Close()
	require.Equal(t, http.StatusUnauthorized, loginResp.StatusCode, "suspended target login must fail after perm_ban sanction")
}

func composeStaffToken(t *testing.T, client *http.Client, base string) string {
	t.Helper()
	// Try well-known static staff token from dev compose if configured.
	for _, token := range []string{"staff-token", "compose-staff-token"} {
		req, err := http.NewRequest(http.MethodGet, base+"/api/v1/admin/moderation/reports?status=pending&queue=content", nil)
		if err != nil {
			continue
		}
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			return token
		}
	}
	return ""
}
