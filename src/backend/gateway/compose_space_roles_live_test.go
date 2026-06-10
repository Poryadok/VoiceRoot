package main

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestComposeSpaceRoles_live documents PLAN Phase 5 space roles via Gateway REST:
// bootstrap hierarchy, join assigns Member, permission gates for invites.
func TestComposeSpaceRoles_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()

	n := time.Now().UnixNano()
	sessOwner := registerComposeUser(t, client, base, formatComposeEmail("space-roles-owner", n), "VoiceQaTest1!")
	sessDelegate := registerComposeUser(t, client, base, formatComposeEmail("space-roles-delegate", n), "VoiceQaTest1!")
	sessJoiner := registerComposeUser(t, client, base, formatComposeEmail("space-roles-joiner", n), "VoiceQaTest1!")

	spaceID := createComposeSpace(t, client, base, sessOwner.AccessToken, "Roles QA", "phase 5")

	roles := listComposeSpaceRoles(t, client, base, sessOwner.AccessToken, spaceID)
	roleNames := map[string]bool{}
	for _, r := range roles {
		roleNames[r.Name] = true
	}
	require.True(t, roleNames["Owner"])
	require.True(t, roleNames["Admin"])
	require.True(t, roleNames["Moderator"])
	require.True(t, roleNames["Member"])
	require.True(t, roleNames["Guest"])

	invite := createComposeSpaceInvite(t, client, base, sessOwner.AccessToken, spaceID)
	joinComposeSpaceByInvite(t, client, base, sessJoiner.AccessToken, invite.Code)

	memberRoles := getComposeMemberRoles(t, client, base, sessOwner.AccessToken, spaceID, sessJoiner.ProfileID)
	require.Contains(t, memberRoles, "Member")

	members := listComposeSpaceMembers(t, client, base, sessOwner.AccessToken, spaceID)
	ownerRoles := map[string][]string{}
	for _, m := range members {
		ownerRoles[m.ProfileID] = m.RoleNames
	}
	require.Contains(t, ownerRoles[sessOwner.ProfileID], "Owner")
	require.Contains(t, ownerRoles[sessJoiner.ProfileID], "Member")

	require.Equal(t, http.StatusForbidden, createComposeSpaceInviteStatus(t, client, base, sessDelegate.AccessToken, spaceID))

	assignComposeSpaceRole(t, client, base, sessOwner.AccessToken, spaceID, sessDelegate.ProfileID, "Admin")
	require.Equal(t, http.StatusOK, createComposeSpaceInviteStatus(t, client, base, sessDelegate.AccessToken, spaceID))
}
