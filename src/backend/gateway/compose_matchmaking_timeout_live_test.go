package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestComposeMatchmakingSearchTimeout_live waits for search timeout status via polling.
// Requires compose with MATCHMAKING_SEARCH_NUDGE_AFTER=10s and MATCHMAKING_SEARCH_TIMEOUT=20s.
func TestComposeMatchmakingSearchTimeout_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	if os.Getenv("MATCHMAKING_LIVE_SHORT_TIMEOUT") != "true" {
		t.Skip("set MATCHMAKING_LIVE_SHORT_TIMEOUT=true and compose MATCHMAKING_SEARCH_TIMEOUT=20s")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()

	n := time.Now().UnixNano()
	sess := registerComposeUser(t, client, base, formatComposeEmail("mm-timeout", n), "VoiceQaTest1!")

	gamesReq, err := http.NewRequest(http.MethodGet, base+"/api/v1/matchmaking/games", nil)
	require.NoError(t, err)
	gamesReq.Header.Set("Authorization", "Bearer "+sess.AccessToken)
	gamesResp, err := client.Do(gamesReq)
	require.NoError(t, err)
	defer gamesResp.Body.Close()
	gamesRaw, _ := io.ReadAll(gamesResp.Body)
	require.Equal(t, http.StatusOK, gamesResp.StatusCode, "body=%s", string(gamesRaw))

	var gamesPayload map[string]any
	require.NoError(t, json.Unmarshal(gamesRaw, &gamesPayload))
	gameList, ok := gamesPayload["gameList"].(map[string]any)
	if !ok {
		gameList, ok = gamesPayload["game_list"].(map[string]any)
	}
	require.True(t, ok)
	games := gameList["games"].([]any)
	var gameID string
	for _, g := range games {
		m := g.(map[string]any)
		if m["name"] == "Dota 2" {
			gameID, _ = m["id"].(string)
			break
		}
	}
	require.NotEmpty(t, gameID)

	criteria := map[string]any{
		"region": "eu",
		"self": map[string]string{
			"role": "Carry",
			"rank": "Herald",
		},
		"sought": map[string]string{
			"rank_min": "Herald",
			"rank_max": "Legend",
		},
	}
	criteriaBytes, _ := json.Marshal(criteria)
	startBody, _ := json.Marshal(map[string]any{
		"gameId":       gameID,
		"mode":         "5v5 Ranked",
		"criteriaJson": string(criteriaBytes),
	})
	startReq, err := http.NewRequest(http.MethodPost, base+"/api/v1/matchmaking/search", bytes.NewReader(startBody))
	require.NoError(t, err)
	startReq.Header.Set("Authorization", "Bearer "+sess.AccessToken)
	startReq.Header.Set("Content-Type", "application/json")
	startResp, err := client.Do(startReq)
	require.NoError(t, err)
	defer startResp.Body.Close()
	startRaw, _ := io.ReadAll(startResp.Body)
	require.Equal(t, http.StatusOK, startResp.StatusCode, "body=%s", string(startRaw))

	var startPayload map[string]any
	require.NoError(t, json.Unmarshal(startRaw, &startPayload))
	session, ok := startPayload["searchSession"].(map[string]any)
	if !ok {
		session, ok = startPayload["search_session"].(map[string]any)
	}
	require.True(t, ok)
	sessionID, _ := session["id"].(string)
	require.NotEmpty(t, sessionID)

	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		statusReq, err := http.NewRequest(http.MethodGet, base+"/api/v1/matchmaking/search/"+sessionID, nil)
		require.NoError(t, err)
		statusReq.Header.Set("Authorization", "Bearer "+sess.AccessToken)
		statusResp, err := client.Do(statusReq)
		require.NoError(t, err)
		statusRaw, _ := io.ReadAll(statusResp.Body)
		statusResp.Body.Close()
		require.Equal(t, http.StatusOK, statusResp.StatusCode, "body=%s", string(statusRaw))

		var statusPayload map[string]any
		require.NoError(t, json.Unmarshal(statusRaw, &statusPayload))
		sessMap, ok := statusPayload["searchSession"].(map[string]any)
		if !ok {
			sessMap, ok = statusPayload["search_session"].(map[string]any)
		}
		require.True(t, ok)
		if status, _ := sessMap["status"].(string); status == "timeout" {
			return
		}
		time.Sleep(2 * time.Second)
	}
	t.Fatal("search session did not reach timeout status within deadline")
}
