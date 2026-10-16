package main

import (
	"encoding/json"
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
	assertRequests := func(token, direction string, want int) {
		t.Helper()
		request, err := http.NewRequest(http.MethodGet, base+"/api/v1/friends/requests", nil)
		require.NoError(t, err)
		request.Header.Set("Authorization", "Bearer "+token)
		response, err := client.Do(request)
		require.NoError(t, err)
		defer response.Body.Close()
		require.Equal(t, http.StatusOK, response.StatusCode)
		var snapshot struct {
			List map[string][]json.RawMessage `json:"friend_request_list"`
		}
		require.NoError(t, json.NewDecoder(response.Body).Decode(&snapshot))
		require.Len(t, snapshot.List[direction], want)
	}

	sendComposeFriendInvitation(t, client, base, sessA.AccessToken, sessB.ProfileID)
	assertRequests(sessA.AccessToken, "outgoing", 1)
	assertRequests(sessB.AccessToken, "incoming", 1)
	// Sender cancellation is distinct from receiver decline and accepted removal.
	cancelRequest, err := http.NewRequest(http.MethodPost, base+"/api/v1/friends/invitations/"+sessB.ProfileID+"/cancel", nil)
	require.NoError(t, err)
	cancelRequest.Header.Set("Authorization", "Bearer "+sessA.AccessToken)
	cancelResponse, err := client.Do(cancelRequest)
	require.NoError(t, err)
	_ = cancelResponse.Body.Close()
	require.Equal(t, http.StatusOK, cancelResponse.StatusCode)
	assertRequests(sessA.AccessToken, "outgoing", 0)
	assertRequests(sessB.AccessToken, "incoming", 0)
	sendComposeFriendInvitation(t, client, base, sessA.AccessToken, sessB.ProfileID)
	acceptComposeFriendInvitation(t, client, base, sessB.AccessToken, sessA.ProfileID)

	friendsA := composeFriendIDs(t, client, base, sessA.AccessToken)
	friendsB := composeFriendIDs(t, client, base, sessB.AccessToken)
	require.True(t, slices.Contains(friendsA, sessB.ProfileID), "A should list B as friend: %v", friendsA)
	require.True(t, slices.Contains(friendsB, sessA.ProfileID), "B should list A as friend: %v", friendsB)
}
