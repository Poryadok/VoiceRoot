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

// TestComposePhase16BotsSlash_live registers a polling bot and verifies /ping returns pong in channel history.
func TestComposePhase16BotsSlash_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()
	n := time.Now().UnixNano()

	sessB := registerComposeUser(t, client, base, formatComposeEmail("p16-member-b", n), "VoiceQaTest1!")
	sessC := registerComposeUser(t, client, base, formatComposeEmail("p16-member-c", n), "VoiceQaTest1!")
	sess := registerComposeUser(t, client, base, formatComposeEmail("p16-owner", n), "VoiceQaTest1!")
	spaceID := createComposeSpace(t, client, base, sess.AccessToken, fmt.Sprintf("Bots E2E %d", n), "phase16")
	chatID := createComposeGroup(t, client, base, sess.AccessToken, fmt.Sprintf("bots-%d", n))
	addComposeGroupMembers(t, client, base, sess.AccessToken, chatID, sessB.ProfileID, sessC.ProfileID)
	require.NotEmpty(t, chatID, "expected group chat id")

	botID, botToken := registerComposeBot(t, client, base, sess.AccessToken, fmt.Sprintf("PingBot-%d", n))
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
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(body, &parsed))
	content, _ := parsed["content"].(string)
	require.Equal(t, "pong", content)
	msg, ok := parsed["message"].(map[string]any)
	require.True(t, ok, "expected persisted message in response, body=%s", string(body))
	require.NotNil(t, msg)
	msgContent, _ := msg["content"].(string)
	require.Equal(t, "pong", msgContent)

	assertComposeMessageInHistory(t, client, base, sess.AccessToken, chatID, "pong")
}

// TestComposePhase16BotsSlashDeferred_live verifies defer then async SendBotMessage follow-up.
func TestComposePhase16BotsSlashDeferred_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()
	n := time.Now().UnixNano()

	sessB := registerComposeUser(t, client, base, formatComposeEmail("p16-defer-b", n), "VoiceQaTest1!")
	sessC := registerComposeUser(t, client, base, formatComposeEmail("p16-defer-c", n), "VoiceQaTest1!")
	sess := registerComposeUser(t, client, base, formatComposeEmail("p16-defer", n), "VoiceQaTest1!")
	spaceID := createComposeSpace(t, client, base, sess.AccessToken, fmt.Sprintf("Bots Defer %d", n), "phase16")
	chatID := createComposeGroup(t, client, base, sess.AccessToken, fmt.Sprintf("bots-defer-%d", n))
	addComposeGroupMembers(t, client, base, sess.AccessToken, chatID, sessB.ProfileID, sessC.ProfileID)

	botID, botToken := registerComposeBot(t, client, base, sess.AccessToken, fmt.Sprintf("DeferBot-%d", n))
	applyComposeBotManifestPolling(t, client, base, sess.AccessToken, botID)
	installComposeBot(t, client, base, sess.AccessToken, botID, spaceID, chatID)

	done := make(chan struct{})
	go composePollingDeferBot(client, base, botToken, chatID, done)
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
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(body, &parsed))
	deferred, _ := parsed["deferred"].(bool)
	require.True(t, deferred, "expected deferred response, body=%s", string(body))

	time.Sleep(2 * time.Second)
	assertComposeMessageInHistory(t, client, base, sess.AccessToken, chatID, "pong-deferred")
}

// TestComposePhase16BotsOfflineGreyout_live lists slash commands after install without polling;
// bot must appear offline (online:false) until presence heartbeat.
func TestComposePhase16BotsOfflineGreyout_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()
	n := time.Now().UnixNano()

	sess := registerComposeUser(t, client, base, formatComposeEmail("p16-greyout", n), "VoiceQaTest1!")
	spaceID := createComposeSpace(t, client, base, sess.AccessToken, fmt.Sprintf("Bots Greyout %d", n), "phase16")
	chatID := createComposeGroup(t, client, base, sess.AccessToken, fmt.Sprintf("bots-greyout-%d", n))
	ensureComposeGroupReadyForBot(t, client, base, sess.AccessToken, chatID, n)

	botID, _ := registerComposeBot(t, client, base, sess.AccessToken, fmt.Sprintf("GreyoutBot-%d", n))
	applyComposeBotManifestPolling(t, client, base, sess.AccessToken, botID)
	installComposeBot(t, client, base, sess.AccessToken, botID, spaceID, chatID)

	req, err := http.NewRequest(http.MethodGet,
		base+"/api/v1/bots/commands?chat_id="+chatID+"&chat_type=CHAT_TYPE_GROUP", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+sess.AccessToken)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "list commands body=%s", string(body))

	var parsed struct {
		Commands []struct {
			Name   string `json:"name"`
			Online bool   `json:"online"`
		} `json:"commands"`
	}
	require.NoError(t, json.Unmarshal(body, &parsed))
	require.NotEmpty(t, parsed.Commands, "expected slash commands after install")
	require.False(t, parsed.Commands[0].Online,
		"bot without poll/presence must be offline (online:false), body=%s", string(body))
}

// TestComposePhase16BotsPerChatToggle_live disables a bot in one chat via REST PATCH
// while slash commands remain available in another whitelisted chat (BOT-B).
func TestComposePhase16BotsPerChatToggle_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()
	n := time.Now().UnixNano()

	sess := registerComposeUser(t, client, base, formatComposeEmail("p16-toggle", n), "VoiceQaTest1!")
	spaceID := createComposeSpace(t, client, base, sess.AccessToken, fmt.Sprintf("Bots Toggle %d", n), "phase16")
	chatA := createComposeGroup(t, client, base, sess.AccessToken, fmt.Sprintf("bots-toggle-a-%d", n))
	chatB := createComposeGroup(t, client, base, sess.AccessToken, fmt.Sprintf("bots-toggle-b-%d", n))
	ensureComposeGroupReadyForBot(t, client, base, sess.AccessToken, chatA, n+1)
	ensureComposeGroupReadyForBot(t, client, base, sess.AccessToken, chatB, n+2)

	botID, botToken := registerComposeBot(t, client, base, sess.AccessToken, fmt.Sprintf("ToggleBot-%d", n))
	applyComposeBotManifestPolling(t, client, base, sess.AccessToken, botID)
	installComposeBotChats(t, client, base, sess.AccessToken, botID, spaceID, chatA, chatB)

	done := make(chan struct{})
	go composePollingPingBot(client, base, botToken, done)
	defer close(done)

	commandsA := listComposeSlashCommands(t, client, base, sess.AccessToken, chatA)
	require.NotEmpty(t, commandsA, "chat A must list slash commands after install")
	require.Equal(t, "ping", commandsA[0].Name)

	commandsB := listComposeSlashCommands(t, client, base, sess.AccessToken, chatB)
	require.NotEmpty(t, commandsB, "chat B must list slash commands after install")

	setComposeBotChatEnabled(t, client, base, sess.AccessToken, botID, spaceID, chatA, false)

	commandsADisabled := listComposeSlashCommands(t, client, base, sess.AccessToken, chatA)
	require.Empty(t, commandsADisabled,
		"disabled bot must not appear in /-menu for that chat (BOT-B)")

	commandsBStill := listComposeSlashCommands(t, client, base, sess.AccessToken, chatB)
	require.NotEmpty(t, commandsBStill,
		"bot disabled in chat A must remain available in chat B")

	inChatA := listComposeBotsInChat(t, client, base, sess.AccessToken, spaceID, chatA)
	require.Len(t, inChatA, 1)
	require.False(t, inChatA[0].Enabled, "per-chat toggle must report enabled:false")
	require.True(t, inChatA[0].Whitelisted)

	setComposeBotChatEnabled(t, client, base, sess.AccessToken, botID, spaceID, chatA, true)
	commandsAReenabled := listComposeSlashCommands(t, client, base, sess.AccessToken, chatA)
	require.NotEmpty(t, commandsAReenabled, "re-enabling bot must restore slash commands")
}

// TestComposePhase16BotsPrivilegedInstall_live rejects installing a history bot without owner ack.
func TestComposePhase16BotsPrivilegedInstall_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()
	n := time.Now().UnixNano()

	sess := registerComposeUser(t, client, base, formatComposeEmail("p16-priv", n), "VoiceQaTest1!")
	spaceID := createComposeSpace(t, client, base, sess.AccessToken, fmt.Sprintf("Bots Priv %d", n), "phase16")
	chatID := createComposeGroup(t, client, base, sess.AccessToken, fmt.Sprintf("bots-priv-%d", n))

	botID := registerComposePrivilegedBot(t, client, base, sess.AccessToken, fmt.Sprintf("HistoryBot-%d", n))
	applyComposeBotManifestPolling(t, client, base, sess.AccessToken, botID)

	payload, _ := json.Marshal(map[string]any{
		"allowed_chats": []map[string]string{{"id": chatID, "type": "CHAT_TYPE_GROUP"}},
	})
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/bots/"+botID+"/spaces/"+spaceID+"/install", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+sess.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.NotEqual(t, http.StatusOK, resp.StatusCode,
		"install without acknowledge_privileged_scopes must fail, body=%s", string(body))
}

// TestComposePhase16BotsBotCRoutes_live exercises BOT-C bot-token REST routes:
// presence heartbeat, space member list, and create chat in space.
func TestComposePhase16BotsBotCRoutes_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()
	n := time.Now().UnixNano()

	sess := registerComposeUser(t, client, base, formatComposeEmail("p16-botc", n), "VoiceQaTest1!")
	spaceID := createComposeSpace(t, client, base, sess.AccessToken, fmt.Sprintf("Bots BOT-C %d", n), "phase16")
	chatID := createComposeGroup(t, client, base, sess.AccessToken, fmt.Sprintf("bots-botc-%d", n))
	ensureComposeGroupReadyForBot(t, client, base, sess.AccessToken, chatID, n)

	botID, botToken := registerComposeBotWithScopes(t, client, base, sess.AccessToken,
		fmt.Sprintf("BotCRoutes-%d", n),
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
	require.Equal(t, http.StatusNoContent, presenceResp.StatusCode,
		"POST /api/v1/bots/me/presence body=%s", string(presenceBody))

	commands := listComposeSlashCommands(t, client, base, sess.AccessToken, chatID)
	require.NotEmpty(t, commands, "slash commands must be listed after install")
	require.True(t, commands[0].Online,
		"bot must report online after presence heartbeat (BOT-C)")

	membersReq, err := http.NewRequest(http.MethodGet,
		base+"/api/v1/bots/me/spaces/"+spaceID+"/members", nil)
	require.NoError(t, err)
	membersReq.Header.Set("Authorization", auth)
	membersResp, err := client.Do(membersReq)
	require.NoError(t, err)
	membersBody, _ := io.ReadAll(membersResp.Body)
	membersResp.Body.Close()
	require.Equal(t, http.StatusOK, membersResp.StatusCode,
		"GET /api/v1/bots/me/spaces/{space_id}/members body=%s", string(membersBody))
	var membersParsed struct {
		ProfileIds []string `json:"profile_ids"`
	}
	require.NoError(t, json.Unmarshal(membersBody, &membersParsed))
	require.Contains(t, membersParsed.ProfileIds, sess.ProfileID,
		"space member list must include space owner profile (BOT-C)")

	createChatPayload, _ := json.Marshal(map[string]any{
		"space_id":  spaceID,
		"name":      fmt.Sprintf("bot-created-%d", n),
		"chat_type": "channel",
	})
	createChatReq, err := http.NewRequest(http.MethodPost, base+"/api/v1/bots/me/chats", bytes.NewReader(createChatPayload))
	require.NoError(t, err)
	createChatReq.Header.Set("Authorization", auth)
	createChatReq.Header.Set("Content-Type", "application/json")
	createChatResp, err := client.Do(createChatReq)
	require.NoError(t, err)
	createChatBody, _ := io.ReadAll(createChatResp.Body)
	createChatResp.Body.Close()
	require.Equal(t, http.StatusOK, createChatResp.StatusCode,
		"POST /api/v1/bots/me/chats body=%s", string(createChatBody))
	var createChatParsed struct {
		Chat struct {
			ID string `json:"id"`
		} `json:"chat"`
	}
	require.NoError(t, json.Unmarshal(createChatBody, &createChatParsed))
	require.NotEmpty(t, createChatParsed.Chat.ID, "create chat must return linked chat id (BOT-C)")
}

func registerComposeBotWithScopes(t *testing.T, client *http.Client, base, token, name, scopesJSON string) (botID, botToken string) {
	t.Helper()
	payload, _ := json.Marshal(map[string]string{
		"name":        name,
		"description": "BOT-C routes bot",
		"scopes_json": scopesJSON,
	})
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/bots", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, string(body))
	var parsed struct {
		Bot struct {
			ID string `json:"id"`
		} `json:"bot"`
		TokenResponse struct {
			Token string `json:"token"`
		} `json:"tokenResponse"`
	}
	require.NoError(t, json.Unmarshal(body, &parsed))
	botID = parsed.Bot.ID
	require.NotEmpty(t, botID)
	if tok := strings.TrimSpace(parsed.TokenResponse.Token); tok != "" {
		return botID, tok
	}
	regenReq, err := http.NewRequest(http.MethodPost, base+"/api/v1/bots/"+botID+"/token/regenerate", nil)
	require.NoError(t, err)
	regenReq.Header.Set("Authorization", "Bearer "+token)
	regenResp, err := client.Do(regenReq)
	require.NoError(t, err)
	defer regenResp.Body.Close()
	regenBody, _ := io.ReadAll(regenResp.Body)
	require.Equal(t, http.StatusOK, regenResp.StatusCode, string(regenBody))
	var tokenParsed struct {
		TokenResponse struct {
			Token string `json:"token"`
		} `json:"token_response"`
	}
	require.NoError(t, json.Unmarshal(regenBody, &tokenParsed))
	return botID, tokenParsed.TokenResponse.Token
}

func registerComposePrivilegedBot(t *testing.T, client *http.Client, base, token, name string) string {
	t.Helper()
	payload, _ := json.Marshal(map[string]string{
		"name":        name,
		"description": "history bot",
		"scopes_json": `["TEXT_CHAT_SEND_MESSAGES","TEXT_CHAT_READ_HISTORY"]`,
	})
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/bots", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, string(body))
	var parsed struct {
		Bot struct {
			ID string `json:"id"`
		} `json:"bot"`
	}
	require.NoError(t, json.Unmarshal(body, &parsed))
	require.NotEmpty(t, parsed.Bot.ID)
	return parsed.Bot.ID
}

func createComposeSpaceChatLinkedID(t *testing.T, client *http.Client, base, accessToken, spaceID, name string) string {
	t.Helper()
	payload, err := json.Marshal(map[string]string{
		"type": "CHAT_TYPE_GROUP",
		"name": name,
	})
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/spaces/"+spaceID+"/chats", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "POST space chat body=%s", string(body))
	var parsed struct {
		SpaceTreeNode struct {
			LinkedChat struct {
				ID string `json:"id"`
			} `json:"linked_chat"`
		} `json:"space_tree_node"`
	}
	require.NoError(t, json.Unmarshal(body, &parsed))
	return parsed.SpaceTreeNode.LinkedChat.ID
}

func registerComposeBot(t *testing.T, client *http.Client, base, token, name string) (botID, botToken string) {
	t.Helper()
	payload, _ := json.Marshal(map[string]string{
		"name":        name,
		"description": "pong bot",
		"scopes_json": `["TEXT_CHAT_SEND_MESSAGES"]`,
	})
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/bots", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, string(body))
	var parsed struct {
		Bot struct {
			ID string `json:"id"`
		} `json:"bot"`
		TokenResponse struct {
			Token string `json:"token"`
		} `json:"tokenResponse"`
	}
	require.NoError(t, json.Unmarshal(body, &parsed))
	botID = parsed.Bot.ID
	require.NotEmpty(t, botID)
	if tok := strings.TrimSpace(parsed.TokenResponse.Token); tok != "" {
		return botID, tok
	}

	regenReq, err := http.NewRequest(http.MethodPost, base+"/api/v1/bots/"+botID+"/token/regenerate", nil)
	require.NoError(t, err)
	regenReq.Header.Set("Authorization", "Bearer "+token)
	regenResp, err := client.Do(regenReq)
	require.NoError(t, err)
	defer regenResp.Body.Close()
	regenBody, _ := io.ReadAll(regenResp.Body)
	require.Equal(t, http.StatusOK, regenResp.StatusCode, string(regenBody))
	var tokenParsed struct {
		TokenResponse struct {
			Token string `json:"token"`
		} `json:"token_response"`
	}
	require.NoError(t, json.Unmarshal(regenBody, &tokenParsed))
	return botID, tokenParsed.TokenResponse.Token
}

func applyComposeBotManifestPolling(t *testing.T, client *http.Client, base, token, botID string) {
	t.Helper()
	applyComposeBotManifestPollingScopes(t, client, base, token, botID, "TEXT_CHAT_SEND_MESSAGES")
}

func applyComposeBotManifestPollingScopes(t *testing.T, client *http.Client, base, token, botID, scopesYAML string) {
	t.Helper()
	manifest := fmt.Sprintf(`name: PingBot
description: pong
scopes: [%s]
commands:
  - name: ping
    description: ping
`, scopesYAML)
	payload, _ := json.Marshal(map[string]string{"manifest_yaml": manifest})
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/bots/"+botID+"/manifest", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, string(body))
}

type composeSlashCommand struct {
	Name   string
	Online bool
}

type composeChatBotEntry struct {
	Enabled     bool
	Whitelisted bool
}

func listComposeSlashCommands(t *testing.T, client *http.Client, base, token, chatID string) []composeSlashCommand {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet,
		base+"/api/v1/bots/commands?chat_id="+chatID+"&chat_type=CHAT_TYPE_GROUP", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "list commands body=%s", string(body))
	var parsed struct {
		Commands []composeSlashCommand `json:"commands"`
	}
	require.NoError(t, json.Unmarshal(body, &parsed))
	return parsed.Commands
}

func listComposeBotsInChat(t *testing.T, client *http.Client, base, token, spaceID, chatID string) []composeChatBotEntry {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet,
		base+"/api/v1/bots/chats/"+chatID+"?space_id="+spaceID+"&chat_type=CHAT_TYPE_GROUP", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "list bots in chat body=%s", string(body))
	var parsed struct {
		Bots []struct {
			Enabled     bool `json:"enabled"`
			Whitelisted bool `json:"whitelisted"`
		} `json:"bots"`
	}
	require.NoError(t, json.Unmarshal(body, &parsed))
	out := make([]composeChatBotEntry, 0, len(parsed.Bots))
	for _, b := range parsed.Bots {
		out = append(out, composeChatBotEntry{
			Enabled:     b.Enabled,
			Whitelisted: b.Whitelisted,
		})
	}
	return out
}

func setComposeBotChatEnabled(t *testing.T, client *http.Client, base, token, botID, spaceID, chatID string, enabled bool) {
	t.Helper()
	payload, _ := json.Marshal(map[string]any{
		"chat":     map[string]string{"id": chatID, "type": "CHAT_TYPE_GROUP"},
		"space_id": spaceID,
		"enabled":  enabled,
	})
	req, err := http.NewRequest(http.MethodPatch,
		base+"/api/v1/bots/"+botID+"/chats/"+chatID+"/enabled", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusNoContent, resp.StatusCode,
		"set bot chat enabled body=%s", string(body))
}

func installComposeBotChats(t *testing.T, client *http.Client, base, token, botID, spaceID string, chatIDs ...string) {
	t.Helper()
	allowed := make([]map[string]string, 0, len(chatIDs))
	for _, id := range chatIDs {
		allowed = append(allowed, map[string]string{"id": id, "type": "CHAT_TYPE_GROUP"})
	}
	payload, _ := json.Marshal(map[string]any{"allowed_chats": allowed})
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/bots/"+botID+"/spaces/"+spaceID+"/install", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, string(body))
}

func installComposeBot(t *testing.T, client *http.Client, base, token, botID, spaceID, chatID string) {
	t.Helper()
	installComposeBotChats(t, client, base, token, botID, spaceID, chatID)
}

// waitComposeBotOnline polls slash commands until the bot reports online (poll loop touches presence).
func waitComposeBotOnline(t *testing.T, client *http.Client, base, token, chatID string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		req, err := http.NewRequest(http.MethodGet,
			base+"/api/v1/bots/commands?chat_id="+chatID+"&chat_type=CHAT_TYPE_GROUP", nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := client.Do(req)
		if err == nil && resp != nil {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			var parsed struct {
				Commands []struct {
					Online bool `json:"online"`
				} `json:"commands"`
			}
			if json.Unmarshal(body, &parsed) == nil && len(parsed.Commands) > 0 && parsed.Commands[0].Online {
				return
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatal("timed out waiting for bot online presence")
}

func composePollingPingBot(client *http.Client, base, botToken string, stop <-chan struct{}) {
	auth := "Bot " + botToken
	for {
		select {
		case <-stop:
			return
		default:
		}
		req, _ := http.NewRequest(http.MethodGet, base+"/api/v1/bots/me/interactions/poll", nil)
		req.Header.Set("Authorization", auth)
		resp, err := client.Do(req)
		if err != nil || resp == nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		var parsed struct {
			Events []struct {
				PayloadJSON string `json:"payload_json"`
			} `json:"events"`
		}
		_ = json.Unmarshal(body, &parsed)
		for _, evt := range parsed.Events {
			var payload map[string]any
			_ = json.Unmarshal([]byte(evt.PayloadJSON), &payload)
			tok, _ := payload["interaction_token"].(string)
			if tok == "" {
				continue
			}
			complete, _ := json.Marshal(map[string]any{
				"interaction_token": tok,
				"content":           "pong",
				"is_ephemeral":      false,
			})
			creq, _ := http.NewRequest(http.MethodPost, base+"/api/v1/bots/me/interactions/complete", bytes.NewReader(complete))
			creq.Header.Set("Authorization", auth)
			creq.Header.Set("Content-Type", "application/json")
			cresp, _ := client.Do(creq)
			if cresp != nil {
				cresp.Body.Close()
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func composePollingDeferBot(client *http.Client, base, botToken, chatID string, stop <-chan struct{}) {
	auth := "Bot " + botToken
	for {
		select {
		case <-stop:
			return
		default:
		}
		req, _ := http.NewRequest(http.MethodGet, base+"/api/v1/bots/me/interactions/poll", nil)
		req.Header.Set("Authorization", auth)
		resp, err := client.Do(req)
		if err != nil || resp == nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		var parsed struct {
			Events []struct {
				PayloadJSON string `json:"payload_json"`
			} `json:"events"`
		}
		_ = json.Unmarshal(body, &parsed)
		for _, evt := range parsed.Events {
			var payload map[string]any
			_ = json.Unmarshal([]byte(evt.PayloadJSON), &payload)
			tok, _ := payload["interaction_token"].(string)
			if tok == "" {
				continue
			}
			deferPayload, _ := json.Marshal(map[string]any{
				"interaction_token": tok,
				"deferred":          true,
			})
			dreq, _ := http.NewRequest(http.MethodPost, base+"/api/v1/bots/me/interactions/complete", bytes.NewReader(deferPayload))
			dreq.Header.Set("Authorization", auth)
			dreq.Header.Set("Content-Type", "application/json")
			dresp, _ := client.Do(dreq)
			if dresp != nil {
				dresp.Body.Close()
			}
			msgPayload, _ := json.Marshal(map[string]any{
				"chat":              map[string]string{"id": chatID, "type": "CHAT_TYPE_GROUP"},
				"content":           "pong-deferred",
				"interaction_token": tok,
			})
			mreq, _ := http.NewRequest(http.MethodPost, base+"/api/v1/bots/me/messages", bytes.NewReader(msgPayload))
			mreq.Header.Set("Authorization", auth)
			mreq.Header.Set("Content-Type", "application/json")
			mresp, _ := client.Do(mreq)
			if mresp != nil {
				mresp.Body.Close()
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func assertComposeMessageInHistory(t *testing.T, client *http.Client, base, accessToken, chatID, content string) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		req, err := http.NewRequest(http.MethodGet, base+"/api/v1/messages?chat_id="+chatID, nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+accessToken)
		resp, err := client.Do(req)
		if err != nil || resp == nil {
			time.Sleep(200 * time.Millisecond)
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			time.Sleep(200 * time.Millisecond)
			continue
		}
		var hist struct {
			MessageList struct {
				Messages []struct {
					Content string `json:"content"`
				} `json:"messages"`
			} `json:"message_list"`
		}
		if err := json.Unmarshal(body, &hist); err != nil {
			time.Sleep(200 * time.Millisecond)
			continue
		}
		for _, m := range hist.MessageList.Messages {
			if strings.TrimSpace(m.Content) == content {
				return
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("message with content %q not found in chat %s history", content, chatID)
}
