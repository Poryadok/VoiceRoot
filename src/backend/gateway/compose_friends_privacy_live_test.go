package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestComposeFriendsPrivacyDeny_live: allow_friend_requests nobody blocks stranger invites (FR-03 / PV-04).
func TestComposeFriendsPrivacyDeny_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()
	n := time.Now().UnixNano()

	target := registerComposeUser(t, client, base, formatComposeEmail("fr-priv-target", n), "VoiceQaTest1!")
	stranger := registerComposeUser(t, client, base, formatComposeEmail("fr-priv-stranger", n), "VoiceQaTest1!")

	nobody := map[string]any{"friends": false, "friends_of_friends": false, "space_members": false, "include_guests": false}
	patchComposePrivacy(t, client, base, target.AccessToken, map[string]any{
		"preset":                "personal",
		"allow_friend_requests": nobody,
	})

	status := sendComposeFriendInvitationStatus(t, client, base, stranger.AccessToken, target.ProfileID)
	require.Equal(t, http.StatusForbidden, status, "stranger invite must be denied by allow_friend_requests")
}

func sendComposeFriendInvitationStatus(t *testing.T, client *http.Client, base, accessToken, targetProfileID string) int {
	t.Helper()
	payload, err := json.Marshal(map[string]string{"target_profile_id": targetProfileID})
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/friends/invitations", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	_, _ = io.ReadAll(resp.Body)
	return resp.StatusCode
}
