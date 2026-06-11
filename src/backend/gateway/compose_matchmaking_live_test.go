package main

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestComposeMatchmakingListGames_live asserts Gateway exposes GET /api/v1/matchmaking/games
// with seeded Dota 2 catalog including roles/ranks in config_json.
func TestComposeMatchmakingListGames_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()

	n := time.Now().UnixNano()
	sess := registerComposeUser(t, client, base, formatComposeEmail("mm-catalog", n), "VoiceQaTest1!")

	req, err := http.NewRequest(http.MethodGet, base+"/api/v1/matchmaking/games", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+sess.AccessToken)

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "body=%s", string(raw))

	var payload map[string]any
	require.NoError(t, json.Unmarshal(raw, &payload))
	gameList, ok := payload["gameList"].(map[string]any)
	if !ok {
		gameList, ok = payload["game_list"].(map[string]any)
	}
	require.True(t, ok, "expected gameList in response: %s", string(raw))
	games, ok := gameList["games"].([]any)
	require.True(t, ok)
	require.NotEmpty(t, games)

	var dota map[string]any
	for _, g := range games {
		m, _ := g.(map[string]any)
		if m["name"] == "Dota 2" {
			dota = m
			break
		}
	}
	require.NotNil(t, dota, "seeded Dota 2 must be in catalog")
	configJSON, _ := dota["configJson"].(string)
	if configJSON == "" {
		configJSON, _ = dota["config_json"].(string)
	}
	require.Contains(t, configJSON, "Carry")
	require.Contains(t, configJSON, "Herald")
}
