package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestComposePins_live: pin message in group chat, list pinned, WS fan-out.
func TestComposePins_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()
	n := time.Now().UnixNano()

	sessOwner := registerComposeUser(t, client, base, formatComposeEmail("pin-owner", n), "VoiceQaTest1!")
	sessMember := registerComposeUser(t, client, base, formatComposeEmail("pin-member", n), "VoiceQaTest1!")
	sessFiller := registerComposeUser(t, client, base, formatComposeEmail("pin-filler", n), "VoiceQaTest1!")
	groupID := createComposeGroup(t, client, base, sessOwner.AccessToken, "Pins QA")
	addComposeGroupMembers(
		t, client, base, sessOwner.AccessToken, groupID,
		sessMember.ProfileID, sessFiller.ProfileID,
	)

	msgID := sendComposeMessage(t, client, base, sessOwner.AccessToken, groupID, "pin-me-compose")

	wsMember := connectComposeWSSubscribed(t, base, sessMember.AccessToken, groupID)
	pinCh := make(chan composeWSFrame, 1)
	go func() {
		pinCh <- waitComposeWSOp(t, wsMember, "message_pinned", 20*time.Second, func(d map[string]any) bool {
			return d["chat_id"] == groupID && d["message_id"] == msgID
		})
	}()

	pinPayload, err := json.Marshal(map[string]any{
		"chat": map[string]string{"id": groupID},
	})
	require.NoError(t, err)
	pinReq, err := http.NewRequest(http.MethodPost, base+"/api/v1/messages/"+msgID+"/pin", bytes.NewReader(pinPayload))
	require.NoError(t, err)
	pinReq.Header.Set("Authorization", "Bearer "+sessMember.AccessToken)
	pinReq.Header.Set("Content-Type", "application/json")
	pinResp, err := client.Do(pinReq)
	require.NoError(t, err)
	defer pinResp.Body.Close()
	require.Equal(t, http.StatusNoContent, pinResp.StatusCode)

	select {
	case frame := <-pinCh:
		var d map[string]any
		require.NoError(t, json.Unmarshal(frame.D, &d))
		require.Equal(t, sessMember.ProfileID, d["pinned_by"])
	case <-time.After(25 * time.Second):
		t.Fatal("timeout waiting for message_pinned")
	}

	listReq, err := http.NewRequest(http.MethodGet, base+"/api/v1/chats/"+groupID+"/pinned-messages", nil)
	require.NoError(t, err)
	listReq.Header.Set("Authorization", "Bearer "+sessOwner.AccessToken)
	listResp, err := client.Do(listReq)
	require.NoError(t, err)
	defer listResp.Body.Close()
	require.Equal(t, http.StatusOK, listResp.StatusCode)

	unpinReq, err := http.NewRequest(http.MethodDelete, base+"/api/v1/messages/"+msgID+"/pin?chat_id="+groupID, nil)
	require.NoError(t, err)
	unpinReq.Header.Set("Authorization", "Bearer "+sessMember.AccessToken)
	unpinResp, err := client.Do(unpinReq)
	require.NoError(t, err)
	defer unpinResp.Body.Close()
	require.Equal(t, http.StatusNoContent, unpinResp.StatusCode)
}
