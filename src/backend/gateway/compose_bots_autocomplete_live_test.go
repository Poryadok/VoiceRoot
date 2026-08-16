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

// TestComposeBotsAutocomplete_live covers BT-07: polling autocomplete round-trip +
// developer-portal catalog APIs (GET commands / GET manifest) with subcommands.
func TestComposeBotsAutocomplete_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()
	n := time.Now().UnixNano()

	sessB := registerComposeUser(t, client, base, formatComposeEmail("bt07-member-b", n), "VoiceQaTest1!")
	sessC := registerComposeUser(t, client, base, formatComposeEmail("bt07-member-c", n), "VoiceQaTest1!")
	sess := registerComposeUser(t, client, base, formatComposeEmail("bt07-owner", n), "VoiceQaTest1!")
	spaceID := createComposeSpace(t, client, base, sess.AccessToken, fmt.Sprintf("Bots AC %d", n), "bt07")
	chatID := createComposeGroup(t, client, base, sess.AccessToken, fmt.Sprintf("bots-ac-%d", n))
	addComposeGroupMembersForInvitees(t, client, base, sess.AccessToken, chatID, sessB, sessC)

	botID, botToken := registerComposeBot(t, client, base, sess.AccessToken, fmt.Sprintf("StatsBot-%d", n))
	applyComposeBotAutocompleteManifest(t, client, base, sess.AccessToken, botID)
	installComposeBot(t, client, base, sess.AccessToken, botID, spaceID, chatID)

	assertComposeBotCommandCatalog(t, client, base, sess.AccessToken, botID)
	assertComposeBotManifestRoundTrip(t, client, base, sess.AccessToken, botID)

	done := make(chan struct{})
	go composePollingAutocompleteBot(client, base, botToken, done)
	defer close(done)
	waitComposeBotOnline(t, client, base, sess.AccessToken, chatID, 15*time.Second)

	choices := waitComposeAutocompleteChoices(t, client, base, sess.AccessToken, botID, chatID, "cs", 10*time.Second)
	require.NotEmpty(t, choices, "expected autocomplete choices after bot CompleteAutocomplete")
	require.Equal(t, "CS2", choices[0].Name)
	require.Equal(t, "cs2", choices[0].Value)
}

type composeAutocompleteChoice struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func applyComposeBotAutocompleteManifest(t *testing.T, client *http.Client, base, token, botID string) {
	t.Helper()
	manifest := `name: StatsBot
description: autocomplete + subcommands
scopes: [TEXT_CHAT_SEND_MESSAGES]
commands:
  - name: stats
    description: Show player stats
    options:
      - name: game
        type: string
        required: true
        autocomplete: true
  - name: queue
    description: Queue group
    subcommands:
      - name: join
        description: Join queue
      - name: leave
        description: Leave queue
`
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

func assertComposeBotCommandCatalog(t *testing.T, client *http.Client, base, token, botID string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, base+"/api/v1/bots/"+botID+"/commands", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "GET commands body=%s", string(body))

	var parsed struct {
		CommandList struct {
			CommandsJSON string `json:"commands_json"`
		} `json:"command_list"`
	}
	require.NoError(t, json.Unmarshal(body, &parsed))
	require.NotEmpty(t, parsed.CommandList.CommandsJSON)

	var commands []struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Options     []struct {
			Name         string `json:"name"`
			Autocomplete bool   `json:"autocomplete"`
		} `json:"options"`
	}
	require.NoError(t, json.Unmarshal([]byte(parsed.CommandList.CommandsJSON), &commands))
	require.GreaterOrEqual(t, len(commands), 3, "catalog must include stats + queue join/leave")

	var sawStats, sawJoin, sawLeave bool
	for _, c := range commands {
		switch c.Name {
		case "stats":
			sawStats = true
			require.NotEmpty(t, c.Options)
			require.True(t, c.Options[0].Autocomplete, "stats.game must be autocomplete")
		case "queue join":
			sawJoin = true
		case "queue leave":
			sawLeave = true
		}
	}
	require.True(t, sawStats && sawJoin && sawLeave, "commands_json=%s", parsed.CommandList.CommandsJSON)
}

func assertComposeBotManifestRoundTrip(t *testing.T, client *http.Client, base, token, botID string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, base+"/api/v1/bots/"+botID+"/manifest", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "GET manifest body=%s", string(body))

	var parsed struct {
		ManifestYAML string `json:"manifest_yaml"`
	}
	require.NoError(t, json.Unmarshal(body, &parsed))
	require.Contains(t, parsed.ManifestYAML, "stats")
	require.Contains(t, parsed.ManifestYAML, "autocomplete")
	require.Contains(t, parsed.ManifestYAML, "queue")
}

func waitComposeAutocompleteChoices(
	t *testing.T,
	client *http.Client,
	base, token, botID, chatID, focused string,
	timeout time.Duration,
) []composeAutocompleteChoice {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		payload, _ := json.Marshal(map[string]any{
			"chat":          map[string]string{"id": chatID, "type": "CHAT_TYPE_GROUP"},
			"bot_id":        botID,
			"command_name":  "stats",
			"option_name":   "game",
			"focused_value": focused,
			"options_json":  "{}",
		})
		req, err := http.NewRequest(http.MethodPost, base+"/api/v1/bots/autocomplete", bytes.NewReader(payload))
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		require.NoError(t, err)
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode, "autocomplete body=%s", string(body))

		var parsed struct {
			Choices []composeAutocompleteChoice `json:"choices"`
			Pending *bool                       `json:"pending"`
		}
		require.NoError(t, json.Unmarshal(body, &parsed))
		if len(parsed.Choices) > 0 {
			return parsed.Choices
		}
		if parsed.Pending != nil && !*parsed.Pending {
			t.Fatalf("autocomplete returned no choices and pending=false: %s", string(body))
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatal("timed out waiting for autocomplete choices")
	return nil
}

func composePollingAutocompleteBot(client *http.Client, base, botToken string, stop <-chan struct{}) {
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
				EventType   string `json:"event_type"`
				PayloadJSON string `json:"payload_json"`
			} `json:"events"`
		}
		_ = json.Unmarshal(body, &parsed)
		for _, evt := range parsed.Events {
			var payload map[string]any
			_ = json.Unmarshal([]byte(evt.PayloadJSON), &payload)
			typ, _ := payload["type"].(string)
			if typ != "autocomplete" && evt.EventType != "autocomplete" {
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
				continue
			}
			requestID, _ := payload["request_id"].(string)
			if requestID == "" {
				continue
			}
			complete, _ := json.Marshal(map[string]any{
				"request_id": requestID,
				"choices": []map[string]string{
					{"name": "CS2", "value": "cs2"},
					{"name": "CS:GO", "value": "csgo"},
				},
			})
			creq, _ := http.NewRequest(http.MethodPost, base+"/api/v1/bots/me/autocomplete/complete", bytes.NewReader(complete))
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
