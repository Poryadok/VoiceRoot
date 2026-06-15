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
	manifest := fmt.Sprintf(`name: PingBot
description: pong
scopes: [TEXT_CHAT_SEND_MESSAGES]
commands:
  - name: ping
    description: ping
`)
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

func installComposeBot(t *testing.T, client *http.Client, base, token, botID, spaceID, chatID string) {
	t.Helper()
	payload, _ := json.Marshal(map[string]any{
		"allowed_chats": []map[string]string{{"id": chatID, "type": "CHAT_TYPE_GROUP"}},
	})
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
