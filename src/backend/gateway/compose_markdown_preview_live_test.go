package main

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestComposeMarkdownPreview_live: markdown source round-trip and stripped list preview.
func TestComposeMarkdownPreview_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()

	n := time.Now().UnixNano()
	sessA := registerComposeUser(t, client, base, formatComposeEmail("md-a", n), "VoiceQaTest1!")
	sessB := registerComposeUser(t, client, base, formatComposeEmail("md-b", n), "VoiceQaTest1!")
	chatID := createComposeDM(t, client, base, sessA.AccessToken, sessB.ProfileID)

	const body = "**bold** preview"
	msgID := sendComposeMessage(t, client, base, sessA.AccessToken, chatID, body)
	getComposeMessagesContains(t, client, base, sessB.AccessToken, chatID, msgID, body)

	list := listComposeChats(t, client, base, sessB.AccessToken, "main")
	var preview string
	for _, item := range list {
		if item.ChatID == chatID {
			preview = item.LastPreview
			break
		}
	}
	require.Equal(t, "bold preview", preview)
}
