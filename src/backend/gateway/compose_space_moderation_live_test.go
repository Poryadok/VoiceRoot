package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestComposeSpaceModeration_live documents spaces moderation via Gateway REST:
// kick, ban/unban, timeout (slow mode covered in chat/messaging tests).
func TestComposeSpaceModeration_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()

	n := time.Now().UnixNano()
	sessOwner := registerComposeUser(t, client, base, formatComposeEmail("space-mod-owner", n), "VoiceQaTest1!")
	sessMod := registerComposeUser(t, client, base, formatComposeEmail("space-mod-mod", n), "VoiceQaTest1!")
	sessMember := registerComposeUser(t, client, base, formatComposeEmail("space-mod-member", n), "VoiceQaTest1!")

	spaceID := createComposeSpace(t, client, base, sessOwner.AccessToken, "Moderation QA", "phase 5")

	invite := createComposeSpaceInvite(t, client, base, sessOwner.AccessToken, spaceID)
	joinComposeSpaceByInvite(t, client, base, sessMod.AccessToken, invite.Code)
	joinComposeSpaceByInvite(t, client, base, sessMember.AccessToken, invite.Code)

	assignComposeSpaceRole(t, client, base, sessOwner.AccessToken, spaceID, sessMod.ProfileID, "Moderator")

	require.Equal(t, http.StatusNoContent, kickComposeSpaceMemberStatus(t, client, base, sessMod.AccessToken, spaceID, sessMember.ProfileID))
	members := listComposeSpaceMembers(t, client, base, sessOwner.AccessToken, spaceID)
	for _, m := range members {
		require.NotEqual(t, sessMember.ProfileID, m.ProfileID)
	}

	invite2 := createComposeSpaceInvite(t, client, base, sessOwner.AccessToken, spaceID)
	joinComposeSpaceByInvite(t, client, base, sessMember.AccessToken, invite2.Code)

	require.Equal(t, http.StatusNoContent, banComposeSpaceMemberStatus(t, client, base, sessOwner.AccessToken, spaceID, sessMember.AccountID, "abuse"))
	bans := listComposeSpaceBans(t, client, base, sessOwner.AccessToken, spaceID)
	require.NotEmpty(t, bans)

	require.NotEqual(t, http.StatusOK, joinComposeSpaceByInviteStatus(t, client, base, sessMember.AccessToken, invite2.Code))

	require.Equal(t, http.StatusNoContent, unbanComposeSpaceMemberStatus(t, client, base, sessOwner.AccessToken, spaceID, sessMember.AccountID))
	require.Equal(t, http.StatusOK, joinComposeSpaceByInviteStatus(t, client, base, sessMember.AccessToken, invite2.Code))

	require.Equal(t, http.StatusNoContent, timeoutComposeSpaceMemberStatus(t, client, base, sessMod.AccessToken, spaceID, sessMember.ProfileID, 300))
	require.Equal(t, http.StatusNoContent, removeComposeSpaceMemberTimeoutStatus(t, client, base, sessMod.AccessToken, spaceID, sessMember.ProfileID))
}

func kickComposeSpaceMemberStatus(t *testing.T, client *http.Client, base, accessToken, spaceID, profileID string) int {
	t.Helper()
	req, err := http.NewRequest(http.MethodDelete, base+"/api/v1/spaces/"+spaceID+"/members/"+profileID, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	return resp.StatusCode
}

func banComposeSpaceMemberStatus(t *testing.T, client *http.Client, base, accessToken, spaceID, accountID, reason string) int {
	t.Helper()
	payload, err := json.Marshal(map[string]string{"account_id": accountID, "reason": reason})
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/spaces/"+spaceID+"/bans", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	return resp.StatusCode
}

func unbanComposeSpaceMemberStatus(t *testing.T, client *http.Client, base, accessToken, spaceID, accountID string) int {
	t.Helper()
	req, err := http.NewRequest(http.MethodDelete, base+"/api/v1/spaces/"+spaceID+"/bans/"+accountID, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	return resp.StatusCode
}

func listComposeSpaceBans(t *testing.T, client *http.Client, base, accessToken, spaceID string) []map[string]any {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, base+"/api/v1/spaces/"+spaceID+"/bans", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var parsed map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&parsed))
	list, _ := parsed["ban_list"].(map[string]any)
	bans, _ := list["bans"].([]any)
	out := make([]map[string]any, 0, len(bans))
	for _, b := range bans {
		if m, ok := b.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func joinComposeSpaceByInviteStatus(t *testing.T, client *http.Client, base, accessToken, code string) int {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/invites/"+code+"/join", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	return resp.StatusCode
}

func timeoutComposeSpaceMemberStatus(t *testing.T, client *http.Client, base, accessToken, spaceID, profileID string, seconds int) int {
	t.Helper()
	payload, err := json.Marshal(map[string]any{"duration_seconds": seconds})
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/spaces/"+spaceID+"/members/"+profileID+"/timeout", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	return resp.StatusCode
}

func removeComposeSpaceMemberTimeoutStatus(t *testing.T, client *http.Client, base, accessToken, spaceID, profileID string) int {
	t.Helper()
	req, err := http.NewRequest(http.MethodDelete, base+"/api/v1/spaces/"+spaceID+"/members/"+profileID+"/timeout", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	return resp.StatusCode
}
