package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestComposePhase16BotsUninstallCleanup_live verifies uninstall removes bot roles and pins but keeps messages.
func TestComposePhase16BotsUninstallCleanup_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()
	n := time.Now().UnixNano()

	sess := registerComposeUser(t, client, base, formatComposeEmail("p16-uninstall", n), "VoiceQaTest1!")
	spaceID := createComposeSpace(t, client, base, sess.AccessToken, fmt.Sprintf("Bots Uninstall %d", n), "phase16")
	chatID := createComposeGroup(t, client, base, sess.AccessToken, fmt.Sprintf("bots-uninstall-%d", n))
	ensureComposeGroupReadyForBot(t, client, base, sess.AccessToken, chatID, n)

	botID, botToken := registerComposeBot(t, client, base, sess.AccessToken, fmt.Sprintf("CleanupBot-%d", n))
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
	require.Equal(t, http.StatusOK, resp.StatusCode, "slash interaction body=%s", string(body))

	var interactionParsed map[string]any
	require.NoError(t, json.Unmarshal(body, &interactionParsed))
	msg, ok := interactionParsed["message"].(map[string]any)
	require.True(t, ok, "expected persisted message in slash response, body=%s", string(body))
	msgID, _ := msg["id"].(string)
	require.NotEmpty(t, msgID)

	assertComposeMessageInHistory(t, client, base, sess.AccessToken, chatID, "pong")

	commandsBefore := listComposeSlashCommands(t, client, base, sess.AccessToken, chatID)
	require.NotEmpty(t, commandsBefore, "slash commands must be listed before uninstall")

	pinPayload, err := json.Marshal(map[string]any{
		"chat": map[string]string{"id": chatID},
	})
	require.NoError(t, err)
	pinReq, err := http.NewRequest(http.MethodPost, base+"/api/v1/messages/"+msgID+"/pin", bytes.NewReader(pinPayload))
	require.NoError(t, err)
	pinReq.Header.Set("Authorization", "Bearer "+sess.AccessToken)
	pinReq.Header.Set("Content-Type", "application/json")
	pinResp, err := client.Do(pinReq)
	require.NoError(t, err)
	defer pinResp.Body.Close()
	require.Equal(t, http.StatusNoContent, pinResp.StatusCode, "pin bot message before uninstall")

	pinnedBefore := listComposePinnedMessages(t, client, base, sess.AccessToken, chatID)
	require.Contains(t, pinnedBefore, msgID, "bot message must be pinned before uninstall")

	uninstallReq, err := http.NewRequest(http.MethodDelete, base+"/api/v1/bots/"+botID+"/spaces/"+spaceID, nil)
	require.NoError(t, err)
	uninstallReq.Header.Set("Authorization", "Bearer "+sess.AccessToken)
	uninstallResp, err := client.Do(uninstallReq)
	require.NoError(t, err)
	defer uninstallResp.Body.Close()
	uninstallBody, _ := io.ReadAll(uninstallResp.Body)
	require.Equal(t, http.StatusNoContent, uninstallResp.StatusCode,
		"uninstall body=%s", string(uninstallBody))

	listRolesReq, err := http.NewRequest(http.MethodGet, base+"/api/v1/roles?space_id="+spaceID, nil)
	require.NoError(t, err)
	listRolesReq.Header.Set("Authorization", "Bearer "+sess.AccessToken)
	listRolesResp, err := client.Do(listRolesReq)
	require.NoError(t, err)
	defer listRolesResp.Body.Close()
	rolesBody, _ := io.ReadAll(listRolesResp.Body)
	require.Equal(t, http.StatusOK, listRolesResp.StatusCode, string(rolesBody))
	require.NotContains(t, string(rolesBody), "CleanupBot",
		"bot-created roles must be deleted on uninstall (BOT-B)")

	assertComposeMessageInHistory(t, client, base, sess.AccessToken, chatID, "pong")

	commandsAfter := listComposeSlashCommands(t, client, base, sess.AccessToken, chatID)
	require.Empty(t, commandsAfter, "slash commands must disappear after uninstall (BOT-B)")

	pinnedAfter := listComposePinnedMessages(t, client, base, sess.AccessToken, chatID)
	require.NotContains(t, pinnedAfter, msgID,
		"bot-authored pins must be removed on uninstall (BOT-B)")

	installedReq, err := http.NewRequest(http.MethodGet, base+"/api/v1/bots/spaces/"+spaceID+"/installed", nil)
	require.NoError(t, err)
	installedReq.Header.Set("Authorization", "Bearer "+sess.AccessToken)
	installedResp, err := client.Do(installedReq)
	require.NoError(t, err)
	defer installedResp.Body.Close()
	installedBody, _ := io.ReadAll(installedResp.Body)
	require.Equal(t, http.StatusOK, installedResp.StatusCode, string(installedBody))
	var installedParsed struct {
		InstalledBots []any `json:"installed_bots"`
	}
	require.NoError(t, json.Unmarshal(installedBody, &installedParsed))
	require.Empty(t, installedParsed.InstalledBots)
}

func listComposePinnedMessages(t *testing.T, client *http.Client, base, token, chatID string) []string {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, base+"/api/v1/chats/"+chatID+"/pinned-messages", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "pinned messages body=%s", string(body))
	var parsed struct {
		MessageList struct {
			Messages []struct {
				ID string `json:"id"`
			} `json:"messages"`
		} `json:"message_list"`
	}
	require.NoError(t, json.Unmarshal(body, &parsed))
	ids := make([]string, 0, len(parsed.MessageList.Messages))
	for _, m := range parsed.MessageList.Messages {
		id := strings.TrimSpace(m.ID)
		if id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}
