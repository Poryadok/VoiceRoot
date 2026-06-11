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

// TestComposeMatchmakingRating_live exercises complete → rate → player rating aggregate.
func TestComposeMatchmakingRating_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()
	n := time.Now().UnixNano()

	sessA := registerComposeUser(t, client, base, formatComposeEmail("mm-rate-a", n), "VoiceQaTest1!")
	sessB := registerComposeUser(t, client, base, formatComposeEmail("mm-rate-b", n), "VoiceQaTest1!")

	gameID := findComposeMatchmakingGameID(t, client, base, sessA.AccessToken, "MM Duo Live")
	criteria := map[string]any{"region": "eu"}
	criteriaBytes, _ := json.Marshal(criteria)

	sessionA := startComposeMatchmakingSearch(t, client, base, sessA.AccessToken, gameID, "Duo", string(criteriaBytes))
	_ = startComposeMatchmakingSearch(t, client, base, sessB.AccessToken, gameID, "Duo", string(criteriaBytes))

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
		time.Sleep(2 * time.Second)
	}
	require.NotEmpty(t, matchID)

	respondComposeMatch(t, client, base, sessA.AccessToken, matchID, true)
	respB := respondComposeMatch(t, client, base, sessB.AccessToken, matchID, true)
	matchActive := respB["match"].(map[string]any)
	if matchActive == nil {
		matchActive = respB["Match"].(map[string]any)
	}
	require.Equal(t, "active", matchActive["status"])

	profileB := extractProfileID(t, matchActive, sessB.ProfileID)
	completeComposeMatch(t, client, base, sessA.AccessToken, matchID)
	completeComposeMatch(t, client, base, sessB.AccessToken, matchID)

	rateComposeMatch(t, client, base, sessA.AccessToken, matchID, profileB, 5)

	rating := getComposePlayerRating(t, client, base, sessA.AccessToken, profileB, gameID)
	ratingValue, _ := rating["ratingValue"].(float64)
	if ratingValue == 0 {
		ratingValue, _ = rating["rating_value"].(float64)
	}
	require.Equal(t, 5.0, ratingValue)
}

func extractProfileID(t *testing.T, match map[string]any, fallback string) string {
	t.Helper()
	ids, ok := match["profileIds"].([]any)
	if !ok {
		ids, ok = match["profile_ids"].([]any)
	}
	if ok {
		for _, id := range ids {
			s, _ := id.(string)
			if s != "" && s != fallback {
				return s
			}
		}
	}
	require.NotEmpty(t, fallback)
	return fallback
}

func completeComposeMatch(t *testing.T, client *http.Client, base, token, matchID string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/matchmaking/matches/"+matchID+"/complete", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "body=%s", string(raw))
}

func rateComposeMatch(t *testing.T, client *http.Client, base, token, matchID, ratedProfileID string, stars int) {
	t.Helper()
	body, _ := json.Marshal(map[string]any{
		"ratedProfileId": ratedProfileID,
		"stars":          stars,
	})
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/matchmaking/matches/"+matchID+"/rate", bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "body=%s", string(raw))
}

func getComposePlayerRating(t *testing.T, client *http.Client, base, token, profileID, gameID string) map[string]any {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, base+"/api/v1/matchmaking/players/"+profileID+"/rating?game_id="+gameID, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "body=%s", string(raw))

	var payload map[string]any
	require.NoError(t, json.Unmarshal(raw, &payload))
	rating, ok := payload["playerRating"].(map[string]any)
	if !ok {
		rating, ok = payload["player_rating"].(map[string]any)
	}
	require.True(t, ok, "expected playerRating: %s", string(raw))
	return rating
}
