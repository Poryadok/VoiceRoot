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

// TestStagingBotsWebhook_live exercises webhook slash delivery against a deployed staging stack.
//
// Opt-in:
//
//	VOICE_STAGING_API_URL=https://voice.comrade.click \
//	VOICE_STAGING_WEBHOOK_PING_URL=https://<reachable-echo>/ping \
//	go test -run TestStagingBotsWebhook_live -count=1 ./...
//
// VOICE_STAGING_WEBHOOK_PING_URL must be reachable from the staging Bot pod (not localhost).
// The endpoint should respond to Bot webhook POST with JSON body {"content":"pong"}.
func TestStagingBotsWebhook_live(t *testing.T) {
	if !liveStagingEnabled() {
		t.Skip("set VOICE_STAGING_API_URL to run against staging (default host: deploy/staging/domains.defaults)")
	}

	client := &http.Client{Timeout: 60 * time.Second}
	base := liveStagingBaseURL()
	webhookURL := liveStagingWebhookPingURL(t)
	n := time.Now().UnixNano()

	sess := registerComposeUser(t, client, base, formatComposeEmail("p16-stg-webhook", n), "VoiceQaTest1!")
	spaceID := createComposeSpace(t, client, base, sess.AccessToken, fmt.Sprintf("Bots Stg Webhook %d", n), "phase16")
	chatID := createComposeGroup(t, client, base, sess.AccessToken, fmt.Sprintf("bots-stg-webhook-%d", n))
	ensureComposeGroupReadyForBot(t, client, base, sess.AccessToken, chatID, n)

	botID, botToken := registerComposeBot(t, client, base, sess.AccessToken, fmt.Sprintf("StgWebhookBot-%d", n))
	manifest := fmt.Sprintf(`name: StgWebhookBot
description: staging webhook pong
webhook_url: %s
scopes: [TEXT_CHAT_SEND_MESSAGES]
commands:
  - name: ping
    description: ping
`, webhookURL)
	manifestPayload, _ := json.Marshal(map[string]string{"manifest_yaml": manifest})
	manifestReq, err := http.NewRequest(http.MethodPost, base+"/api/v1/bots/"+botID+"/manifest", bytes.NewReader(manifestPayload))
	require.NoError(t, err)
	manifestReq.Header.Set("Authorization", "Bearer "+sess.AccessToken)
	manifestReq.Header.Set("Content-Type", "application/json")
	manifestResp, err := client.Do(manifestReq)
	require.NoError(t, err)
	manifestBody, _ := io.ReadAll(manifestResp.Body)
	manifestResp.Body.Close()
	require.Equal(t, http.StatusOK, manifestResp.StatusCode, string(manifestBody))
	installComposeBot(t, client, base, sess.AccessToken, botID, spaceID, chatID)

	auth := "Bot " + botToken
	presenceReq, err := http.NewRequest(http.MethodPost, base+"/api/v1/bots/me/presence", http.NoBody)
	require.NoError(t, err)
	presenceReq.Header.Set("Authorization", auth)
	presenceResp, err := client.Do(presenceReq)
	require.NoError(t, err)
	presenceBody, _ := io.ReadAll(presenceResp.Body)
	presenceResp.Body.Close()
	require.Equal(t, http.StatusNoContent, presenceResp.StatusCode, string(presenceBody))

	payload, _ := json.Marshal(map[string]any{
		"chat":         map[string]string{"id": chatID, "type": "CHAT_TYPE_GROUP"},
		"bot_id":       botID,
		"command_name": "ping",
		"options_json": "{}",
	})
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/bots/interactions", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+sess.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "slash interaction body=%s", string(body))
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(body, &parsed))
	content, _ := parsed["content"].(string)
	require.Equal(t, "pong", content)

	assertComposeMessageInHistory(t, client, base, sess.AccessToken, chatID, "pong")
}
