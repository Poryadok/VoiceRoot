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

	"voice/backend/pkg/composefixture"
)

// TestComposeE2EDM_live mirrors encryption_dm_e2e_live_test.dart on the Go/Gateway path.
func TestComposeE2EDM_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()

	n := time.Now().UnixNano()
	sessA := registerComposeUser(t, client, base, formatComposeEmail("p15-a", n), "VoiceQaTest1!")
	sessB := registerComposeUser(t, client, base, formatComposeEmail("p15-b", n), "VoiceQaTest1!")
	chatID := createComposeDMBetween(t, client, base, sessA, sessB)

	uploadComposePreKeyBundle(t, client, base, sessA.AccessToken, validComposePreKeyBundleB64())
	uploadComposePreKeyBundle(t, client, base, sessB.AccessToken, validComposePeerPreKeyBundleB64())
	enableComposeChatE2E(t, client, base, sessA.AccessToken, chatID)
	enableComposeChatE2E(t, client, base, sessB.AccessToken, chatID)

	secret := fmt.Sprintf("phase15-compose-secret-%d", n)
	ciphertext := composefixture.LibsignalGoldenE2ECiphertextB64()
	require.NotEqual(t, secret, ciphertext)
	require.NotEqual(t, composefixture.E2ECiphertextGoldenPlaintext, ciphertext)

	msgID := sendComposeE2EMessage(t, client, base, sessA.AccessToken, chatID, ciphertext)
	require.NotEqual(t, secret, getComposeMessageContent(t, client, base, sessA.AccessToken, chatID, msgID))
	require.NotEqual(t, composefixture.E2ECiphertextGoldenPlaintext, getComposeMessageContent(t, client, base, sessA.AccessToken, chatID, msgID))

	globalURL := fmt.Sprintf("%s/api/v1/search/global?q=%s", base, url.QueryEscape(secret))
	requireComposeSearchExcludesToken(t, client, sessA.AccessToken, globalURL, secret)

	inChatURL := fmt.Sprintf("%s/api/v1/search/in-chat?chat_id=%s&q=%s",
		base, url.QueryEscape(chatID), url.QueryEscape(secret))
	requireComposeSearchExcludesToken(t, client, sessA.AccessToken, inChatURL, secret)
}

// TestComposeE2E_GroupEnableRejected_live documents E2E is DM-only (encryption.md).
func TestComposeE2E_GroupEnableRejected_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 30 * time.Second}
	base := liveGatewayBaseURL()

	n := time.Now().UnixNano()
	owner := registerComposeUser(t, client, base, formatComposeEmail("p15-grp-o", n), "VoiceQaTest1!")

	groupID := createComposeGroup(t, client, base, owner.AccessToken, "phase15-no-e2e")

	status := postComposeChatE2EEnableStatus(t, client, base, owner.AccessToken, groupID)
	require.NotEqual(t, http.StatusOK, status, "E2E enable must fail on group chats")
}

// TestComposeE2E_EnableRejectedWhenPeerMissingPreKey_live documents pre-key gate (Batch E2E-A).
func TestComposeE2E_EnableRejectedWhenPeerMissingPreKey_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 30 * time.Second}
	base := liveGatewayBaseURL()

	n := time.Now().UnixNano()
	sessA := registerComposeUser(t, client, base, formatComposeEmail("p15-gate-a", n), "VoiceQaTest1!")
	sessB := registerComposeUser(t, client, base, formatComposeEmail("p15-gate-b", n), "VoiceQaTest1!")
	chatID := createComposeDMBetween(t, client, base, sessA, sessB)

	uploadComposePreKeyBundle(t, client, base, sessA.AccessToken, validComposePreKeyBundleB64())

	status := postComposeChatE2EEnableStatus(t, client, base, sessA.AccessToken, chatID)
	require.NotEqual(t, http.StatusOK, status, "E2E enable must fail when peer has no pre-key bundle")
}

func uploadComposePreKeyBundle(t *testing.T, client *http.Client, base, accessToken, bundle string) {
	t.Helper()
	payload, err := json.Marshal(map[string]string{"bundle": bundle})
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/messages/prekeys", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Contains(t, []int{http.StatusOK, http.StatusNoContent}, resp.StatusCode, "POST prekeys body=%s", string(body))
}

func enableComposeChatE2E(t *testing.T, client *http.Client, base, accessToken, chatID string) {
	t.Helper()
	status := postComposeChatE2EEnableStatus(t, client, base, accessToken, chatID)
	require.Contains(t, []int{http.StatusOK, http.StatusNoContent}, status, "POST e2e-enable for chat %s", chatID)
}

func postComposeChatE2EEnableStatus(t *testing.T, client *http.Client, base, accessToken, chatID string) int {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/chats/"+chatID+"/e2e-enable", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	_, _ = io.ReadAll(resp.Body)
	return resp.StatusCode
}

func sendComposeE2EMessage(t *testing.T, client *http.Client, base, accessToken, chatID, content string) string {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"chat":              map[string]string{"id": chatID},
		"content":           content,
		"is_e2e":            true,
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
	require.Equal(t, http.StatusOK, resp.StatusCode, "POST send E2E body=%s", string(body))

	var parsed struct {
		Message struct {
			ID string `json:"id"`
		} `json:"message"`
	}
	require.NoError(t, json.Unmarshal(body, &parsed))
	require.NotEmpty(t, parsed.Message.ID)
	return parsed.Message.ID
}

func getComposeMessageContent(t *testing.T, client *http.Client, base, accessToken, chatID, messageID string) string {
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
				ID      string `json:"id"`
				Content string `json:"content"`
			} `json:"messages"`
		} `json:"message_list"`
	}
	require.NoError(t, json.Unmarshal(body, &hist))
	for _, m := range hist.MessageList.Messages {
		if m.ID == messageID {
			return m.Content
		}
	}
	t.Fatalf("message %s not in history: %s", messageID, string(body))
	return ""
}

func requireComposeSearchExcludesToken(t *testing.T, client *http.Client, accessToken, searchURL, token string) {
	t.Helper()
	var lastBody string
	var lastCode int
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
		if resp.StatusCode != http.StatusOK {
			return false
		}
		return !bytes.Contains(body, []byte(token))
	}, 45*time.Second, 2*time.Second, "search must exclude E2E token; last status=%d body=%s", lastCode, lastBody)
}
