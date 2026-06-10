package main

import (
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestComposeSpaces_live: create space, list, get, icon + description via Gateway REST.
func TestComposeSpaces_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()

	n := time.Now().UnixNano()
	sess := registerComposeUser(t, client, base, formatComposeEmail("space-owner", n), "VoiceQaTest1!")

	spaceID := createComposeSpace(t, client, base, sess.AccessToken, "Friday squad", "We raid on Fridays")

	list := listComposeSpaces(t, client, base, sess.AccessToken)
	require.True(t, slices.ContainsFunc(list, func(item composeSpaceItem) bool {
		return item.ID == spaceID && item.Name == "Friday squad" && item.Description == "We raid on Fridays"
	}), "created space must appear in list: %+v", list)

	got := getComposeSpace(t, client, base, sess.AccessToken, spaceID)
	require.Equal(t, "Friday squad", got.Name)
	require.Equal(t, "We raid on Fridays", got.Description)

	const icon = "https://cdn.voice.gg/spaces/compose-live.webp"
	const desc = "Updated about us"
	updateComposeSpace(t, client, base, sess.AccessToken, spaceID, icon, desc)

	updated := getComposeSpace(t, client, base, sess.AccessToken, spaceID)
	require.Equal(t, icon, updated.IconURL)
	require.Equal(t, desc, updated.Description)
}

// TestComposeSpacesTree_live: category, voice room, space chat, list tree, reorder.
func TestComposeSpacesTree_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()

	n := time.Now().UnixNano()
	sess := registerComposeUser(t, client, base, formatComposeEmail("space-tree", n), "VoiceQaTest1!")

	spaceID := createComposeSpace(t, client, base, sess.AccessToken, "Tree QA", "tree e2e")
	_ = createComposeSpaceCategory(t, client, base, sess.AccessToken, spaceID, "General")
	voiceRoomID := createComposeSpaceVoiceRoom(t, client, base, sess.AccessToken, spaceID, "Lobby")
	chatNodeID := createComposeSpaceChat(t, client, base, sess.AccessToken, spaceID, "announcements")

	tree := getComposeSpaceTree(t, client, base, sess.AccessToken, spaceID)
	require.Len(t, tree.Categories, 1)
	require.GreaterOrEqual(t, len(tree.Nodes), 2)

	var voiceNodeID string
	for _, node := range tree.Nodes {
		if node.Kind == "voice_room" {
			voiceNodeID = node.ID
			require.Equal(t, voiceRoomID, node.VoiceRoomID)
		}
	}
	require.NotEmpty(t, voiceNodeID)

	reorderComposeSpaceTree(t, client, base, sess.AccessToken, spaceID, []string{chatNodeID, voiceNodeID})

	after := getComposeSpaceTree(t, client, base, sess.AccessToken, spaceID)
	require.Equal(t, chatNodeID, after.Nodes[0].ID)
	require.Equal(t, voiceNodeID, after.Nodes[1].ID)
}

// TestComposeSpacesInvites_live: create invite, list, join as second user, revoke blocks new joins.
func TestComposeSpacesInvites_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()

	n := time.Now().UnixNano()
	ownerSess := registerComposeUser(t, client, base, formatComposeEmail("space-invite-owner", n), "VoiceQaTest1!")
	joinerSess := registerComposeUser(t, client, base, formatComposeEmail("space-invite-joiner", n), "VoiceQaTest1!")

	spaceID := createComposeSpace(t, client, base, ownerSess.AccessToken, "Invite QA", "invites e2e")
	inv := createComposeSpaceInvite(t, client, base, ownerSess.AccessToken, spaceID)

	list := listComposeSpaceInvites(t, client, base, ownerSess.AccessToken, spaceID)
	require.True(t, slices.ContainsFunc(list, func(item composeSpaceInvite) bool {
		return item.ID == inv.ID && item.Code == inv.Code
	}))

	joinComposeSpaceByInvite(t, client, base, joinerSess.AccessToken, inv.Code)

	joinerList := listComposeSpaces(t, client, base, joinerSess.AccessToken)
	require.True(t, slices.ContainsFunc(joinerList, func(item composeSpaceItem) bool {
		return item.ID == spaceID
	}))

	revokeComposeSpaceInvite(t, client, base, ownerSess.AccessToken, spaceID, inv.ID)

	lateSess := registerComposeUser(t, client, base, formatComposeEmail("space-invite-late", n), "VoiceQaTest1!")
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/invites/"+inv.Code+"/join", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+lateSess.AccessToken)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
}
