package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestComposePrivacyFoF_live documents A—C—B graph: stranger denied, FoF allowed for DM.
func TestComposePrivacyFoF_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 60 * time.Second}
	base := liveGatewayBaseURL()
	n := time.Now().UnixNano()

	profA := registerComposeUser(t, client, base, formatComposeEmail("p11fof-a", n), "VoiceQaTest1!")
	profC := registerComposeUser(t, client, base, formatComposeEmail("p11fof-c", n), "VoiceQaTest1!")
	profB := registerComposeUser(t, client, base, formatComposeEmail("p11fof-b", n), "VoiceQaTest1!")

	sendComposeFriendInvitation(t, client, base, profA.AccessToken, profC.ProfileID)
	acceptComposeFriendInvitation(t, client, base, profC.AccessToken, profA.ProfileID)
	sendComposeFriendInvitation(t, client, base, profC.AccessToken, profB.ProfileID)
	acceptComposeFriendInvitation(t, client, base, profB.AccessToken, profC.ProfileID)

	privacyBody, err := json.Marshal(map[string]any{
		"settings": map[string]any{
			"preset": "personal",
			"allow_dm": map[string]any{
				"friends":            true,
				"friends_of_friends": true,
			},
			"show_online": map[string]any{"friends": true},
			"show_game_status": map[string]any{"friends": true},
			"show_mm_rating": map[string]any{"friends": true, "friends_of_friends": true},
			"show_phone": map[string]any{"friends": false, "friends_of_friends": false, "space_members": false, "include_guests": false},
			"show_stories": map[string]any{"friends": true, "friends_of_friends": true},
			"allow_friend_requests": map[string]any{
				"friends": true, "friends_of_friends": true, "space_members": true, "include_guests": true,
			},
			"allow_guest_dm": false,
		},
	})
	require.NoError(t, err)
	privReq, err := http.NewRequest(http.MethodPatch, base+"/api/v1/users/me/privacy", bytes.NewReader(privacyBody))
	require.NoError(t, err)
	privReq.Header.Set("Authorization", "Bearer "+profB.AccessToken)
	privReq.Header.Set("Content-Type", "application/json")
	privResp, err := client.Do(privReq)
	require.NoError(t, err)
	defer privResp.Body.Close()
	require.Equal(t, http.StatusOK, privResp.StatusCode)

	stranger := registerComposeUser(t, client, base, formatComposeEmail("p11fof-stranger", n), "VoiceQaTest1!")
	dmPayload, err := json.Marshal(map[string]string{"other_profile_id": profB.ProfileID})
	require.NoError(t, err)

	strangerReq, err := http.NewRequest(http.MethodPost, base+"/api/v1/chats/dm", bytes.NewReader(dmPayload))
	require.NoError(t, err)
	strangerReq.Header.Set("Authorization", "Bearer "+stranger.AccessToken)
	strangerReq.Header.Set("Content-Type", "application/json")
	strangerResp, err := client.Do(strangerReq)
	require.NoError(t, err)
	defer strangerResp.Body.Close()
	require.Equal(t, http.StatusForbidden, strangerResp.StatusCode)

	fofReq, err := http.NewRequest(http.MethodPost, base+"/api/v1/chats/dm", bytes.NewReader(dmPayload))
	require.NoError(t, err)
	fofReq.Header.Set("Authorization", "Bearer "+profA.AccessToken)
	fofReq.Header.Set("Content-Type", "application/json")
	fofResp, err := client.Do(fofReq)
	require.NoError(t, err)
	defer fofResp.Body.Close()
	require.Equal(t, http.StatusOK, fofResp.StatusCode)
}
