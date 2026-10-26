package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
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

	profileB := extractProfileID(t, matchActive, sessA.ProfileID)
	require.Equal(t, sessB.ProfileID, profileB)
	completeComposeMatch(t, client, base, sessA.AccessToken, matchID)
	completeComposeMatch(t, client, base, sessB.AccessToken, matchID)
	waitComposeMatchSquadTeardownComplete(t, matchID)
	assertComposeMatchHistoryContains(t, client, base, sessA.AccessToken, matchID)

	rateComposeMatch(t, client, base, sessA.AccessToken, matchID, profileB, 5)

	rating := getComposePlayerRating(t, client, base, sessA.AccessToken, profileB, gameID)
	ratingValue, _ := rating["ratingValue"].(float64)
	if ratingValue == 0 {
		ratingValue, _ = rating["rating_value"].(float64)
	}
	require.Equal(t, 5.0, ratingValue)

	banComposeMMPeer(t, client, base, sessA.AccessToken, profileB)
	require.True(t, getComposeMMBanStatus(t, client, base, sessA.AccessToken, profileB))

	nextA := startComposeMatchmakingSearch(t, client, base, sessA.AccessToken, gameID, "Duo", string(criteriaBytes))
	nextB := startComposeMatchmakingSearch(t, client, base, sessB.AccessToken, gameID, "Duo", string(criteriaBytes))
	deadline = time.Now().Add(12 * time.Second)
	for time.Now().Before(deadline) {
		statusA := getComposeMatchmakingSearch(t, client, base, sessA.AccessToken, nextA)
		statusB := getComposeMatchmakingSearch(t, client, base, sessB.AccessToken, nextB)
		require.Empty(t, composeSearchMatchID(statusA), "banned peers must not match in a subsequent search")
		require.Empty(t, composeSearchMatchID(statusB), "banned peers must not match in a subsequent search")
		time.Sleep(2 * time.Second)
	}
}

func waitComposeMatchSquadTeardownComplete(t *testing.T, matchID string) {
	t.Helper()
	id, err := uuid.Parse(matchID)
	require.NoError(t, err, "match id must be a UUID before querying teardown state")
	project := os.Getenv("COMPOSE_PROJECT_NAME")
	require.NotEmpty(t, project, "COMPOSE_PROJECT_NAME must bind the read to this disposable Compose project")
	// Provider teardown and search-reservation projection are independent durable
	// cleanup steps. Observe both before exercising the next concurrent searches.
	query := fmt.Sprintf(`
		SELECT count(*)::text || ':' || COALESCE(max(state), '') || ':' ||
		       (SELECT count(*)::text FROM search_sessions WHERE match_id='%[1]s' AND status='matched') || ':' ||
		       (SELECT count(*)::text FROM matchmaking_terminal_search_releases
		        WHERE search_session_id IN (SELECT search_session_id FROM match_proposals WHERE match_id='%[1]s'))
		FROM matchmaking_match_squad_teardowns WHERE match_id='%[1]s'
	`, id.String())
	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		cmd := exec.Command("docker", "compose", "--project-name", project, "exec", "-T", "postgres", "psql", "-X", "-U", "voice", "-d", "matchmaking_db", "-t", "-A", "-v", "ON_ERROR_STOP=1", "-c", query)
		cmd.Dir = repoRootFromTest(t)
		out, runErr := cmd.CombinedOutput()
		require.NoError(t, runErr, "required teardown-state query failed (project=%s): %s", project, strings.TrimSpace(string(out)))
		if strings.TrimSpace(string(out)) == "1:complete:2:0" {
			return
		}
		time.Sleep(time.Second)
	}
	t.Fatalf("match cleanup did not complete provider teardown and both search releases for match %s in the disposable Compose project", id)
}

func assertComposeMatchHistoryContains(t *testing.T, client *http.Client, base, token, matchID string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, base+"/api/v1/matchmaking/profile/me/matches?page_size=25", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode, "match history body=%s", string(raw))
	require.Contains(t, string(raw), matchID, "completed match must remain in matchmaking history")
}

func composeSearchMatchID(status map[string]any) string {
	id, _ := status["matchId"].(string)
	if id == "" {
		id, _ = status["match_id"].(string)
	}
	return id
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
	body, err := json.Marshal(map[string]string{"operationId": uuid.NewString()})
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/matchmaking/matches/"+matchID+"/complete", bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
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
