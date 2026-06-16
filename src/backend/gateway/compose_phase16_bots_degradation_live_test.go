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

// TestComposePhase16BotsSlashWhenSearchDown_live ensures slash interactions work without Search.
func TestComposePhase16BotsSlashWhenSearchDown_live(t *testing.T) {
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
