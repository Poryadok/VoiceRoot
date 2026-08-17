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

// TestComposeForwardWithoutAttribution_live documents FW-03:
// without_attribution copies content as a regular message (no Forwarded-from).
func TestComposeForwardWithoutAttribution_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()
	n := time.Now().UnixNano()

	sender := registerComposeUser(t, client, base, formatComposeEmail("fw03-a", n), "VoiceQaTest1!")
	peer := registerComposeUser(t, client, base, formatComposeEmail("fw03-b", n), "VoiceQaTest1!")
	other := registerComposeUser(t, client, base, formatComposeEmail("fw03-c", n), "VoiceQaTest1!")

	srcDM := createComposeDMBetween(t, client, base, sender, peer)
	srcID := sendComposeMessage(t, client, base, peer.AccessToken, srcDM, "copy-me-plain")

	targetDM := createComposeDMBetween(t, client, base, sender, other)
	status, raw := forwardComposeMessageWithoutAttributionStatus(t, client, base, sender.AccessToken, srcID, targetDM)
	require.Equal(t, http.StatusOK, status, "body=%s", string(raw))

	var parsed struct {
		Message struct {
			ID                string `json:"id"`
			Content           string `json:"content"`
			MessageKind       string `json:"message_kind"`
			Type              string `json:"type"`
			ForwardFromID     string `json:"forward_from_id"`
			ForwardFromSender string `json:"forward_from_sender"`
		} `json:"message"`
	}
	require.NoError(t, json.Unmarshal(raw, &parsed))
	require.Equal(t, "copy-me-plain", parsed.Message.Content)
	require.Empty(t, parsed.Message.ForwardFromID)
	require.Empty(t, parsed.Message.ForwardFromSender)
	kind := parsed.Message.MessageKind
	if kind == "" {
		kind = parsed.Message.Type
	}
	require.Contains(t, []string{"MESSAGE_KIND_REGULAR", "regular", "REGULAR"}, kind)

	contents := composeMessageContentsInChat(t, client, base, sender.AccessToken, targetDM)
	require.Contains(t, contents, "copy-me-plain")
}

// TestComposeForwardMulti_live documents FW-05: multi-select forward of two
// messages into one target with a single commentary before the batch.
func TestComposeForwardMulti_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()
	n := time.Now().UnixNano()

	sender := registerComposeUser(t, client, base, formatComposeEmail("fw05-a", n), "VoiceQaTest1!")
	peer := registerComposeUser(t, client, base, formatComposeEmail("fw05-b", n), "VoiceQaTest1!")
	member := registerComposeUser(t, client, base, formatComposeEmail("fw05-c", n), "VoiceQaTest1!")

	dmID := createComposeDMBetween(t, client, base, sender, peer)
	srcA := sendComposeMessage(t, client, base, peer.AccessToken, dmID, "fw-05-a")
	srcB := sendComposeMessage(t, client, base, peer.AccessToken, dmID, "fw-05-b")

	groupID := createComposeGroup(t, client, base, sender.AccessToken, "Fwd multi target")
	addComposeGroupMembersForInvitees(t, client, base, sender.AccessToken, groupID, peer, member)

	status, raw := forwardComposeMessageStatus(t, client, base, sender.AccessToken, srcA, groupID, "batch note")
	require.Equal(t, http.StatusOK, status, "body=%s", string(raw))
	status, raw = forwardComposeMessageStatus(t, client, base, sender.AccessToken, srcB, groupID, "")
	require.Equal(t, http.StatusOK, status, "body=%s", string(raw))

	contents := composeMessageContentsInChat(t, client, base, member.AccessToken, groupID)
	require.Contains(t, contents, "fw-05-a")
	require.Contains(t, contents, "fw-05-b")
	require.Contains(t, contents, "batch note")
	noteCount := 0
	for _, c := range contents {
		if c == "batch note" {
			noteCount++
		}
	}
	require.Equal(t, 1, noteCount)
}

func forwardComposeMessageWithoutAttributionStatus(
	t *testing.T, client *http.Client, base, accessToken, sourceMessageID, targetChatID string,
) (int, []byte) {
	t.Helper()
	bodyMap := map[string]any{
		"source_message_id":   sourceMessageID,
		"target_chat":         map[string]string{"id": targetChatID},
		"without_attribution": true,
	}
	payload, err := json.Marshal(bodyMap)
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/messages/forward", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, raw
}
