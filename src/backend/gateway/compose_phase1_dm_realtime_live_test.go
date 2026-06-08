package main

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestComposePhase1DMRealtime_live mirrors phase1_two_users_e2e_live_test.dart on the Go/Gateway path.
func TestComposePhase1DMRealtime_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()

	n := time.Now().UnixNano()
	sessA := registerComposeUser(t, client, base, formatComposeEmail("p1-rt-a", n), "VoiceQaTest1!")
	sessB := registerComposeUser(t, client, base, formatComposeEmail("p1-rt-b", n), "VoiceQaTest1!")
	chatID := createComposeDM(t, client, base, sessA.AccessToken, sessB.ProfileID)

	wsB := connectComposeWSSubscribed(t, base, sessB.AccessToken, chatID)

	const firstContent = "phase1-go-e2e-first"
	msg1Ch := make(chan composeWSFrame, 1)
	go func() {
		msg1Ch <- waitComposeWSOp(t, wsB, "message_create", 20*time.Second, func(d map[string]any) bool {
			return d["chat_id"] == chatID
		})
	}()
	msg1ID := sendComposeMessage(t, client, base, sessA.AccessToken, chatID, firstContent)

	select {
	case frame := <-msg1Ch:
		var d map[string]any
		require.NoError(t, json.Unmarshal(frame.D, &d))
		require.Equal(t, chatID, d["chat_id"])
		require.Equal(t, msg1ID, d["message_id"])
	case <-time.After(25 * time.Second):
		t.Fatal("timeout waiting for message_create")
	}

	oldAccess := sessB.AccessToken
	sessB = refreshComposeSession(t, client, base, sessB.RefreshToken)
	require.NotEqual(t, oldAccess, sessB.AccessToken)

	_ = wsB.Close()
	wsB = connectComposeWSSubscribed(t, base, sessB.AccessToken, chatID)

	getComposeMessagesContains(t, client, base, sessB.AccessToken, chatID, msg1ID, firstContent)

	const secondContent = "phase1-go-e2e-after-refresh"
	msg2Ch := make(chan composeWSFrame, 1)
	go func() {
		msg2Ch <- waitComposeWSOp(t, wsB, "message_create", 20*time.Second, func(d map[string]any) bool {
			return d["chat_id"] == chatID
		})
	}()
	msg2ID := sendComposeMessage(t, client, base, sessA.AccessToken, chatID, secondContent)

	select {
	case frame := <-msg2Ch:
		var d map[string]any
		require.NoError(t, json.Unmarshal(frame.D, &d))
		require.Equal(t, msg2ID, d["message_id"])
	case <-time.After(25 * time.Second):
		t.Fatal("timeout waiting for second message_create")
	}

	markReadComposeMessage(t, client, base, sessB.AccessToken, chatID, msg2ID)
	lastRead := getComposeReadState(t, client, base, sessB.AccessToken, chatID)
	require.Equal(t, msg2ID, lastRead)

	wsB2 := connectComposeWSSubscribed(t, base, sessB.AccessToken, chatID)
	composeWSSend(t, wsB2, map[string]any{
		"op": "mark_read",
		"d": map[string]string{
			"chat_id":    chatID,
			"message_id": msg2ID,
		},
	})
	frame := waitComposeWSOp(t, wsB, "mark_read", 15*time.Second, func(d map[string]any) bool {
		return d["chat_id"] == chatID && d["message_id"] == msg2ID
	})
	var markData map[string]any
	require.NoError(t, json.Unmarshal(frame.D, &markData))
	require.Equal(t, msg2ID, markData["message_id"])
}
