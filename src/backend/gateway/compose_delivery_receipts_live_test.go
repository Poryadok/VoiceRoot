package main

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestComposeDeliveryReceipts_live: delivery_ack notifies sender via message_delivered WS.
func TestComposeDeliveryReceipts_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()

	n := time.Now().UnixNano()
	sessA := registerComposeUser(t, client, base, formatComposeEmail("delivery-a", n), "VoiceQaTest1!")
	sessB := registerComposeUser(t, client, base, formatComposeEmail("delivery-b", n), "VoiceQaTest1!")
	chatID := createComposeDM(t, client, base, sessA.AccessToken, sessB.ProfileID)

	wsA := connectComposeWSSubscribed(t, base, sessA.AccessToken, chatID)
	wsB := connectComposeWSSubscribed(t, base, sessB.AccessToken, chatID)

	deliveredCh := make(chan composeWSFrame, 1)
	go func() {
		deliveredCh <- waitComposeWSOp(t, wsA, "message_delivered", 20*time.Second, nil)
	}()

	msgID := sendComposeMessage(t, client, base, sessA.AccessToken, chatID, "delivery-e2e")
	_ = waitComposeWSOp(t, wsB, "message_create", 20*time.Second, func(d map[string]any) bool {
		return d["message_id"] == msgID
	})

	composeWSSend(t, wsB, map[string]any{
		"op": "delivery_ack",
		"d": map[string]string{
			"chat_id":            chatID,
			"message_id":         msgID,
			"sender_profile_id":  sessA.ProfileID,
		},
	})

	select {
	case frame := <-deliveredCh:
		var d map[string]any
		require.NoError(t, json.Unmarshal(frame.D, &d))
		require.Equal(t, msgID, d["message_id"])
		require.Equal(t, chatID, d["chat_id"])
	case <-time.After(25 * time.Second):
		t.Fatal("timeout waiting for message_delivered")
	}
}
