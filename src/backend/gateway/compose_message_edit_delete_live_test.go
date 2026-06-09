package main

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestComposeMessageEditDelete_live: REST edit/delete and WS message_update fanout.
func TestComposeMessageEditDelete_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()

	n := time.Now().UnixNano()
	sessA := registerComposeUser(t, client, base, formatComposeEmail("edit-a", n), "VoiceQaTest1!")
	sessB := registerComposeUser(t, client, base, formatComposeEmail("edit-b", n), "VoiceQaTest1!")
	chatID := createComposeDM(t, client, base, sessA.AccessToken, sessB.ProfileID)

	wsB := connectComposeWSSubscribed(t, base, sessB.AccessToken, chatID)
	msgID := sendComposeMessage(t, client, base, sessA.AccessToken, chatID, "before-edit")
	_ = waitComposeWSOp(t, wsB, "message_create", 20*time.Second, func(d map[string]any) bool {
		return d["message_id"] == msgID
	})

	const edited = "after-edit"
	updateCh := make(chan composeWSFrame, 1)
	go func() {
		updateCh <- waitComposeWSOp(t, wsB, "message_update", 20*time.Second, func(d map[string]any) bool {
			return d["message_id"] == msgID
		})
	}()
	editComposeMessage(t, client, base, sessA.AccessToken, msgID, edited)

	select {
	case frame := <-updateCh:
		var d map[string]any
		require.NoError(t, json.Unmarshal(frame.D, &d))
		require.Equal(t, chatID, d["chat_id"])
	case <-time.After(25 * time.Second):
		t.Fatal("timeout waiting for message_update")
	}
	getComposeMessagesContains(t, client, base, sessB.AccessToken, chatID, msgID, edited)

	deleteComposeMessage(t, client, base, sessA.AccessToken, msgID, "everyone")
	composeMessageAbsentFromHistory(t, client, base, sessB.AccessToken, chatID, msgID)
}

func composeMessageAbsentFromHistory(
	t *testing.T,
	client *http.Client,
	base, accessToken, chatID, messageID string,
) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, base+"/api/v1/messages?chat_id="+chatID, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "GET messages body=%s", string(body))

	var hist struct {
		MessageList struct {
			Messages []struct {
				ID string `json:"id"`
			} `json:"messages"`
		} `json:"message_list"`
	}
	require.NoError(t, json.Unmarshal(body, &hist))
	for _, m := range hist.MessageList.Messages {
		require.NotEqual(t, messageID, m.ID, "deleted message must not appear in history")
	}
}
