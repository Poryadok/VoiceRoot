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

// TestComposeThreadsDMReply_live exercises Phase 10 DM reply + thread fetch through Gateway.
//
// Opt-in: VOICE_RUN_LIVE_COMPOSE=true VOICE_API_BASE_URL=http://127.0.0.1:18080
func TestComposeThreadsDMReply_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 30 * time.Second}
	base := liveGatewayBaseURL()

	n := time.Now().UnixNano()
	sessA := registerComposeUser(t, client, base, fmt.Sprintf("thread-a-%d@voice-qa.test", n), "VoiceQaTest1!")
	sessB := registerComposeUser(t, client, base, fmt.Sprintf("thread-b-%d@voice-qa.test", n), "VoiceQaTest1!")

	dmPayload, _ := json.Marshal(map[string]string{"other_profile_id": sessB.ProfileID})
	dmReq, _ := http.NewRequest(http.MethodPost, base+"/api/v1/chats/dm", bytes.NewReader(dmPayload))
	dmReq.Header.Set("Authorization", "Bearer "+sessA.AccessToken)
	dmReq.Header.Set("Content-Type", "application/json")
	dmResp, err := client.Do(dmReq)
	require.NoError(t, err)
	defer dmResp.Body.Close()
	dmBody, _ := io.ReadAll(dmResp.Body)
	require.Equal(t, http.StatusOK, dmResp.StatusCode, "body=%s", string(dmBody))

	var dmParsed struct {
		Chat struct {
			ID string `json:"id"`
		} `json:"chat"`
	}
	require.NoError(t, json.Unmarshal(dmBody, &dmParsed))
	chatID := dmParsed.Chat.ID
	require.NotEmpty(t, chatID)

	rootPayload, _ := json.Marshal(map[string]any{
		"chat":    map[string]string{"id": chatID},
		"content": "thread-root",
	})
	rootReq, _ := http.NewRequest(http.MethodPost, base+"/api/v1/messages/send", bytes.NewReader(rootPayload))
	rootReq.Header.Set("Authorization", "Bearer "+sessA.AccessToken)
	rootReq.Header.Set("Content-Type", "application/json")
	rootResp, err := client.Do(rootReq)
	require.NoError(t, err)
	defer rootResp.Body.Close()
	rootBody, _ := io.ReadAll(rootResp.Body)
	require.Equal(t, http.StatusOK, rootResp.StatusCode, "body=%s", string(rootBody))

	var rootParsed struct {
		Message struct {
			ID string `json:"id"`
		} `json:"message"`
	}
	require.NoError(t, json.Unmarshal(rootBody, &rootParsed))
	parentID := rootParsed.Message.ID
	require.NotEmpty(t, parentID)

	replyPayload, _ := json.Marshal(map[string]any{
		"chat":             map[string]string{"id": chatID},
		"content":          "thread-reply",
		"thread_parent_id": parentID,
	})
	replyReq, _ := http.NewRequest(http.MethodPost, base+"/api/v1/messages/send", bytes.NewReader(replyPayload))
	replyReq.Header.Set("Authorization", "Bearer "+sessB.AccessToken)
	replyReq.Header.Set("Content-Type", "application/json")
	replyResp, err := client.Do(replyReq)
	require.NoError(t, err)
	defer replyResp.Body.Close()
	replyBody, _ := io.ReadAll(replyResp.Body)
	require.Equal(t, http.StatusOK, replyResp.StatusCode, "body=%s", string(replyBody))

	mainURL := fmt.Sprintf("%s/api/v1/messages?chat_id=%s", base, chatID)
	mainReq, _ := http.NewRequest(http.MethodGet, mainURL, nil)
	mainReq.Header.Set("Authorization", "Bearer "+sessA.AccessToken)
	mainResp, err := client.Do(mainReq)
	require.NoError(t, err)
	defer mainResp.Body.Close()
	mainBody, _ := io.ReadAll(mainResp.Body)
	require.Equal(t, http.StatusOK, mainResp.StatusCode, "body=%s", string(mainBody))

	var mainHist struct {
		MessageList struct {
			Messages []struct {
				ID      string `json:"id"`
				Content string `json:"content"`
			} `json:"messages"`
		} `json:"message_list"`
	}
	require.NoError(t, json.Unmarshal(mainBody, &mainHist))
	for _, m := range mainHist.MessageList.Messages {
		require.NotEqual(t, "thread-reply", m.Content, "reply must not appear in main feed")
	}

	threadURL := fmt.Sprintf("%s/api/v1/messages/thread?chat_id=%s&thread_parent_id=%s", base, chatID, parentID)
	threadReq, _ := http.NewRequest(http.MethodGet, threadURL, nil)
	threadReq.Header.Set("Authorization", "Bearer "+sessA.AccessToken)
	threadResp, err := client.Do(threadReq)
	require.NoError(t, err)
	defer threadResp.Body.Close()
	threadBody, _ := io.ReadAll(threadResp.Body)
	require.Equal(t, http.StatusOK, threadResp.StatusCode, "body=%s", string(threadBody))

	var threadHist struct {
		MessageList struct {
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
		} `json:"message_list"`
	}
	require.NoError(t, json.Unmarshal(threadBody, &threadHist))
	require.NotEmpty(t, threadHist.MessageList.Messages)
	foundReply := false
	for _, m := range threadHist.MessageList.Messages {
		if m.Content == "thread-reply" {
			foundReply = true
		}
	}
	require.True(t, foundReply, "thread endpoint must return reply")
}
