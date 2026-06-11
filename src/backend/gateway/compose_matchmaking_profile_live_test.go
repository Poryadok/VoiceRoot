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

// TestComposeMatchmakingPlayerProfile_live exercises profile CRUD via Gateway.
func TestComposeMatchmakingPlayerProfile_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()

	n := time.Now().UnixNano()
	sess := registerComposeUser(t, client, base, formatComposeEmail("mm-profile", n), "VoiceQaTest1!")

	listReq, err := http.NewRequest(http.MethodGet, base+"/api/v1/matchmaking/games", nil)
	require.NoError(t, err)
	listReq.Header.Set("Authorization", "Bearer "+sess.AccessToken)
	listResp, err := client.Do(listReq)
	require.NoError(t, err)
	defer listResp.Body.Close()
	listRaw, _ := io.ReadAll(listResp.Body)
	require.Equal(t, http.StatusOK, listResp.StatusCode, "body=%s", string(listRaw))

	var listPayload map[string]any
	require.NoError(t, json.Unmarshal(listRaw, &listPayload))
	gameList, ok := listPayload["gameList"].(map[string]any)
	if !ok {
		gameList, ok = listPayload["game_list"].(map[string]any)
	}
	require.True(t, ok)
	games, ok := gameList["games"].([]any)
	require.True(t, ok)
	require.NotEmpty(t, games)
	dota, ok := games[0].(map[string]any)
	require.True(t, ok)
	gameID, _ := dota["id"].(string)
	require.NotEmpty(t, gameID)

	upsertBody, _ := json.Marshal(map[string]any{
		"region": "eu",
		"role":   "Carry",
		"rank":   "Herald",
	})
	putReq, err := http.NewRequest(http.MethodPut, base+"/api/v1/matchmaking/profile/games/"+gameID, bytes.NewReader(upsertBody))
	require.NoError(t, err)
	putReq.Header.Set("Authorization", "Bearer "+sess.AccessToken)
	putReq.Header.Set("Content-Type", "application/json")
	putResp, err := client.Do(putReq)
	require.NoError(t, err)
	defer putResp.Body.Close()
	putRaw, _ := io.ReadAll(putResp.Body)
	require.Equal(t, http.StatusOK, putResp.StatusCode, "body=%s", string(putRaw))

	meReq, err := http.NewRequest(http.MethodGet, base+"/api/v1/matchmaking/profile/me", nil)
	require.NoError(t, err)
	meReq.Header.Set("Authorization", "Bearer "+sess.AccessToken)
	meResp, err := client.Do(meReq)
	require.NoError(t, err)
	defer meResp.Body.Close()
	meRaw, _ := io.ReadAll(meResp.Body)
	require.Equal(t, http.StatusOK, meResp.StatusCode, "body=%s", string(meRaw))
	require.Contains(t, string(meRaw), gameID)
	require.Contains(t, string(meRaw), "Herald")

	delReq, err := http.NewRequest(http.MethodDelete, base+"/api/v1/matchmaking/profile/games/"+gameID, nil)
	require.NoError(t, err)
	delReq.Header.Set("Authorization", "Bearer "+sess.AccessToken)
	delResp, err := client.Do(delReq)
	require.NoError(t, err)
	defer delResp.Body.Close()
	require.Equal(t, http.StatusOK, delResp.StatusCode)
}
