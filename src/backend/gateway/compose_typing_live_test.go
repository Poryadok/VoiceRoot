package main

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestComposeTyping_live: typing_start fans out typing event to subscribed peer.
func TestComposeTyping_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()

	n := time.Now().UnixNano()
	sessA := registerComposeUser(t, client, base, formatComposeEmail("typing-a", n), "VoiceQaTest1!")
	sessB := registerComposeUser(t, client, base, formatComposeEmail("typing-b", n), "VoiceQaTest1!")
	chatID := createComposeDM(t, client, base, sessA.AccessToken, sessB.ProfileID)

	wsA := connectComposeWSSubscribed(t, base, sessA.AccessToken, chatID)
	wsB := connectComposeWSSubscribed(t, base, sessB.AccessToken, chatID)

	typingCh := make(chan composeWSFrame, 1)
	go func() {
		typingCh <- waitComposeWSOp(t, wsB, "typing", 20*time.Second, func(d map[string]any) bool {
			return d["chat_id"] == chatID && d["profile_id"] == sessA.ProfileID
		})
	}()

	composeWSSend(t, wsA, map[string]any{
		"op": "typing_start",
		"d":  map[string]string{"chat_id": chatID},
	})

	select {
	case frame := <-typingCh:
		var d map[string]any
		require.NoError(t, json.Unmarshal(frame.D, &d))
		require.Equal(t, "start", d["kind"])
	case <-time.After(25 * time.Second):
		t.Fatal("timeout waiting for typing event")
	}
}
