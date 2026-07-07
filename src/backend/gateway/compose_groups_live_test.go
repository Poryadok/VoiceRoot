package main

import (
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestComposeGroups_live: create standalone group, invite, avatar, kick via Gateway REST.
func TestComposeGroups_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()

	n := time.Now().UnixNano()
	sessA := registerComposeUser(t, client, base, formatComposeEmail("group-owner", n), "VoiceQaTest1!")
	sessB := registerComposeUser(t, client, base, formatComposeEmail("group-member-b", n), "VoiceQaTest1!")
	sessC := registerComposeUser(t, client, base, formatComposeEmail("group-member-c", n), "VoiceQaTest1!")

	chatID := createComposeGroup(t, client, base, sessA.AccessToken, "Friday squad")
	addComposeGroupMembersForInvitees(t, client, base, sessA.AccessToken, chatID, sessB, sessC)

	listB := listComposeChats(t, client, base, sessB.AccessToken, "main")
	require.True(t, slices.ContainsFunc(listB, func(item composeChatListItem) bool {
		return item.ChatID == chatID
	}), "invited member B must list group: %+v", listB)

	const avatar = "https://cdn.voice.gg/groups/compose-live.webp"
	updateComposeGroupAvatar(t, client, base, sessA.AccessToken, chatID, avatar)

	removeComposeGroupMember(t, client, base, sessA.AccessToken, chatID, sessC.ProfileID)
	require.Equal(t, http.StatusForbidden, getComposeChatStatus(t, client, base, sessC.AccessToken, chatID))
	require.Equal(t, http.StatusOK, getComposeChatStatus(t, client, base, sessB.AccessToken, chatID))
}
