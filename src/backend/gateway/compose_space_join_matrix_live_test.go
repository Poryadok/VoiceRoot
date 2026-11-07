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

// TestComposeSpaceProtectedJoinMatrix_live extends the existing single-channel
// proof with admission negatives and independent member AND channel cursors.
// Privacy updates round-trip the existing settings and change only invite rules.
func TestComposeSpaceProtectedJoinMatrix_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()
	n := time.Now().UnixNano()
	owner := registerComposeUser(t, client, base, formatComposeEmail("space-matrix-owner", n), "VoiceQaTest1!")
	b := registerComposeUser(t, client, base, formatComposeEmail("space-matrix-b", n), "VoiceQaTest1!")
	c := registerComposeUser(t, client, base, formatComposeEmail("space-matrix-c", n), "VoiceQaTest1!")
	guest := registerComposeGuest(t, client, base, "VoiceQaTest1!")
	space := createComposeSpace(t, client, base, owner.AccessToken, fmt.Sprintf("matrix-%d", n), "protected admission matrix")
	invite := createComposeSpaceInvite(t, client, base, owner.AccessToken, space)

	// A stranger cannot use an invite when its recipient permits only friends.
	// Save every other setting, rather than applying the gaming-open helper.
	baseline := composeMatrixPrivacy(t, client, base, b.AccessToken)
	closed := map[string]any{"friends": true}
	composeMatrixInvitePrivacy(t, client, base, b.AccessToken, baseline, closed)
	composeMatrixDeny(t, client, base, b.AccessToken, "/api/v1/invites/"+invite.Code+"/join", "invite blocked by recipient privacy settings")
	require.Empty(t, listComposeSpaces(t, client, base, b.AccessToken), "denied join must not publish membership")
	composeMatrixMemberAbsent(t, client, base, owner.AccessToken, space, b.ProfileID)
	t.Log("PASS privacy friends-only denial; no membership")
	everyone := map[string]any{"friends": true, "friends_of_friends": true, "space_members": true, "include_guests": true}
	composeMatrixInvitePrivacy(t, client, base, b.AccessToken, baseline, everyone)
	joinComposeSpaceByInvite(t, client, base, b.AccessToken, invite.Code)
	cBaseline := composeMatrixPrivacy(t, client, base, c.AccessToken)
	composeMatrixInvitePrivacy(t, client, base, c.AccessToken, cBaseline, everyone)
	joinComposeSpaceByInvite(t, client, base, c.AccessToken, invite.Code)
	t.Log("PASS invite-only privacy delta; unrelated settings preserved")

	// Two channels, two messages each: reading an older message advances exactly
	// one member/channel position. Reading C later cannot advance B to message 2.
	channels := []string{
		createComposeSpaceChannel(t, client, base, owner.AccessToken, space, "matrix-one"),
		createComposeSpaceChannel(t, client, base, owner.AccessToken, space, "matrix-two"),
	}
	var messages [2][2]string
	for i, channel := range channels {
		for j := range messages[i] {
			messages[i][j] = sendComposeMessage(t, client, base, owner.AccessToken, channel, fmt.Sprintf("matrix-%d-%d-%d", n, i, j))
		}
	}
	assertUnread := func(b0, b1, c0, c1 int64) {
		t.Helper()
		require.Eventually(t, func() bool {
			return composeA1UnreadCount(t, client, base, b.AccessToken, channels[0]) == b0 &&
				composeA1UnreadCount(t, client, base, b.AccessToken, channels[1]) == b1 &&
				composeA1UnreadCount(t, client, base, c.AccessToken, channels[0]) == c0 &&
				composeA1UnreadCount(t, client, base, c.AccessToken, channels[1]) == c1
		}, 45*time.Second, 500*time.Millisecond)
	}
	assertUnread(2, 2, 2, 2)
	markReadComposeMessage(t, client, base, b.AccessToken, channels[0], messages[0][0])
	assertUnread(1, 2, 2, 2)
	require.Equal(t, messages[0][0], getComposeReadState(t, client, base, b.AccessToken, channels[0]))
	require.Empty(t, getComposeReadState(t, client, base, b.AccessToken, channels[1]))
	require.Empty(t, getComposeReadState(t, client, base, c.AccessToken, channels[0]))
	markReadComposeMessage(t, client, base, c.AccessToken, channels[0], messages[0][1])
	assertUnread(1, 2, 0, 2)
	require.Equal(t, messages[0][0], getComposeReadState(t, client, base, b.AccessToken, channels[0]))
	require.Equal(t, messages[0][1], getComposeReadState(t, client, base, c.AccessToken, channels[0]))
	markReadComposeMessage(t, client, base, b.AccessToken, channels[1], messages[1][1])
	assertUnread(1, 0, 0, 2)
	require.Empty(t, getComposeReadState(t, client, base, c.AccessToken, channels[1]))
	t.Log("PASS two-member/two-channel partial and later-read cursor isolation")

	// Guest admission defaults closed even with permissive invite privacy.
	guestBaseline := composeMatrixPrivacy(t, client, base, guest.AccessToken)
	composeMatrixInvitePrivacy(t, client, base, guest.AccessToken, guestBaseline, everyone)
	assertGuestSetting := func(want bool) {
		t.Helper()
		var response struct {
			Space struct {
				AllowGuests bool `json:"allow_guests"`
			} `json:"space"`
		}
		require.Equal(t, http.StatusOK, composeMatrixRequest(t, client, base, owner.AccessToken, http.MethodGet, "/api/v1/spaces/"+space, nil, &response))
		require.Equal(t, want, response.Space.AllowGuests)
	}
	assertGuestSetting(false)
	composeMatrixDeny(t, client, base, guest.AccessToken, "/api/v1/invites/"+invite.Code+"/join", "guests not allowed in this space")
	composeMatrixMemberAbsent(t, client, base, owner.AccessToken, space, guest.ProfileID)
	require.Equal(t, http.StatusOK, composeMatrixRequest(t, client, base, owner.AccessToken, http.MethodPatch, "/api/v1/spaces/"+space, map[string]any{"allow_guests": true}, nil))
	assertGuestSetting(true)
	composeMatrixDeny(t, client, base, guest.AccessToken, "/api/v1/spaces/"+space+"/join", "guests cannot join spaces directly")
	composeMatrixMemberAbsent(t, client, base, owner.AccessToken, space, guest.ProfileID)
	joinedGuest := joinComposeSpaceByInvite(t, client, base, guest.AccessToken, invite.Code)
	require.Equal(t, guest.ProfileID, joinedGuest.GetProfileId())
	require.Equal(t, space, joinedGuest.GetSpaceId())
	composeMatrixMemberPresent(t, client, base, owner.AccessToken, space, guest.ProfileID)
	t.Log("PASS guest default denial, direct denial and explicit invite opt-in")

	for _, visibility := range []string{"public", "invite_only", "private"} {
		t.Run(visibility, func(t *testing.T) {
			id := createComposeSpace(t, client, base, owner.AccessToken, fmt.Sprintf("%s-%d", visibility, n), "direct versus invite")
			require.Equal(t, http.StatusOK, composeMatrixRequest(t, client, base, owner.AccessToken, http.MethodPatch, "/api/v1/spaces/"+id, map[string]any{"visibility": visibility}, nil))
			if visibility == "public" {
				require.Equal(t, http.StatusOK, composeMatrixRequest(t, client, base, b.AccessToken, http.MethodPost, "/api/v1/spaces/"+id+"/join", nil, nil))
			} else {
				composeMatrixDeny(t, client, base, b.AccessToken, "/api/v1/spaces/"+id+"/join", "space is not public")
				composeMatrixMemberAbsent(t, client, base, owner.AccessToken, id, b.ProfileID)
				inv := createComposeSpaceInvite(t, client, base, owner.AccessToken, id)
				joinComposeSpaceByInvite(t, client, base, b.AccessToken, inv.Code)
			}
			composeMatrixMemberPresent(t, client, base, owner.AccessToken, id, b.ProfileID)
		})
	}
}

func composeMatrixRequest(t *testing.T, client *http.Client, base, token, method, path string, payload any, result any) int {
	t.Helper()
	var data []byte
	var err error
	if payload != nil {
		data, err = json.Marshal(payload)
		require.NoError(t, err)
	}
	req, err := http.NewRequest(method, base+path, bytes.NewReader(data))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	if result != nil {
		require.NoError(t, json.Unmarshal(body, result))
	}
	return resp.StatusCode
}

func composeMatrixPrivacy(t *testing.T, client *http.Client, base, token string) map[string]any {
	t.Helper()
	var result struct {
		Settings map[string]any `json:"privacy_settings"`
	}
	require.Equal(t, http.StatusOK, composeMatrixRequest(t, client, base, token, http.MethodGet, "/api/v1/users/me/privacy", nil, &result))
	require.NotEmpty(t, result.Settings)
	return result.Settings
}

func composeMatrixInvitePrivacy(t *testing.T, client *http.Client, base, token string, baseline, audience map[string]any) {
	t.Helper()
	settings := make(map[string]any, len(baseline))
	for k, v := range baseline {
		settings[k] = v
	}
	settings["allow_chat_space_invites"] = audience
	require.Equal(t, http.StatusOK, composeMatrixRequest(t, client, base, token, http.MethodPatch, "/api/v1/users/me/privacy", map[string]any{"settings": settings}, nil))
	after := composeMatrixPrivacy(t, client, base, token)
	require.Len(t, after, len(baseline), "privacy response keyset must be preserved")
	gotAudience, ok := after["allow_chat_space_invites"].(map[string]any)
	require.True(t, ok, "invite audience must be returned")
	for _, field := range []string{"friends", "friends_of_friends", "space_members", "include_guests"} {
		want, _ := audience[field].(bool)
		got, _ := gotAudience[field].(bool)
		require.Equal(t, want, got, "invite audience field %s must match requested value", field)
	}
	for k, before := range baseline {
		if k != "allow_chat_space_invites" && k != "updated_at" {
			require.Equal(t, before, after[k], "privacy field %s must be preserved", k)
		}
	}
}

func composeMatrixDeny(t *testing.T, client *http.Client, base, token, path, reason string) {
	t.Helper()
	var failure struct {
		Code    string `json:"error_code"`
		Message string `json:"message"`
	}
	require.Equal(t, http.StatusForbidden, composeMatrixRequest(t, client, base, token, http.MethodPost, path, nil, &failure))
	require.Equal(t, "permission_denied", failure.Code)
	require.Equal(t, reason, failure.Message)
}

func composeMatrixMemberAbsent(t *testing.T, client *http.Client, base, ownerToken, spaceID, deniedProfile string) {
	t.Helper()
	for _, member := range listComposeSpaceMembers(t, client, base, ownerToken, spaceID) {
		require.NotEqual(t, deniedProfile, member.ProfileID, "denied admission must not publish membership")
	}
}

func composeMatrixMemberPresent(t *testing.T, client *http.Client, base, ownerToken, spaceID, profile string) {
	t.Helper()
	for _, member := range listComposeSpaceMembers(t, client, base, ownerToken, spaceID) {
		if member.ProfileID == profile {
			return
		}
	}
	t.Fatal("successful admission must publish the exact requested profile membership")
}
