package main

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestComposeInAppNotifications_live documents WS notification fanout for group messages (docs/features/notifications.md).
func TestComposeInAppNotifications_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 60 * time.Second}
	base := liveGatewayBaseURL()
	n := time.Now().UnixNano()

	owner := registerComposeUser(t, client, base, formatComposeEmail("inapp-owner", n), "VoiceQaTest1!")
	member := registerComposeUser(t, client, base, formatComposeEmail("inapp-member", n), "VoiceQaTest1!")
	filler := registerComposeUser(t, client, base, formatComposeEmail("inapp-filler", n), "VoiceQaTest1!")

	groupID := createComposeGroup(t, client, base, owner.AccessToken, "In-app notify QA")
	addComposeGroupMembersForInvitees(t, client, base, owner.AccessToken, groupID, member, filler)

	wsMember := connectComposeWSSubscribed(t, base, member.AccessToken, groupID)
	defer wsMember.Close()

	notifyCh := make(chan composeWSFrame, 1)
	go func() {
		notifyCh <- waitComposeWSOp(t, wsMember, "notification", 25*time.Second, func(d map[string]any) bool {
			return d["type"] == "new_message" && d["chat_id"] == groupID
		})
	}()

	marker := "in-app-notify-" + formatComposeEmail("msg", n)
	sendComposeMessage(t, client, base, owner.AccessToken, groupID, marker)

	select {
	case frame := <-notifyCh:
		var d map[string]any
		require.NoError(t, json.Unmarshal(frame.D, &d))
		require.Equal(t, groupID, d["chat_id"])
		require.Equal(t, "new_message", d["type"])
	case <-time.After(30 * time.Second):
		t.Fatal("timeout waiting for in-app notification WS frame")
	}
}
