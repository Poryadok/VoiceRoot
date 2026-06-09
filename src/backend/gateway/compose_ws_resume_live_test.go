package main

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestComposeWSResume_live: catch-up missed messages via REST after WS disconnect (resume is client bookkeeping).
func TestComposeWSResume_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()

	n := time.Now().UnixNano()
	sessA := registerComposeUser(t, client, base, formatComposeEmail("ws-resume-a", n), "VoiceQaTest1!")
	sessB := registerComposeUser(t, client, base, formatComposeEmail("ws-resume-b", n), "VoiceQaTest1!")
	chatID := createComposeDM(t, client, base, sessA.AccessToken, sessB.ProfileID)

	wsB := connectComposeWSSubscribed(t, base, sessB.AccessToken, chatID)
	msg1Ch := make(chan composeWSFrame, 1)
	go func() {
		msg1Ch <- waitComposeWSOp(t, wsB, "message_create", 20*time.Second, func(d map[string]any) bool {
			return d["chat_id"] == chatID
		})
	}()
	sendComposeMessage(t, client, base, sessA.AccessToken, chatID, "resume-baseline")
	<-msg1Ch
	require.NoError(t, wsB.Close())

	missedID := sendComposeMessage(t, client, base, sessA.AccessToken, chatID, "resume-missed")
	getComposeMessagesContains(t, client, base, sessB.AccessToken, chatID, missedID, "resume-missed")

	wsB2 := connectComposeWSSubscribed(t, base, sessB.AccessToken, chatID)
	// Resume is client bookkeeping; message catch-up after reconnect is via Messaging REST.
	composeWSSendResume(t, wsB2, 1)
	_ = wsB2.Close()
}
