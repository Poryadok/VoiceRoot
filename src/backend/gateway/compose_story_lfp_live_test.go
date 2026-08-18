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

// TestComposeStoryLfpParty_live documents ST-04:
// LFP story → JOIN response → DecideLfp ACCEPT → party search sessions.
func TestComposeStoryLfpParty_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 60 * time.Second}
	base := liveGatewayBaseURL()
	n := time.Now().UnixNano()

	author := registerComposeUser(t, client, base, formatComposeEmail("st04-author", n), "VoiceQaTest1!")
	responder := registerComposeUser(t, client, base, formatComposeEmail("st04-resp", n), "VoiceQaTest1!")

	// Open privacy so LFP visibility floors do not block respond.
	patchComposePrivacy(t, client, base, author.AccessToken, composeGamingOpenPrivacySettings())

	gameID := findComposeMatchmakingGameID(t, client, base, author.AccessToken, "MM Duo Live")
	criteria := fmt.Sprintf(
		`{"game_id":%q,"mode":"Duo","region":"eu","visibility":"everyone"}`,
		gameID,
	)

	story := composeCreateLookingForParty(t, client, base, author.AccessToken, criteria)
	storyID := composeStoryID(t, story)
	require.NotEmpty(t, storyID)

	composeRespondToLfpStory(t, client, base, responder.AccessToken, storyID, "JOIN")

	var decide map[string]any
	require.Eventually(t, func() bool {
		status, raw := composeDecideLfpRequestStatus(
			t, client, base, author.AccessToken, storyID, responder.ProfileID, "JOIN", "ACCEPT",
		)
		if status != http.StatusOK {
			return false
		}
		var payload map[string]any
		if json.Unmarshal(raw, &payload) != nil {
			return false
		}
		st := stringFromAny(payload["status"])
		if st != "accepted" {
			return false
		}
		decide = payload
		return true
	}, 25*time.Second, 500*time.Millisecond, "storyconsume must materialize pending LFP request")

	require.NotEmpty(t, stringFromAny(decide["partyId"], decide["party_id"]))
	sessions, _ := decide["searchSessions"].([]any)
	if sessions == nil {
		sessions, _ = decide["search_sessions"].([]any)
	}
	require.Len(t, sessions, 2, "ACCEPT JOIN must enqueue author+responder search sessions")
	for _, s := range sessions {
		sm, _ := s.(map[string]any)
		require.Equal(t, "searching", sm["status"])
	}
}

func composeCreateLookingForParty(t *testing.T, client *http.Client, base, token, criteriaJSON string) map[string]any {
	t.Helper()
	payload, err := json.Marshal(map[string]string{"criteria_json": criteriaJSON})
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/stories/looking-for-party", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "create LFP body=%s", string(raw))
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(raw, &parsed))
	story, _ := parsed["story"].(map[string]any)
	require.NotNil(t, story)
	return story
}

func composeRespondToLfpStory(t *testing.T, client *http.Client, base, token, storyID, responseType string) {
	t.Helper()
	payload, err := json.Marshal(map[string]string{
		"response_type": responseType,
	})
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/stories/"+storyID+"/lfp-response", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	require.True(t, resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusOK,
		"lfp-response body=%s", string(raw))
}

func composeDecideLfpRequestStatus(
	t *testing.T, client *http.Client, base, token, storyID, responderProfileID, responseType, decision string,
) (int, []byte) {
	t.Helper()
	payload, err := json.Marshal(map[string]string{
		"story_id":             storyID,
		"responder_profile_id": responderProfileID,
		"response_type":        responseType,
		"decision":             decision,
	})
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/matchmaking/lfp-requests/decide", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, raw
}
