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

// TestComposeBotsSlashWhenSearchDown_live ensures slash interactions work without Search.
func TestComposeBotsSlashWhenSearchDown_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	if os.Getenv("VOICE_SEARCH_DEGRADATION_TEST") != "true" {
		t.Skip("set VOICE_SEARCH_DEGRADATION_TEST=true with search container stopped")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()
	n := time.Now().UnixNano()

	sess := registerComposeUser(t, client, base, formatComposeEmail("p16-deg-slash", n), "VoiceQaTest1!")
	spaceID := createComposeSpace(t, client, base, sess.AccessToken, fmt.Sprintf("Bots Deg Slash %d", n), "phase16")
	chatID := createComposeGroup(t, client, base, sess.AccessToken, fmt.Sprintf("bots-deg-%d", n))
	ensureComposeGroupReadyForBot(t, client, base, sess.AccessToken, chatID, n)

	botID, botToken := registerComposeBot(t, client, base, sess.AccessToken, fmt.Sprintf("DegPingBot-%d", n))
	applyComposeBotManifestPolling(t, client, base, sess.AccessToken, botID)
	installComposeBot(t, client, base, sess.AccessToken, botID, spaceID, chatID)

	done := make(chan struct{})
	go composePollingPingBot(client, base, botToken, done)
	defer close(done)
	waitComposeBotOnline(t, client, base, sess.AccessToken, chatID, 15*time.Second)

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
	require.Equal(t, http.StatusOK, resp.StatusCode,
		"slash must work when Search is down, body=%s", string(body))

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(body, &parsed))
	content, _ := parsed["content"].(string)
	require.Equal(t, "pong", content)
	assertComposeMessageInHistory(t, client, base, sess.AccessToken, chatID, "pong")
}

// TestComposeBotsBotCWhenBotDown_live verifies BOT-C routes degrade when bot upstream is absent.
func TestComposeBotsBotCWhenBotDown_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	if os.Getenv("VOICE_BOT_DEGRADATION_TEST") != "true" {
		t.Skip("set VOICE_BOT_DEGRADATION_TEST=true with bot container stopped")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 15 * time.Second}
	base := liveGatewayBaseURL()
	n := time.Now().UnixNano()

	sess := registerComposeUser(t, client, base, formatComposeEmail("p16-deg-botc", n), "VoiceQaTest1!")
	spaceID := createComposeSpace(t, client, base, sess.AccessToken, fmt.Sprintf("Bots Deg BOT-C %d", n), "phase16")
	chatID := createComposeGroup(t, client, base, sess.AccessToken, fmt.Sprintf("bots-deg-botc-%d", n))
	ensureComposeGroupReadyForBot(t, client, base, sess.AccessToken, chatID, n)

	botID, botToken := registerComposeBotWithScopes(t, client, base, sess.AccessToken,
		fmt.Sprintf("DegBotC-%d", n),
		`["TEXT_CHAT_SEND_MESSAGES","SPACE_VIEW_MEMBER_LIST","TEXT_CHAT_CREATE_IN_SPACE"]`)
	applyComposeBotManifestPollingScopes(t, client, base, sess.AccessToken, botID,
		"TEXT_CHAT_SEND_MESSAGES, SPACE_VIEW_MEMBER_LIST, TEXT_CHAT_CREATE_IN_SPACE")
	installComposeBot(t, client, base, sess.AccessToken, botID, spaceID, chatID)

	auth := "Bot " + botToken
	presenceReq, err := http.NewRequest(http.MethodPost, base+"/api/v1/bots/me/presence", http.NoBody)
	require.NoError(t, err)
	presenceReq.Header.Set("Authorization", auth)
	presenceResp, err := client.Do(presenceReq)
	require.NoError(t, err)
	presenceBody, _ := io.ReadAll(presenceResp.Body)
	presenceResp.Body.Close()
	require.Equal(t, http.StatusServiceUnavailable, presenceResp.StatusCode,
		"presence must return 503 when bot service is down, body=%s", string(presenceBody))

	cmdReq, err := http.NewRequest(http.MethodGet,
		base+"/api/v1/bots/commands?chat_id="+chatID+"&chat_type=CHAT_TYPE_GROUP", nil)
	require.NoError(t, err)
	cmdReq.Header.Set("Authorization", "Bearer "+sess.AccessToken)
	cmdResp, err := client.Do(cmdReq)
	require.NoError(t, err)
	cmdBody, _ := io.ReadAll(cmdResp.Body)
	cmdResp.Body.Close()
	require.Equal(t, http.StatusServiceUnavailable, cmdResp.StatusCode,
		"slash command list must return 503 when bot service is down, body=%s", string(cmdBody))
}
