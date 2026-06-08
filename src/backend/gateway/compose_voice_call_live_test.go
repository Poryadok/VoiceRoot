package main

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestComposeVoiceCall1to1_live exercises the production DM voice call path:
// Gateway REST → Voice gRPC → NATS voice.events → Realtime WS fanout → accept → tokens → end.
//
// Prerequisites: docker compose --profile app up (Auth, Chat, Messaging, Realtime, Voice, LiveKit, Gateway).
//
// Opt-in:
//
//	VOICE_RUN_LIVE_COMPOSE=true VOICE_API_BASE_URL=http://127.0.0.1:18080 go test -run TestComposeVoiceCall1to1_live -count=1 ./...
func TestComposeVoiceCall1to1_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()

	healthResp, err := client.Get(base + "/health")
	require.NoError(t, err)
	healthResp.Body.Close()
	require.Equal(t, http.StatusOK, healthResp.StatusCode)

	n := time.Now().UnixNano()
	sessA := registerComposeUser(t, client, base, formatComposeEmail("call-a", n), "VoiceQaTest1!")
	sessB := registerComposeUser(t, client, base, formatComposeEmail("call-b", n), "VoiceQaTest1!")

	chatID := createComposeDM(t, client, base, sessA.AccessToken, sessB.ProfileID)

	wsB := dialComposeRealtimeWS(t, base, sessB.AccessToken)
	waitComposeWSHello(t, wsB)

	incomingCh := make(chan composeWSFrame, 1)
	go func() {
		frame := waitComposeWSOp(t, wsB, "call_incoming", 20*time.Second, func(d map[string]any) bool {
			return d["callee_profile_id"] == sessB.ProfileID
		})
		incomingCh <- frame
	}()

	call := startComposeCall(t, client, base, sessA.AccessToken, chatID, sessB.ProfileID)
	require.Equal(t, "CALL_STATUS_RINGING", call.Status)
	require.Equal(t, "CALL_MEDIA_KIND_AUDIO", call.MediaKind)
	require.Equal(t, sessA.ProfileID, call.InitiatorProfileID)
	require.Equal(t, sessB.ProfileID, call.CalleeProfileID)
	require.Equal(t, chatID, call.LinkedChat.ID)
	require.NotEmpty(t, call.LivekitRoomName)

	activeA := getComposeActiveCall(t, client, base, sessA.AccessToken)
	require.NotNil(t, activeA)
	require.Equal(t, call.RoomID, activeA.RoomID)

	var incomingFrame composeWSFrame
	select {
	case incomingFrame = <-incomingCh:
	case <-time.After(25 * time.Second):
		t.Fatal("timeout waiting for WS call_incoming on callee")
	}
	var incoming map[string]any
	require.NoError(t, json.Unmarshal(incomingFrame.D, &incoming))
	require.Equal(t, call.RoomID, incoming["room_id"])
	require.Equal(t, chatID, incoming["chat_id"])
	require.Equal(t, sessA.ProfileID, incoming["initiator_profile_id"])
	require.Equal(t, call.LivekitRoomName, incoming["livekit_room_name"])

	acceptedCh := make(chan composeWSFrame, 2)
	go func() {
		frame := waitComposeWSOp(t, wsB, "call_accepted", 20*time.Second, func(d map[string]any) bool {
			return d["room_id"] == call.RoomID
		})
		acceptedCh <- frame
	}()

	accepted := acceptComposeCall(t, client, base, sessB.AccessToken, call.RoomID)
	require.Equal(t, "CALL_STATUS_ACTIVE", accepted.Status)

	var acceptedFrame composeWSFrame
	select {
	case acceptedFrame = <-acceptedCh:
	case <-time.After(25 * time.Second):
		t.Fatal("timeout waiting for WS call_accepted")
	}
	var acceptedData map[string]any
	require.NoError(t, json.Unmarshal(acceptedFrame.D, &acceptedData))
	require.Equal(t, call.RoomID, acceptedData["room_id"])
	require.Equal(t, sessB.ProfileID, acceptedData["accepted_by_profile_id"])

	tokenA := getComposeJoinToken(t, client, base, sessA.AccessToken, call.RoomID)
	tokenB := getComposeJoinToken(t, client, base, sessB.AccessToken, call.RoomID)
	require.NotEmpty(t, tokenA.LivekitURL)
	require.NotEmpty(t, tokenB.JWT)

	endedCh := make(chan composeWSFrame, 2)
	go func() {
		frame := waitComposeWSOp(t, wsB, "call_ended", 20*time.Second, func(d map[string]any) bool {
			return d["room_id"] == call.RoomID
		})
		endedCh <- frame
	}()

	endComposeCall(t, client, base, sessA.AccessToken, call.RoomID)

	select {
	case <-endedCh:
	case <-time.After(25 * time.Second):
		t.Fatal("timeout waiting for WS call_ended")
	}

	require.Nil(t, getComposeActiveCall(t, client, base, sessA.AccessToken))
	require.Nil(t, getComposeActiveCall(t, client, base, sessB.AccessToken))
}

// TestComposeVoiceCallDecline_live: callee declines → WS call_declined + call_ended for caller.
func TestComposeVoiceCallDecline_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()
	n := time.Now().UnixNano()
	sessA := registerComposeUser(t, client, base, formatComposeEmail("call-decline-a", n), "VoiceQaTest1!")
	sessB := registerComposeUser(t, client, base, formatComposeEmail("call-decline-b", n), "VoiceQaTest1!")
	chatID := createComposeDM(t, client, base, sessA.AccessToken, sessB.ProfileID)

	wsA := dialComposeRealtimeWS(t, base, sessA.AccessToken)
	waitComposeWSHello(t, wsA)

	call := startComposeCall(t, client, base, sessA.AccessToken, chatID, sessB.ProfileID)
	_ = declineComposeCall(t, client, base, sessB.AccessToken, call.RoomID)

	declinedFrame := waitComposeWSOp(t, wsA, "call_declined", 20*time.Second, func(d map[string]any) bool {
		return d["room_id"] == call.RoomID
	})
	var declinedData map[string]any
	require.NoError(t, json.Unmarshal(declinedFrame.D, &declinedData))
	require.Equal(t, call.RoomID, declinedData["room_id"])

	endedFrame := waitComposeWSOp(t, wsA, "call_ended", 20*time.Second, func(d map[string]any) bool {
		return d["room_id"] == call.RoomID
	})
	var endedData map[string]any
	require.NoError(t, json.Unmarshal(endedFrame.D, &endedData))
	require.Equal(t, call.RoomID, endedData["room_id"])

	require.Nil(t, getComposeActiveCall(t, client, base, sessA.AccessToken))
}
