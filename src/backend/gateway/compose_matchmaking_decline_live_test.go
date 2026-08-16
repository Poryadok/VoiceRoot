package main

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestComposeMatchmakingCrossPartyDecline_live documents MM-07:
// solo parties — acceptor continues searching after foreign-party decline.
func TestComposeMatchmakingCrossPartyDecline_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 60 * time.Second}
	base := liveGatewayBaseURL()
	n := time.Now().UnixNano()

	sessA := registerComposeUser(t, client, base, formatComposeEmail("mm07-a", n), "VoiceQaTest1!")
	sessB := registerComposeUser(t, client, base, formatComposeEmail("mm07-b", n), "VoiceQaTest1!")

	gameID := findComposeMatchmakingGameID(t, client, base, sessA.AccessToken, "MM Duo Live")
	criteriaBytes := []byte(`{"region":"eu"}`)

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

	respA := respondComposeMatch(t, client, base, sessA.AccessToken, matchID, true)
	matchAfterA, _ := respA["match"].(map[string]any)
	if matchAfterA == nil {
		matchAfterA, _ = respA["Match"].(map[string]any)
	}
	require.NotNil(t, matchAfterA)
	require.Equal(t, "pending_accept", matchAfterA["status"])

	respB := respondComposeMatch(t, client, base, sessB.AccessToken, matchID, false)
	declinerSession, _ := respB["searchSession"].(map[string]any)
	if declinerSession == nil {
		declinerSession, _ = respB["search_session"].(map[string]any)
	}
	require.NotNil(t, declinerSession)
	require.Equal(t, "cancelled", declinerSession["status"])

	matchAbandoned, _ := respB["match"].(map[string]any)
	if matchAbandoned == nil {
		matchAbandoned, _ = respB["Match"].(map[string]any)
	}
	require.NotNil(t, matchAbandoned)
	require.Equal(t, "abandoned", matchAbandoned["status"])

	// Acceptor's own party (solo) continues searching.
	statusA := getComposeMatchmakingSearch(t, client, base, sessA.AccessToken, sessionA)
	require.Equal(t, "searching", statusA["status"], "acceptor must continue searching after foreign decline")
}
