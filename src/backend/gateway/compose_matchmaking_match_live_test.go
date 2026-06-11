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

// TestComposeMatchmakingMatch_live exercises matcher → get match → accept → active squad ids.
func TestComposeMatchmakingMatch_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()
	n := time.Now().UnixNano()

	sessA := registerComposeUser(t, client, base, formatComposeEmail("mm-match-a", n), "VoiceQaTest1!")
	sessB := registerComposeUser(t, client, base, formatComposeEmail("mm-match-b", n), "VoiceQaTest1!")

	gameID := findComposeMatchmakingGameID(t, client, base, sessA.AccessToken, "MM Duo Live")
	criteria := map[string]any{"region": "eu"}
	criteriaBytes, _ := json.Marshal(criteria)

	sessionA := startComposeMatchmakingSearch(t, client, base, sessA.AccessToken, gameID, "Duo", string(criteriaBytes))
	sessionB := startComposeMatchmakingSearch(t, client, base, sessB.AccessToken, gameID, "Duo", string(criteriaBytes))

	deadline := time.Now().Add(30 * time.Second)
	var matchID string
	for time.Now().Before(deadline) {
		statusA := getComposeMatchmakingSearch(t, client, base, sessA.AccessToken, sessionA)
		if id, ok := statusA["matchId"].(string); ok && id != "" {
			matchID = id
			break
		}
		if id, ok := statusA["match_id"].(string); ok && id != "" {
			matchID = id
			break
		}
		statusB := getComposeMatchmakingSearch(t, client, base, sessB.AccessToken, sessionB)
		if id, ok := statusB["matchId"].(string); ok && id != "" {
			matchID = id
			break
		}
		if id, ok := statusB["match_id"].(string); ok && id != "" {
			matchID = id
			break
		}
		time.Sleep(2 * time.Second)
	}
	require.NotEmpty(t, matchID, "matcher must create a match for two Duo searches")

	matchA := getComposeMatch(t, client, base, sessA.AccessToken, matchID)
	require.Equal(t, "pending_accept", matchA["status"])

	matchB := getComposeMatch(t, client, base, sessB.AccessToken, matchID)
	require.Equal(t, "pending_accept", matchB["status"])

	respA := respondComposeMatch(t, client, base, sessA.AccessToken, matchID, true)
	matchAfterA := respA["match"].(map[string]any)
	if matchAfterA == nil {
		matchAfterA = respA["Match"].(map[string]any)
	}
	require.NotNil(t, matchAfterA)
	require.Equal(t, "pending_accept", matchAfterA["status"])

	respB := respondComposeMatch(t, client, base, sessB.AccessToken, matchID, true)
	matchActive := respB["match"].(map[string]any)
	if matchActive == nil {
		matchActive = respB["Match"].(map[string]any)
	}
	require.NotNil(t, matchActive)
	require.Equal(t, "active", matchActive["status"])

	chatID, _ := matchActive["chatId"].(string)
	if chatID == "" {
		chatID, _ = matchActive["chat_id"].(string)
	}
	voiceRoomID, _ := matchActive["voiceRoomId"].(string)
	if voiceRoomID == "" {
		voiceRoomID, _ = matchActive["voice_room_id"].(string)
	}
	require.NotEmpty(t, chatID, "active match must have chat_id when squad provisioner is wired")
	require.NotEmpty(t, voiceRoomID, "active match must have voice_room_id when squad provisioner is wired")
}

func findComposeMatchmakingGameID(t *testing.T, client *http.Client, base, token, name string) string {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, base+"/api/v1/matchmaking/games", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
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
	require.True(t, ok, "expected gameList: %s", string(raw))
	games := gameList["games"].([]any)
	for _, g := range games {
		m := g.(map[string]any)
		if m["name"] == name {
			id, _ := m["id"].(string)
			require.NotEmpty(t, id)
			return id
		}
	}
	t.Fatalf("game %q not found in catalog", name)
	return ""
}

func startComposeMatchmakingSearch(t *testing.T, client *http.Client, base, token, gameID, mode, criteriaJSON string) string {
	t.Helper()
	body, _ := json.Marshal(map[string]any{
		"gameId":       gameID,
		"mode":         mode,
		"criteriaJson": criteriaJSON,
	})
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/matchmaking/search", bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "body=%s", string(raw))

	var payload map[string]any
	require.NoError(t, json.Unmarshal(raw, &payload))
	session, ok := payload["searchSession"].(map[string]any)
	if !ok {
		session, ok = payload["search_session"].(map[string]any)
	}
	require.True(t, ok, "expected searchSession: %s", string(raw))
	id, _ := session["id"].(string)
	require.NotEmpty(t, id)
	return id
}

func getComposeMatchmakingSearch(t *testing.T, client *http.Client, base, token, sessionID string) map[string]any {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, base+"/api/v1/matchmaking/search/"+sessionID, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "body=%s", string(raw))

	var payload map[string]any
	require.NoError(t, json.Unmarshal(raw, &payload))
	session, ok := payload["searchSession"].(map[string]any)
	if !ok {
		session, ok = payload["search_session"].(map[string]any)
	}
	require.True(t, ok)
	return session
}

func getComposeMatch(t *testing.T, client *http.Client, base, token, matchID string) map[string]any {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, base+"/api/v1/matchmaking/matches/"+matchID, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "body=%s", string(raw))

	var payload map[string]any
	require.NoError(t, json.Unmarshal(raw, &payload))
	match, ok := payload["match"].(map[string]any)
	if !ok {
		match, ok = payload["Match"].(map[string]any)
	}
	require.True(t, ok, "expected match: %s", string(raw))
	return match
}

func respondComposeMatch(t *testing.T, client *http.Client, base, token, matchID string, accept bool) map[string]any {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"accept": accept})
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/matchmaking/matches/"+matchID+"/respond", bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "body=%s", string(raw))

	var payload map[string]any
	require.NoError(t, json.Unmarshal(raw, &payload))
	return payload
}
