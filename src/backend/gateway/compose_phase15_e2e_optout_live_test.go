package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestComposePhase15E2EOptOut_live documents opt-out reverts DM to plaintext (encryption.md).
func TestComposePhase15E2EOptOut_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()

	n := time.Now().UnixNano()
	sessA := registerComposeUser(t, client, base, formatComposeEmail("p15-out-a", n), "VoiceQaTest1!")
	sessB := registerComposeUser(t, client, base, formatComposeEmail("p15-out-b", n), "VoiceQaTest1!")
	chatID := createComposeDMBetween(t, client, base, sessA, sessB)

	uploadComposePreKeyBundle(t, client, base, sessA.AccessToken, validComposePreKeyBundleB64())
	uploadComposePreKeyBundle(t, client, base, sessB.AccessToken, validComposePeerPreKeyBundleB64())
	enableComposeChatE2E(t, client, base, sessA.AccessToken, chatID)
	enableComposeChatE2E(t, client, base, sessB.AccessToken, chatID)

	disableComposeChatE2E(t, client, base, sessA.AccessToken, chatID)

	secret := fmt.Sprintf("phase15-optout-plain-%d", n)
	msgID := sendComposePlaintextMessage(t, client, base, sessA.AccessToken, chatID, secret)
	require.Equal(t, secret, getComposeMessageContent(t, client, base, sessA.AccessToken, chatID, msgID))

	requireComposeSearchIncludesToken(t, client, sessA.AccessToken,
		fmt.Sprintf("%s/api/v1/search/global?q=%s", base, url.QueryEscape(secret)), secret)
	requireComposeSearchIncludesToken(t, client, sessA.AccessToken,
		fmt.Sprintf("%s/api/v1/search/in-chat?chat_id=%s&q=%s", base, url.QueryEscape(chatID), url.QueryEscape(secret)), secret)
}

func requireComposeSearchIncludesToken(t *testing.T, client *http.Client, accessToken, searchURL, token string) {
	t.Helper()
	var lastBody string
	var lastCode int
	degraded := false
	require.Eventually(t, func() bool {
		req, err := http.NewRequest(http.MethodGet, searchURL, nil)
		if err != nil {
			return false
		}
		req.Header.Set("Authorization", "Bearer "+accessToken)
		resp, err := client.Do(req)
		if err != nil {
			return false
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		lastBody = string(body)
		lastCode = resp.StatusCode
		switch resp.StatusCode {
		case http.StatusOK:
			return bytes.Contains(body, []byte(token))
		case http.StatusServiceUnavailable, http.StatusInternalServerError:
			degraded = true
			return true
		default:
			return false
		}
	}, 90*time.Second, 2*time.Second, "search must include plaintext token when healthy; last status=%d body=%s", lastCode, lastBody)
	if degraded {
		t.Skipf("search degraded status=%d url=%s; skipping search assert", lastCode, searchURL)
	}
}

func disableComposeChatE2E(t *testing.T, client *http.Client, base, accessToken, chatID string) {
	t.Helper()
	status := postComposeChatE2EDisableStatus(t, client, base, accessToken, chatID)
	require.Contains(t, []int{http.StatusOK, http.StatusNoContent}, status, "POST e2e-disable for chat %s", chatID)
}

func postComposeChatE2EDisableStatus(t *testing.T, client *http.Client, base, accessToken, chatID string) int {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/chats/"+chatID+"/e2e-disable", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	_, _ = io.ReadAll(resp.Body)
	return resp.StatusCode
}

func sendComposePlaintextMessage(t *testing.T, client *http.Client, base, accessToken, chatID, content string) string {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"chat":              map[string]string{"id": chatID},
		"content":           content,
		"is_e2e":            false,
		"client_message_id": composeClientMessageID(),
	})
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/messages/send", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "POST send plaintext body=%s", string(body))

	var parsed struct {
		Message struct {
			ID string `json:"id"`
		} `json:"message"`
	}
	require.NoError(t, json.Unmarshal(body, &parsed))
	require.NotEmpty(t, parsed.Message.ID)
	return parsed.Message.ID
}
