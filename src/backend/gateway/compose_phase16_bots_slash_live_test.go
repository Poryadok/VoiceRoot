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

// TestComposePhase16BotsSlash_live registers a polling bot and verifies /ping returns pong.
func TestComposePhase16BotsSlash_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()
	n := time.Now().UnixNano()

	sess := registerComposeUser(t, client, base, formatComposeEmail("p16-owner", n), "VoiceQaTest1!")
	spaceID := createComposeSpace(t, client, base, sess.AccessToken, fmt.Sprintf("Bots E2E %d", n), "phase16")
	chatID := createComposeSpaceChatLinkedID(t, client, base, sess.AccessToken, spaceID, "general")
	require.NotEmpty(t, chatID, "expected linked chat id from space chat create")

	botID, botToken := registerComposeBot(t, client, base, sess.AccessToken, fmt.Sprintf("PingBot-%d", n))
	applyComposeBotManifestPolling(t, client, base, sess.AccessToken, botID)
	installComposeBot(t, client, base, sess.AccessToken, botID, spaceID, chatID)

	done := make(chan struct{})
	go composePollingPingBot(client, base, botToken, done)
	defer close(done)

	payload, _ := json.Marshal(map[string]any{
		"chat":         map[string]string{"id": chatID, "type": "CHAT_TYPE_CHANNEL"},
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
	if msg, ok := parsed["message"].(map[string]any); ok && msg != nil {
		msgContent, _ := msg["content"].(string)
		require.Equal(t, "pong", msgContent)
	}
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
	}
	require.NoError(t, json.Unmarshal(body, &parsed))
	botID = parsed.Bot.ID
	require.NotEmpty(t, botID)

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
		"allowed_chats": []map[string]string{{"id": chatID, "type": "CHAT_TYPE_CHANNEL"}},
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
				"is_ephemeral":      true,
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
