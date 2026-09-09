package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestComposeSearchInChat_live indexes a message via compose and finds it through Search.
//
// Opt-in: VOICE_RUN_LIVE_COMPOSE=true VOICE_API_BASE_URL=http://127.0.0.1:18080
func TestComposeSearchInChat_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 30 * time.Second}
	base := liveGatewayBaseURL()

	healthResp, err := client.Get(base + "/health")
	require.NoError(t, err)
	healthResp.Body.Close()
	require.Equal(t, http.StatusOK, healthResp.StatusCode)

	n := time.Now().UnixNano()
	token := fmt.Sprintf("search-token-%d", n)
	emailA := fmt.Sprintf("search-a-%d@voice-qa.test", n)
	emailB := fmt.Sprintf("search-b-%d@voice-qa.test", n)
	const password = "VoiceQaTest1!"

	sessA := registerComposeUser(t, client, base, emailA, password)
	sessB := registerComposeUser(t, client, base, emailB, password)

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
	require.Equal(t, http.StatusOK, dmResp.StatusCode, "body=%s", string(dmBody))

	var dmParsed struct {
		Chat struct {
			ID string `json:"id"`
		} `json:"chat"`
	}
	require.NoError(t, json.Unmarshal(dmBody, &dmParsed))
	require.NotEmpty(t, dmParsed.Chat.ID)

	sendPayload, err := json.Marshal(map[string]any{
		"chat":    map[string]string{"id": dmParsed.Chat.ID},
		"content": "compose search unique " + token,
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
	var sent struct {
		Message struct {
			ID string `json:"id"`
		} `json:"message"`
	}
	require.NoError(t, json.Unmarshal(sendBody, &sent))
	require.NotEmpty(t, sent.Message.ID)

	searchURL := fmt.Sprintf("%s/api/v1/search/in-chat?chat_id=%s&q=%s",
		base, url.QueryEscape(dmParsed.Chat.ID), url.QueryEscape(token))
	var lastBody string
	var lastCode int
	var lastErr string
	if !assert.Eventually(t, func() bool {
		req, err := http.NewRequest(http.MethodGet, searchURL, nil)
		if err != nil {
			return false
		}
		req.Header.Set("Authorization", "Bearer "+sessA.AccessToken)
		resp, err := client.Do(req)
		if err != nil {
			lastCode = 0
			lastBody = ""
			lastErr = err.Error()
			return false
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		lastBody = string(body)
		lastCode = resp.StatusCode
		lastErr = ""
		return resp.StatusCode == http.StatusOK && searchResponseContainsMessage(body, sent.Message.ID, token)
	}, 45*time.Second, 2*time.Second) {
		t.Fatalf("search must return indexed message; last status=%d body=%s transport_error=%s", lastCode, lastBody, lastErr)
	}
}

// TestComposeSearchNamespace_live ensures /api/v1/search routes are wired (not 404).
func TestComposeSearchNamespace_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 15 * time.Second}
	base := liveGatewayBaseURL()
	sess := registerComposeUser(t, client, base, fmt.Sprintf("search-ns-%d@voice-qa.test", time.Now().UnixNano()), "VoiceQaTest1!")

	req, err := http.NewRequest(http.MethodGet, base+"/api/v1/search/global?q=test", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+sess.AccessToken)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.NotEqual(t, http.StatusNotFound, resp.StatusCode,
		"GET /api/v1/search/global must be wired when search upstream exists; body=%s", string(body))
}

func searchResponseContainsMessage(body []byte, messageID, token string) bool {
	var parsed struct {
		SearchResults struct {
			Hits []struct {
				MessageID string `json:"message_id"`
				Snippet   string `json:"snippet"`
			} `json:"hits"`
		} `json:"search_results"`
	}
	if json.Unmarshal(body, &parsed) != nil {
		return false
	}
	for _, hit := range parsed.SearchResults.Hits {
		if hit.MessageID == messageID && strings.Contains(stripSearchHighlightTags(hit.Snippet), token) {
			return true
		}
	}
	return false
}

// stripSearchHighlightTags removes the <b> tags emitted by MessageSearchStore's ts_headline configuration.
func stripSearchHighlightTags(snippet string) string {
	return strings.NewReplacer("<b>", "", "</b>", "").Replace(snippet)
}

func TestSearchResponseContainsMessage(t *testing.T) {
	body := []byte(`{"search_results":{"hits":[{"message_id":"message-1","snippet":"compose <b>search</b> unique <b>search</b>-<b>token</b>"}]}}`)

	require.True(t, searchResponseContainsMessage(body, "message-1", "search-token"))
	require.False(t, searchResponseContainsMessage(body, "other-message", "search-token"))
	require.False(t, searchResponseContainsMessage(body, "message-1", "other-token"))
	require.False(t, searchResponseContainsMessage([]byte(`not json`), "message-1", "search-token"))
}
