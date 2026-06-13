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

// TestComposeVoiceScreenShare_live exercises REST → Voice → NATS → Realtime for screen share.
//
// Opt-in: VOICE_RUN_LIVE_COMPOSE=true VOICE_API_BASE_URL=http://127.0.0.1:18080
func TestComposeVoiceScreenShare_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()

	n := time.Now().UnixNano()
	sessA := registerComposeUser(t, client, base, formatComposeEmail("ss-a", n), "VoiceQaTest1!")
	sessB := registerComposeUser(t, client, base, formatComposeEmail("ss-b", n), "VoiceQaTest1!")
	chatID := createComposeDM(t, client, base, sessA.AccessToken, sessB.ProfileID)

	wsB := dialComposeRealtimeWS(t, base, sessB.AccessToken)
	waitComposeWSHello(t, wsB)

	call := startComposeCall(t, client, base, sessA.AccessToken, chatID, sessB.ProfileID)
	_ = acceptComposeCall(t, client, base, sessB.AccessToken, call.RoomID)

	startedCh := make(chan composeWSFrame, 1)
	go func() {
		frame := waitComposeWSOp(t, wsB, "screen_share_started", 20*time.Second, func(d map[string]any) bool {
			return d["room_id"] == call.RoomID
		})
		startedCh <- frame
	}()

	streamID := startComposeScreenShare(t, client, base, sessA.AccessToken, call.RoomID)
	require.NotEmpty(t, streamID)

	var startedFrame composeWSFrame
	select {
	case startedFrame = <-startedCh:
	case <-time.After(25 * time.Second):
		t.Fatal("timeout waiting for WS screen_share_started")
	}
	var started map[string]any
	require.NoError(t, json.Unmarshal(startedFrame.D, &started))
	require.Equal(t, call.RoomID, started["room_id"])
	require.Equal(t, sessA.ProfileID, started["profile_id"])
	require.Equal(t, streamID, started["stream_id"])

	stoppedCh := make(chan composeWSFrame, 1)
	go func() {
		frame := waitComposeWSOp(t, wsB, "screen_share_stopped", 20*time.Second, func(d map[string]any) bool {
			return d["room_id"] == call.RoomID
		})
		stoppedCh <- frame
	}()

	stopComposeScreenShare(t, client, base, sessA.AccessToken, call.RoomID, streamID)

	select {
	case <-stoppedCh:
	case <-time.After(25 * time.Second):
		t.Fatal("timeout waiting for WS screen_share_stopped")
	}
}

func startComposeScreenShare(t *testing.T, client *http.Client, base, accessToken, roomID string) string {
	t.Helper()
	url := fmt.Sprintf("%s/api/v1/voice/calls/%s/screen-share/start", base, roomID)
	req, err := http.NewRequest(http.MethodPost, url, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "start screen share body=%s", string(body))
	var parsed struct {
		ScreenShareSession struct {
			StreamID string `json:"stream_id"`
		} `json:"screen_share_session"`
	}
	require.NoError(t, json.Unmarshal(body, &parsed))
	return parsed.ScreenShareSession.StreamID
}

func stopComposeScreenShare(t *testing.T, client *http.Client, base, accessToken, roomID, streamID string) {
	t.Helper()
	url := fmt.Sprintf("%s/api/v1/voice/calls/%s/screen-share/stop", base, roomID)
	payload, err := json.Marshal(map[string]string{"stream_id": streamID})
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusNoContent, resp.StatusCode)
}
