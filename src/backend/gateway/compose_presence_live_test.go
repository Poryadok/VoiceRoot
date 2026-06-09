package main

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestComposePresence_live: callee WS connect sets online; observer reads REST presence.
func TestComposePresence_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()

	n := time.Now().UnixNano()
	sessA := registerComposeUser(t, client, base, formatComposeEmail("presence-a", n), "VoiceQaTest1!")
	sessB := registerComposeUser(t, client, base, formatComposeEmail("presence-b", n), "VoiceQaTest1!")

	wsB := dialComposeRealtimeWS(t, base, sessB.AccessToken)
	waitComposeWSHello(t, wsB)

	deadline := time.Now().Add(20 * time.Second)
	var status string
	for time.Now().Before(deadline) {
		status = composeGetPresenceStatus(t, client, base, sessA.AccessToken, sessB.ProfileID)
		if status == "online" {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	require.Equal(t, "online", status, "profile B should appear online after WS connect")
}

func composeGetPresenceStatus(t *testing.T, client *http.Client, base, accessToken, profileID string) string {
	t.Helper()
	url := base + "/api/v1/users/profiles/" + profileID + "/presence"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "presence body=%s", string(body))

	var parsed struct {
		PresenceStatus struct {
			Status string `json:"status"`
		} `json:"presence_status"`
	}
	require.NoError(t, json.Unmarshal(body, &parsed))
	return parsed.PresenceStatus.Status
}
