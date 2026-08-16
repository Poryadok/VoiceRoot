package main

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const (
	permBitTextChatSendMessages     = uint64(1) << 15
	permBitVoiceJoin                = uint64(1) << 17
	permBitTextChatMentionAllInChat = uint64(1) << 24
)

// TestComposeRolesSendDeny_live documents RL-02: chat_override deny TEXT_CHAT_SEND_MESSAGES → SendMessage 403.
func TestComposeRolesSendDeny_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 60 * time.Second}
	base := liveGatewayBaseURL()
	n := time.Now().UnixNano()

	owner := registerComposeUser(t, client, base, formatComposeEmail("rl02-owner", n), "VoiceQaTest1!")
	member := registerComposeUser(t, client, base, formatComposeEmail("rl02-member", n), "VoiceQaTest1!")

	spaceID := createComposeSpace(t, client, base, owner.AccessToken, "RL-02 QA", "send deny")
	invite := createComposeSpaceInvite(t, client, base, owner.AccessToken, spaceID)
	joinComposeSpaceByInvite(t, client, base, member.AccessToken, invite.Code)

	chatID := createComposeSpaceChannel(t, client, base, owner.AccessToken, spaceID, "muted-chat")
	memberRoleID := composeRoleIDByName(t, client, base, owner.AccessToken, spaceID, "Member")
	setComposeChatOverride(t, client, base, owner.AccessToken, spaceID, chatID, memberRoleID, permBitTextChatSendMessages)

	status := sendComposeMessageStatus(t, client, base, member.AccessToken, chatID, "should be blocked", "[]")
	require.Equal(t, http.StatusForbidden, status)
}

// TestComposeMentionsEveryoneDeny_live documents TC-MSG-03: @everyone without mention permission → 403.
func TestComposeMentionsEveryoneDeny_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 60 * time.Second}
	base := liveGatewayBaseURL()
	n := time.Now().UnixNano()

	owner := registerComposeUser(t, client, base, formatComposeEmail("mention-owner", n), "VoiceQaTest1!")
	member := registerComposeUser(t, client, base, formatComposeEmail("mention-member", n), "VoiceQaTest1!")

	spaceID := createComposeSpace(t, client, base, owner.AccessToken, "Mention QA", "tc-msg-03")
	invite := createComposeSpaceInvite(t, client, base, owner.AccessToken, spaceID)
	joinComposeSpaceByInvite(t, client, base, member.AccessToken, invite.Code)

	chatID := createComposeSpaceChannel(t, client, base, owner.AccessToken, spaceID, "general")
	memberRoleID := composeRoleIDByName(t, client, base, owner.AccessToken, spaceID, "Member")
	setComposeChatOverride(t, client, base, owner.AccessToken, spaceID, chatID, memberRoleID, permBitTextChatMentionAllInChat)

	status := sendComposeMessageStatus(t, client, base, member.AccessToken, chatID, "@everyone hi",
		`[{"type":"everyone"}]`)
	require.Equal(t, http.StatusForbidden, status)

	// Bare @everyone in content (no mentions_json) must also be gated.
	statusBare := sendComposeMessageStatus(t, client, base, member.AccessToken, chatID, "ping @everyone now", "[]")
	require.Equal(t, http.StatusForbidden, statusBare)
}

// TestComposeVoiceJoinDeny_live documents RL-03: voice room VOICE_JOIN deny → join 403.
func TestComposeVoiceJoinDeny_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 60 * time.Second}
	base := liveGatewayBaseURL()
	n := time.Now().UnixNano()

	owner := registerComposeUser(t, client, base, formatComposeEmail("vj-owner", n), "VoiceQaTest1!")
	member := registerComposeUser(t, client, base, formatComposeEmail("vj-member", n), "VoiceQaTest1!")

	spaceID := createComposeSpace(t, client, base, owner.AccessToken, "Voice Join QA", "rl-03")
	invite := createComposeSpaceInvite(t, client, base, owner.AccessToken, spaceID)
	joinComposeSpaceByInvite(t, client, base, member.AccessToken, invite.Code)

	voiceRoomID := createComposeSpaceVoiceRoom(t, client, base, owner.AccessToken, spaceID, "Locked Lobby")
	memberRoleID := composeRoleIDByName(t, client, base, owner.AccessToken, spaceID, "Member")
	setComposeVoiceOverride(t, client, base, owner.AccessToken, spaceID, voiceRoomID, memberRoleID, permBitVoiceJoin)

	status := joinComposeVoiceRoomStatus(t, client, base, member.AccessToken, voiceRoomID, spaceID)
	require.Equal(t, http.StatusForbidden, status)
}
