package main

import (
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestComposeDMRequests_live: stranger DM lands in requests inbox until accepted.
func TestComposeDMRequests_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()

	n := time.Now().UnixNano()
	sessA := registerComposeUser(t, client, base, formatComposeEmail("dmreq-a", n), "VoiceQaTest1!")
	sessB := registerComposeUser(t, client, base, formatComposeEmail("dmreq-b", n), "VoiceQaTest1!")
	chatID := createComposeDM(t, client, base, sessA.AccessToken, sessB.ProfileID)

	mainBefore := listComposeChats(t, client, base, sessB.AccessToken, "main")
	for _, item := range mainBefore {
		require.NotEqual(t, chatID, item.ChatID)
	}

	requests := listComposeChats(t, client, base, sessB.AccessToken, "requests")
	require.Len(t, requests, 1)
	require.Equal(t, chatID, requests[0].ChatID)
	require.True(t, requests[0].IsStranger)

	acceptComposeDMRequest(t, client, base, sessB.AccessToken, chatID)

	mainAfter := listComposeChats(t, client, base, sessB.AccessToken, "main")
	require.Len(t, mainAfter, 1)
	require.Equal(t, chatID, mainAfter[0].ChatID)
}

func acceptComposeDMRequest(t *testing.T, client *http.Client, base, accessToken, chatID string) {
	t.Helper()
	url := base + "/api/v1/chats/" + chatID + "/accept-request"
	req, err := http.NewRequest(http.MethodPost, url, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent,
		"accept dm request status=%d body=%s", resp.StatusCode, string(body))
}
