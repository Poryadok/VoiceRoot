package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestComposeGameRequestModeration_live documents GC-02/GC-03:
// user SubmitGameRequest → pending_moderation; staff approve → active catalog.
func TestComposeGameRequestModeration_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()
	n := time.Now().UnixNano()

	user := registerComposeUser(t, client, base, formatComposeEmail("gc02-user", n), "VoiceQaTest1!")
	name := fmt.Sprintf("Compose Game %d", n)
	configJSON := `{"regions":["eu"],"modes":[{"name":"5v5","slots":10,"party_size_min":1,"party_size_max":5,"roles":[{"name":"Carry","required":true}],"ranks":[{"name":"Bronze","value":0}]}]}`

	submitted := composeSubmitGameRequest(t, client, base, user.AccessToken, name, configJSON)
	gameID := stringFromAny(submitted["id"])
	require.NotEmpty(t, gameID)
	status := stringFromAny(submitted["status"])
	require.Equal(t, "pending_moderation", status)

	// Pending games must not appear in public catalog.
	catalog := composeListMatchmakingGames(t, client, base, user.AccessToken)
	for _, g := range catalog {
		require.NotEqual(t, name, stringFromAny(g["name"]))
	}

	staffToken := composeStaffToken(t, client, base)
	if staffToken == "" {
		staffToken = "compose-staff-token"
	}

	approved := composeApproveGameRequest(t, client, base, staffToken, gameID)
	require.Equal(t, "active", stringFromAny(approved["status"]))

	catalog = composeListMatchmakingGames(t, client, base, user.AccessToken)
	found := false
	for _, g := range catalog {
		if stringFromAny(g["name"]) == name {
			found = true
			break
		}
	}
	require.True(t, found, "approved game must appear in public catalog")
}

func composeSubmitGameRequest(t *testing.T, client *http.Client, base, token, name, configJSON string) map[string]any {
	t.Helper()
	payload, err := json.Marshal(map[string]string{
		"name":        name,
		"config_json": configJSON,
	})
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/matchmaking/game-requests", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "submit game request body=%s", string(raw))
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(raw, &parsed))
	game, _ := parsed["game"].(map[string]any)
	if game == nil {
		game = parsed
	}
	return game
}

func composeApproveGameRequest(t *testing.T, client *http.Client, base, staffToken, gameID string) map[string]any {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/admin/matchmaking/game-requests/"+gameID+"/approve", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+staffToken)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "approve game request body=%s", string(raw))
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(raw, &parsed))
	game, _ := parsed["game"].(map[string]any)
	if game == nil {
		game = parsed
	}
	return game
}

func composeListMatchmakingGames(t *testing.T, client *http.Client, base, token string) []map[string]any {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, base+"/api/v1/matchmaking/games", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "list games body=%s", string(raw))
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(raw, &parsed))
	list, _ := parsed["gameList"].(map[string]any)
	if list == nil {
		list, _ = parsed["game_list"].(map[string]any)
	}
	require.NotNil(t, list)
	games, _ := list["games"].([]any)
	out := make([]map[string]any, 0, len(games))
	for _, g := range games {
		if m, ok := g.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}
