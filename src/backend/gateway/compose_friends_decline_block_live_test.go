package main

import (
	"encoding/json"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestComposeFriendsDeclineBlock_live: decline invitation; block prevents DM (chat guard).
func TestComposeFriendsDeclineBlock_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()

	n := time.Now().UnixNano()
	sessA := registerComposeUser(t, client, base, formatComposeEmail("friends-decline-a", n), "VoiceQaTest1!")
	sessB := registerComposeUser(t, client, base, formatComposeEmail("friends-decline-b", n), "VoiceQaTest1!")

	sendComposeFriendInvitation(t, client, base, sessA.AccessToken, sessB.ProfileID)
	declineComposeFriendInvitation(t, client, base, sessB.AccessToken, sessA.ProfileID)

	friendsA := composeFriendIDs(t, client, base, sessA.AccessToken)
	require.False(t, slices.Contains(friendsA, sessB.ProfileID))

	invitePayload, err := json.Marshal(map[string]string{"target_profile_id": sessB.ProfileID})
	require.NoError(t, err)
	status := composePostStatus(t, client, base, sessA.AccessToken, "/api/v1/friends/invitations", invitePayload)
	require.Equal(t, http.StatusOK, status, "re-invite after decline should succeed")

	blockComposeAccount(t, client, base, sessA.AccessToken, sessB.AccountID)

	dmPayload, err := json.Marshal(map[string]string{"other_profile_id": sessB.ProfileID})
	require.NoError(t, err)
	status = composePostStatus(t, client, base, sessA.AccessToken, "/api/v1/chats/dm", dmPayload)
	require.NotEqual(t, http.StatusOK, status, "DM to blocked profile must fail")
}
