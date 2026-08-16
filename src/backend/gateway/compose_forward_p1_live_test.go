package main

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestComposeForwardChannelCommentary_live documents FW-02:
// forward into a space channel with commentary; posted_as_chat when main-feed is restricted.
func TestComposeForwardChannelCommentary_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 60 * time.Second}
	base := liveGatewayBaseURL()
	n := time.Now().UnixNano()

	owner := registerComposeUser(t, client, base, formatComposeEmail("fwd-ch-owner", n), "VoiceQaTest1!")
	peer := registerComposeUser(t, client, base, formatComposeEmail("fwd-ch-peer", n), "VoiceQaTest1!")

	dmID := createComposeDMBetween(t, client, base, owner, peer)
	srcID := sendComposeMessage(t, client, base, peer.AccessToken, dmID, "fwd-channel-src")

	spaceID := createComposeSpace(t, client, base, owner.AccessToken, "Fwd Channel QA", "fw-02")
	channelChatID := createComposeSpaceChannel(t, client, base, owner.AccessToken, spaceID, "announcements")

	status, raw := forwardComposeMessageStatus(t, client, base, owner.AccessToken, srcID, channelChatID, "heads up")
	require.Equal(t, http.StatusOK, status, "body=%s", string(raw))

	var parsed struct {
		Message struct {
			ID           string `json:"id"`
			Content      string `json:"content"`
			MessageKind  string `json:"message_kind"`
			PostedAsChat bool   `json:"posted_as_chat"`
		} `json:"message"`
	}
	require.NoError(t, json.Unmarshal(raw, &parsed))
	require.NotEmpty(t, parsed.Message.ID)
	require.Equal(t, "fwd-channel-src", parsed.Message.Content)
	require.True(t, parsed.Message.PostedAsChat, "channel forward should set posted_as_chat")

	contents := composeMessageContentsInChat(t, client, base, owner.AccessToken, channelChatID)
	require.Contains(t, contents, "fwd-channel-src")
	require.Contains(t, contents, "heads up")
}

// TestComposeForwardPrivacyDeny_live documents FW-04: allow_forward=false → PermissionDenied.
func TestComposeForwardPrivacyDeny_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()
	n := time.Now().UnixNano()

	forwarder := registerComposeUser(t, client, base, formatComposeEmail("fwd-priv-a", n), "VoiceQaTest1!")
	author := registerComposeUser(t, client, base, formatComposeEmail("fwd-priv-b", n), "VoiceQaTest1!")

	patchComposePrivacy(t, client, base, author.AccessToken, map[string]any{
		"allow_forward": false,
	})

	dmID := createComposeDMBetween(t, client, base, forwarder, author)
	srcID := sendComposeMessage(t, client, base, author.AccessToken, dmID, "do-not-forward")

	groupID := createComposeGroup(t, client, base, forwarder.AccessToken, "Fwd deny target")
	status, raw := forwardComposeMessageStatus(t, client, base, forwarder.AccessToken, srcID, groupID, "")
	require.Equal(t, http.StatusForbidden, status, "body=%s", string(raw))
}
