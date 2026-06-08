package main

import (
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestComposeFriendsInvitation_live: search → invite → accept → both list each other as friends.
func TestComposeFriendsInvitation_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()

	n := time.Now().UnixNano()
	sessA := registerComposeUser(t, client, base, formatComposeEmail("friends-a", n), "VoiceQaTest1!")
	sessB := registerComposeUser(t, client, base, formatComposeEmail("friends-b", n), "VoiceQaTest1!")

	sendComposeFriendInvitation(t, client, base, sessA.AccessToken, sessB.ProfileID)
	acceptComposeFriendInvitation(t, client, base, sessB.AccessToken, sessA.ProfileID)

	friendsA := composeFriendIDs(t, client, base, sessA.AccessToken)
	friendsB := composeFriendIDs(t, client, base, sessB.AccessToken)
	require.True(t, slices.Contains(friendsA, sessB.ProfileID), "A should list B as friend: %v", friendsA)
	require.True(t, slices.Contains(friendsB, sessA.ProfileID), "B should list A as friend: %v", friendsB)
}
