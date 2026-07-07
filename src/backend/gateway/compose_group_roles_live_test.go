package main

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestComposeGroupRoles_live: owner/member roles, voluntary leave, owner protections via Gateway REST.
func TestComposeGroupRoles_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()

	n := time.Now().UnixNano()
	sessOwner := registerComposeUser(t, client, base, formatComposeEmail("roles-owner", n), "VoiceQaTest1!")
	sessB := registerComposeUser(t, client, base, formatComposeEmail("roles-member-b", n), "VoiceQaTest1!")
	sessC := registerComposeUser(t, client, base, formatComposeEmail("roles-member-c", n), "VoiceQaTest1!")
	sessLeaver := registerComposeUser(t, client, base, formatComposeEmail("roles-leaver", n), "VoiceQaTest1!")

	chatID := createComposeGroup(t, client, base, sessOwner.AccessToken, "Roles squad")
	addComposeGroupMembersForInvitees(t, client, base, sessOwner.AccessToken, chatID, sessB, sessC, sessLeaver)

	members := listComposeGroupMembers(t, client, base, sessOwner.AccessToken, chatID)
	roles := map[string]string{}
	for _, m := range members {
		roles[m.ProfileID] = m.Role
	}
	require.Equal(t, "owner", roles[sessOwner.ProfileID])
	require.Equal(t, "member", roles[sessB.ProfileID])
	require.Equal(t, "member", roles[sessC.ProfileID])
	require.Equal(t, "member", roles[sessLeaver.ProfileID])

	require.Equal(t, http.StatusPreconditionFailed, removeComposeGroupMemberStatus(t, client, base, sessOwner.AccessToken, chatID, sessOwner.ProfileID))
	require.Equal(t, http.StatusPreconditionFailed, leaveComposeGroupStatus(t, client, base, sessOwner.AccessToken, chatID))

	require.Equal(t, http.StatusNoContent, leaveComposeGroupStatus(t, client, base, sessLeaver.AccessToken, chatID))
	require.Equal(t, http.StatusForbidden, getComposeChatStatus(t, client, base, sessLeaver.AccessToken, chatID))
	require.Equal(t, http.StatusOK, getComposeChatStatus(t, client, base, sessB.AccessToken, chatID))
}
