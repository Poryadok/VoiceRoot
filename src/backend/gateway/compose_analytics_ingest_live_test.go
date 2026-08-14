package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestComposeAnalyticsIngest_live verifies message.sent reaches ClickHouse within 60s (analytics DoD §1).
func TestComposeAnalyticsIngest_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()
	since := time.Now().UTC().Add(-2 * time.Minute)
	baseline := clickhouseEventCount(t, "message_sent", since)

	n := time.Now().UnixNano()
	emailA := formatComposeEmail("analytics-ingest-a", n)
	emailB := formatComposeEmail("analytics-ingest-b", n+1)
	const password = "VoiceQaTest1!"
	sessA := registerComposeUser(t, client, base, emailA, password)
	sessB := registerComposeUser(t, client, base, emailB, password)

	dmPayload, err := json.Marshal(map[string]string{"other_profile_id": sessB.ProfileID})
	require.NoError(t, err)
	dmReq, err := http.NewRequest(http.MethodPost, base+"/api/v1/chats/dm", bytes.NewReader(dmPayload))
	require.NoError(t, err)
	dmReq.Header.Set("Authorization", "Bearer "+sessA.AccessToken)
	dmReq.Header.Set("Content-Type", "application/json")
	dmResp, err := client.Do(dmReq)
	require.NoError(t, err)
	defer dmResp.Body.Close()
	dmBody, _ := io.ReadAll(dmResp.Body)
	require.Equal(t, http.StatusOK, dmResp.StatusCode, "dm create: %s", string(dmBody))

	var dmParsed struct {
		Chat struct {
			ID string `json:"id"`
		} `json:"chat"`
	}
	require.NoError(t, json.Unmarshal(dmBody, &dmParsed))
	require.NotEmpty(t, dmParsed.Chat.ID)

	marker := fmt.Sprintf("analytics-ingest-%d", n)
	sendPayload, err := json.Marshal(map[string]any{
		"chat":    map[string]string{"id": dmParsed.Chat.ID},
		"content": marker,
	})
	require.NoError(t, err)
	sendReq, err := http.NewRequest(http.MethodPost, base+"/api/v1/messages/send", bytes.NewReader(sendPayload))
	require.NoError(t, err)
	sendReq.Header.Set("Authorization", "Bearer "+sessA.AccessToken)
	sendReq.Header.Set("Content-Type", "application/json")
	sendResp, err := client.Do(sendReq)
	require.NoError(t, err)
	defer sendResp.Body.Close()
	sendBody, _ := io.ReadAll(sendResp.Body)
	require.Equal(t, http.StatusOK, sendResp.StatusCode, "send: %s", string(sendBody))

	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		if clickhouseEventCount(t, "message_sent", since) > baseline {
			return
		}
		time.Sleep(2 * time.Second)
	}
	t.Fatalf("message_sent not observed in ClickHouse within 60s (baseline=%d)", baseline)
}

func clickhouseHTTPBase() string {
	if u := strings.TrimSpace(os.Getenv("CLICKHOUSE_HTTP_URL")); u != "" {
		return strings.TrimRight(u, "/")
	}
	port := strings.TrimSpace(os.Getenv("CLICKHOUSE_HTTP_PORT"))
	if port == "" {
		port = "8123"
	}
	return "http://127.0.0.1:" + port
}

func clickhouseEventCount(t *testing.T, eventType string, since time.Time) uint64 {
	t.Helper()
	pass := strings.TrimSpace(os.Getenv("CLICKHOUSE_PASSWORD"))
	if pass == "" {
		pass = "voice-clickhouse-dev"
	}
	q := fmt.Sprintf(
		"SELECT count() FROM voice.events WHERE event_type = '%s' AND timestamp >= toDateTime('%s') FORMAT JSON",
		eventType,
		since.UTC().Format("2006-01-02 15:04:05"),
	)
	reqURL := clickhouseHTTPBase() + "/?" + url.Values{
		"user":     {"default"},
		"password": {pass},
		"query":    {q},
	}.Encode()
	resp, err := http.Get(reqURL)
	if err != nil {
		t.Skipf("clickhouse HTTP unavailable at %s: %v", clickhouseHTTPBase(), err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Skipf("clickhouse query failed: status=%d body=%s", resp.StatusCode, string(body))
	}
	var parsed struct {
		Data []struct {
			Count uint64 `json:"count()"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &parsed))
	if len(parsed.Data) == 0 {
		return 0
	}
	return parsed.Data[0].Count
}
