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

// TestComposeGuestRestrictions_live verifies guest accounts cannot initiate restricted actions via Gateway.
//
// Opt-in: VOICE_RUN_LIVE_COMPOSE=true VOICE_API_BASE_URL=http://127.0.0.1:18080
func TestComposeGuestRestrictions_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()

	const guestPassword = "VoiceQaTest1!"
	guestSess := registerComposeGuest(t, client, base, guestPassword)
	regularSess := registerComposeUser(t, client, base, formatComposeEmail("guest-restrict-regular", time.Now().UnixNano()), guestPassword)

	t.Run("guest cannot create DM", func(t *testing.T) {
		status := composeCreateDMStatus(t, client, base, guestSess.AccessToken, regularSess.ProfileID)
		require.Equal(t, http.StatusForbidden, status, "guest CreateDM must be PermissionDenied")
	})

	t.Run("guest cannot send friend invitation", func(t *testing.T) {
		status := composeSendFriendInvitationStatus(t, client, base, guestSess.AccessToken, regularSess.ProfileID)
		require.Equal(t, http.StatusForbidden, status, "guest SendFriendInvitation must be denied")
	})

	t.Run("guest cannot create space", func(t *testing.T) {
		payload, err := json.Marshal(map[string]string{
			"name":        "guest-space",
			"description": "denied",
		})
		require.NoError(t, err)
		req, err := http.NewRequest(http.MethodPost, base+"/api/v1/spaces", bytes.NewReader(payload))
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+guestSess.AccessToken)
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		require.Equal(t, http.StatusForbidden, resp.StatusCode, "guest CreateSpace body=%s", string(body))
	})

	t.Run("guest cannot start voice call", func(t *testing.T) {
		chatID := createComposeDM(t, client, base, regularSess.AccessToken, guestSess.ProfileID)
		status := composeStartCallStatus(t, client, base, guestSess.AccessToken, chatID, regularSess.ProfileID)
		require.Equal(t, http.StatusForbidden, status, "guest StartCall must be denied")
	})
}

func composeCreateDMStatus(t *testing.T, client *http.Client, base, accessToken, otherProfileID string) int {
	t.Helper()
	payload, err := json.Marshal(map[string]string{"other_profile_id": otherProfileID})
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/chats/dm", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	_, _ = io.ReadAll(resp.Body)
	return resp.StatusCode
}

func composeSendFriendInvitationStatus(t *testing.T, client *http.Client, base, accessToken, targetProfileID string) int {
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

func composeStartCallStatus(t *testing.T, client *http.Client, base, accessToken, chatID, calleeProfileID string) int {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"linked_chat":       map[string]string{"id": chatID},
		"callee_profile_id": calleeProfileID,
		"media_kind":        "CALL_MEDIA_KIND_AUDIO",
	})
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/voice/calls", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	_, _ = io.ReadAll(resp.Body)
	return resp.StatusCode
}
