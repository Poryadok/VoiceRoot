package main

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestComposeWindowsVersionPolicy_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}

	client := &http.Client{Timeout: 15 * time.Second}
	base := liveGatewayBaseURL()

	versionResp, err := client.Get(base + "/api/v1/version?platform=windows&version=0.0.1")
	require.NoError(t, err)
	defer versionResp.Body.Close()
	require.Equal(t, http.StatusOK, versionResp.StatusCode)

	var versionBody map[string]any
	require.NoError(t, json.NewDecoder(versionResp.Body).Decode(&versionBody))
	require.Equal(t, true, versionBody["force_update"])
	require.NotEmpty(t, versionBody["update_url"])

	blockedReq, err := http.NewRequest(http.MethodGet, base+"/api/v1/users/me", nil)
	require.NoError(t, err)
	blockedReq.Header.Set("X-Voice-Client-Platform", "windows")
	blockedReq.Header.Set("X-Voice-Client-Version", "0.0.1")
	blockedResp, err := client.Do(blockedReq)
	require.NoError(t, err)
	defer blockedResp.Body.Close()
	require.Equal(t, http.StatusUpgradeRequired, blockedResp.StatusCode)
	raw, _ := io.ReadAll(blockedResp.Body)
	var blockedBody map[string]string
	require.NoError(t, json.Unmarshal(raw, &blockedBody))
	require.Equal(t, "client_outdated", blockedBody["error"])
}
