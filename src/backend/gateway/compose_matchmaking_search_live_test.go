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

// TestComposeMatchmakingSearch_postReturns200 verifies POST /api/v1/matchmaking/search
// succeeds against local compose (regression for missing search_sessions.nudged_at).
func TestComposeMatchmakingSearch_postReturns200(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()
	sess := registerComposeUser(t, client, base, formatComposeEmail("mm-search-post", time.Now().UnixNano()), "VoiceQaTest1!")

	gameID := composeMatchmakingDotaGameID(t, client, base, sess.AccessToken)
	criteriaBytes, _ := json.Marshal(map[string]any{
		"region": "eu",
		"self":   map[string]string{"role": "Carry", "rank": "Herald"},
	})
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
}

// TestComposeMatchmakingSearch_live exercises start → status → cancel on seeded catalog.
func TestComposeMatchmakingSearch_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()

	n := time.Now().UnixNano()
	sess := registerComposeUser(t, client, base, formatComposeEmail("mm-search", n), "VoiceQaTest1!")

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
	require.True(t, ok, "expected gameList in response: %s", string(gamesRaw))
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
			"rank_max": "Guardian",
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
	require.True(t, ok, "expected searchSession in response: %s", string(startRaw))
	sessionID, _ := session["id"].(string)
	require.NotEmpty(t, sessionID)
	require.Equal(t, "searching", session["status"])

	statusReq, err := http.NewRequest(http.MethodGet, base+"/api/v1/matchmaking/search/"+sessionID, nil)
	require.NoError(t, err)
	statusReq.Header.Set("Authorization", "Bearer "+sess.AccessToken)
	statusResp, err := client.Do(statusReq)
	require.NoError(t, err)
	defer statusResp.Body.Close()
	require.Equal(t, http.StatusOK, statusResp.StatusCode)

	cancelReq, err := http.NewRequest(http.MethodDelete, base+"/api/v1/matchmaking/search/"+sessionID, nil)
	require.NoError(t, err)
	cancelReq.Header.Set("Authorization", "Bearer "+sess.AccessToken)
	cancelResp, err := client.Do(cancelReq)
	require.NoError(t, err)
	defer cancelResp.Body.Close()
	require.Equal(t, http.StatusOK, cancelResp.StatusCode)
}

func composeMatchmakingDotaGameID(t *testing.T, client *http.Client, base, accessToken string) string {
	t.Helper()
	gamesReq, err := http.NewRequest(http.MethodGet, base+"/api/v1/matchmaking/games", nil)
	require.NoError(t, err)
	gamesReq.Header.Set("Authorization", "Bearer "+accessToken)
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
	require.True(t, ok, "expected gameList in response: %s", string(gamesRaw))
	games := gameList["games"].([]any)
	for _, g := range games {
		m := g.(map[string]any)
		if m["name"] == "Dota 2" {
			gameID, _ := m["id"].(string)
			require.NotEmpty(t, gameID)
			return gameID
		}
	}
	t.Fatal("Dota 2 not found in matchmaking catalog")
	return ""
}
