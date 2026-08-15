package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestComposeMatchmakingBan_live: peer MM ban persists and blocks matching (MM-06 / matchmaking.md).
func TestComposeMatchmakingBan_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()
	n := time.Now().UnixNano()

	sessA := registerComposeUser(t, client, base, formatComposeEmail("mm-ban-a", n), "VoiceQaTest1!")
	sessB := registerComposeUser(t, client, base, formatComposeEmail("mm-ban-b", n), "VoiceQaTest1!")

	banComposeMMPeer(t, client, base, sessA.AccessToken, sessB.ProfileID)
	require.True(t, getComposeMMBanStatus(t, client, base, sessA.AccessToken, sessB.ProfileID))

	gameID := findComposeMatchmakingGameID(t, client, base, sessA.AccessToken, "MM Duo Live")
	criteria := map[string]any{"region": "eu"}
	criteriaBytes, _ := json.Marshal(criteria)

	sessionA := startComposeMatchmakingSearch(t, client, base, sessA.AccessToken, gameID, "Duo", string(criteriaBytes))
	sessionB := startComposeMatchmakingSearch(t, client, base, sessB.AccessToken, gameID, "Duo", string(criteriaBytes))

	deadline := time.Now().Add(12 * time.Second)
	for time.Now().Before(deadline) {
		statusA := getComposeMatchmakingSearch(t, client, base, sessA.AccessToken, sessionA)
		statusB := getComposeMatchmakingSearch(t, client, base, sessB.AccessToken, sessionB)
		matchA, _ := statusA["matchId"].(string)
		if matchA == "" {
			matchA, _ = statusA["match_id"].(string)
		}
		matchB, _ := statusB["matchId"].(string)
		if matchB == "" {
			matchB, _ = statusB["match_id"].(string)
		}
		require.Empty(t, matchA, "banned peers must not match")
		require.Empty(t, matchB, "banned peers must not match")
		time.Sleep(2 * time.Second)
	}
}

func banComposeMMPeer(t *testing.T, client *http.Client, base, token, targetProfileID string) {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		"targetProfileId": targetProfileID,
		"reason":          "compose mm ban",
	})
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/matchmaking/bans", bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "ban body=%s", string(raw))
}

func getComposeMMBanStatus(t *testing.T, client *http.Client, base, token, targetProfileID string) bool {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, base+"/api/v1/matchmaking/bans/"+targetProfileID, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "ban status body=%s", string(raw))
	var payload map[string]any
	require.NoError(t, json.Unmarshal(raw, &payload))
	status, ok := payload["mmBanStatus"].(map[string]any)
	if !ok {
		status, ok = payload["mm_ban_status"].(map[string]any)
	}
	require.True(t, ok, "expected mmBanStatus: %s", string(raw))
	banned, _ := status["banned"].(bool)
	return banned
}
