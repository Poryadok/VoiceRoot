package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestComposeSearchUnavailableReturns503_live verifies Tier-2 degradation when search upstream is absent.
// Opt-in: VOICE_RUN_LIVE_COMPOSE=true with gateway configured without search upstream.
func TestComposeSearchUnavailableReturns503_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	if os.Getenv("VOICE_SEARCH_DEGRADATION_TEST") != "true" {
		t.Skip("set VOICE_SEARCH_DEGRADATION_TEST=true with search container stopped")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 15 * time.Second}
	base := liveGatewayBaseURL()
	sess := registerComposeUser(t, client, base, fmt.Sprintf("search-deg-%d@voice-qa.test", time.Now().UnixNano()), "VoiceQaTest1!")

	req, err := http.NewRequest(http.MethodGet, base+"/api/v1/search/global?q=test", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+sess.AccessToken)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusServiceUnavailable, resp.StatusCode, "body=%s", string(body))
}

// TestComposeMessagingWorksWhenSearchDown_live ensures DM send still works when search is degraded.
func TestComposeMessagingWorksWhenSearchDown_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	if os.Getenv("VOICE_SEARCH_DEGRADATION_TEST") != "true" {
		t.Skip("set VOICE_SEARCH_DEGRADATION_TEST=true with search container stopped")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 30 * time.Second}
	base := liveGatewayBaseURL()
	n := time.Now().UnixNano()
	sessA := registerComposeUser(t, client, base, fmt.Sprintf("search-deg-a-%d@voice-qa.test", n), "VoiceQaTest1!")
	sessB := registerComposeUser(t, client, base, fmt.Sprintf("search-deg-b-%d@voice-qa.test", n), "VoiceQaTest1!")

	dmPayload, err := json.Marshal(map[string]string{"other_profile_id": sessB.ProfileID})
	require.NoError(t, err)
	dmReq, err := http.NewRequest(http.MethodPost, base+"/api/v1/chats/dm", bytes.NewReader(dmPayload))
	require.NoError(t, err)
	dmReq.Header.Set("Authorization", "Bearer "+sessA.AccessToken)
	dmReq.Header.Set("Content-Type", "application/json")
	dmResp, err := client.Do(dmReq)
	require.NoError(t, err)
	defer dmResp.Body.Close()
	require.Equal(t, http.StatusOK, dmResp.StatusCode)

	var dmParsed struct {
		Chat struct {
			ID string `json:"id"`
		} `json:"chat"`
	}
	require.NoError(t, json.NewDecoder(dmResp.Body).Decode(&dmParsed))
	require.NotEmpty(t, dmParsed.Chat.ID)

	sendPayload, err := json.Marshal(map[string]any{
		"chat":    map[string]string{"id": dmParsed.Chat.ID},
		"content": "messaging still works " + fmt.Sprint(n),
	})
	require.NoError(t, err)
	sendReq, err := http.NewRequest(http.MethodPost, base+"/api/v1/messages/send", bytes.NewReader(sendPayload))
	require.NoError(t, err)
	sendReq.Header.Set("Authorization", "Bearer "+sessA.AccessToken)
	sendReq.Header.Set("Content-Type", "application/json")
	sendResp, err := client.Do(sendReq)
	require.NoError(t, err)
	defer sendResp.Body.Close()
	sendBody, _ := io.ReadAll(sendResp.Body)
	require.Equal(t, http.StatusOK, sendResp.StatusCode, "body=%s", string(sendBody))
}
