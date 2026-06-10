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
