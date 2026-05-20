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

// TestComposeChatMessagingDM_live exercises Phase-1 DM + message send through Gateway
// with Chat and Messaging gRPC upstreams (docker compose --profile app).
//
// Opt-in: VOICE_RUN_LIVE_COMPOSE=true VOICE_API_BASE_URL=http://127.0.0.1:18080
func TestComposeChatMessagingDM_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}

	client := &http.Client{Timeout: 30 * time.Second}
	base := liveGatewayBaseURL()

	healthResp, err := client.Get(base + "/health")
	require.NoError(t, err)
	healthResp.Body.Close()
	require.Equal(t, http.StatusOK, healthResp.StatusCode, "gateway health at %s", base)

	n := time.Now().UnixNano()
	emailA := fmt.Sprintf("dm-a-%d@voice-qa.test", n)
	emailB := fmt.Sprintf("dm-b-%d@voice-qa.test", n)
	const password = "VoiceQaTest1!"

	sessA := registerComposeUser(t, client, base, emailA, password)
	sessB := registerComposeUser(t, client, base, emailB, password)

	listReq, err := http.NewRequest(http.MethodGet, base+"/api/v1/chats", nil)
	require.NoError(t, err)
	listReq.Header.Set("Authorization", "Bearer "+sessA.AccessToken)
	listResp, err := client.Do(listReq)
	require.NoError(t, err)
	defer listResp.Body.Close()
	listBody, _ := io.ReadAll(listResp.Body)
	require.Equal(t, http.StatusOK, listResp.StatusCode,
		"GET /api/v1/chats must not 404 once chat upstream is wired; body=%s", string(listBody))

	dmPayload, err := json.Marshal(map[string]string{"other_profile_id": sessB.ProfileID})
	require.NoError(t, err)
	dmReq, err := http.NewRequest(http.MethodPost, base+"/api/v1/chats/dm", bytes.NewReader(dmPayload))
	require.NoError(t, err)
	dmReq.Header.Set("Authorization", "Bearer "+sessA.AccessToken)
	dmReq.Header.Set("Content-Type", "application/json")
	dmResp, err := client.Do(dmReq)
	require.NoError(t, err)
	defer dmResp.Body.Close()
	dmBody, _ := io.ReadAll(dmResp.Body)
	require.Equal(t, http.StatusOK, dmResp.StatusCode,
		"POST /api/v1/chats/dm must succeed; body=%s", string(dmBody))

	var dmParsed struct {
		Chat struct {
			ID string `json:"id"`
		} `json:"chat"`
	}
	require.NoError(t, json.Unmarshal(dmBody, &dmParsed))
	require.NotEmpty(t, dmParsed.Chat.ID)

	sendPayload, err := json.Marshal(map[string]any{
		"chat":    map[string]string{"id": dmParsed.Chat.ID},
		"content": "compose-live-smoke",
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
	require.Equal(t, http.StatusOK, sendResp.StatusCode,
		"POST /api/v1/messages/send must not 404 once messaging upstream is wired; body=%s", string(sendBody))

	var sendParsed struct {
		Message struct {
			ID      string `json:"id"`
			Content string `json:"content"`
		} `json:"message"`
	}
	require.NoError(t, json.Unmarshal(sendBody, &sendParsed))
	require.NotEmpty(t, sendParsed.Message.ID)
	require.Equal(t, "compose-live-smoke", sendParsed.Message.Content)

	msgURL := fmt.Sprintf("%s/api/v1/messages?chat_id=%s", base, dmParsed.Chat.ID)
	getReq, err := http.NewRequest(http.MethodGet, msgURL, nil)
	require.NoError(t, err)
	getReq.Header.Set("Authorization", "Bearer "+sessA.AccessToken)
	getResp, err := client.Do(getReq)
	require.NoError(t, err)
	defer getResp.Body.Close()
	getBody, _ := io.ReadAll(getResp.Body)
	require.Equal(t, http.StatusOK, getResp.StatusCode,
		"GET /api/v1/messages must return history; body=%s", string(getBody))

	var hist struct {
		MessageList struct {
			Messages []struct {
				ID      string `json:"id"`
				Content string `json:"content"`
			} `json:"messages"`
		} `json:"message_list"`
	}
	require.NoError(t, json.Unmarshal(getBody, &hist))
	require.NotEmpty(t, hist.MessageList.Messages)
	found := false
	for _, m := range hist.MessageList.Messages {
		if m.Content == "compose-live-smoke" {
			found = true
			break
		}
	}
	require.True(t, found, "message history should include sent content; body=%s", string(getBody))
}
