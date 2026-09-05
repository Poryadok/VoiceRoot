package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestComposeA1TwoAccountsFoundation_live is the REST-only RED foundation for
// the A1 multi-account compose proof. It deliberately does not open Realtime,
// use File storage, render Flutter UI, or inspect the cache; those concerns
// have separate owners and later vertical gates.
func TestComposeA1TwoAccountsFoundation_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()

	health, err := client.Get(base + "/health")
	require.NoError(t, err)
	body, err := io.ReadAll(health.Body)
	health.Body.Close()
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, health.StatusCode, "Gateway health body=%s", string(body))

	n := time.Now().UnixNano()
	const password = "VoiceQaTest1!"
	emailA := formatComposeEmail("a1-t055-a", n)
	emailB := formatComposeEmail("a1-t055-b", n)
	require.NotEqual(t, emailA, emailB)

	sessA := registerComposeUser(t, client, base, emailA, password)
	sessB := registerComposeUser(t, client, base, emailB, password)
	require.NotEmpty(t, sessA.AccountID)
	require.NotEmpty(t, sessB.AccountID)
	require.NotEmpty(t, sessA.ProfileID)
	require.NotEmpty(t, sessB.ProfileID)
	require.NotEqual(t, sessA.AccountID, sessB.AccountID)
	require.NotEqual(t, sessA.ProfileID, sessB.ProfileID)

	setComposePrivacyAllowDmEveryone(t, client, base, sessA.AccessToken)
	setComposePrivacyAllowDmEveryone(t, client, base, sessB.AccessToken)

	searchToken := strings.TrimSuffix(emailB, "@voice-qa.test")
	searchReq, err := http.NewRequest(
		http.MethodGet,
		base+"/api/v1/users/search?q="+url.QueryEscape(searchToken),
		nil,
	)
	require.NoError(t, err)
	searchReq.Header.Set("Authorization", "Bearer "+sessA.AccessToken)
	searchResp, err := client.Do(searchReq)
	require.NoError(t, err)
	searchBody, err := io.ReadAll(searchResp.Body)
	searchResp.Body.Close()
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, searchResp.StatusCode, "profile search body=%s", string(searchBody))
	var search struct {
		ProfileList struct {
			Profiles []struct {
				ID        string `json:"id"`
				AccountID string `json:"account_id"`
			} `json:"profiles"`
		} `json:"profile_list"`
	}
	require.NoError(t, json.Unmarshal(searchBody, &search))
	foundB := false
	for _, profile := range search.ProfileList.Profiles {
		if profile.ID == sessB.ProfileID && profile.AccountID == sessB.AccountID {
			foundB = true
			break
		}
	}
	require.True(t, foundB, "search must return B's profile/account: body=%s", string(searchBody))

	sendComposeFriendInvitation(t, client, base, sessA.AccessToken, sessB.ProfileID)
	acceptComposeFriendInvitation(t, client, base, sessB.AccessToken, sessA.ProfileID)
	friendsA := composeFriendIDs(t, client, base, sessA.AccessToken)
	friendsB := composeFriendIDs(t, client, base, sessB.AccessToken)
	require.Contains(t, friendsA, sessB.ProfileID)
	require.Contains(t, friendsB, sessA.ProfileID)

	chatID := createComposeDM(t, client, base, sessA.AccessToken, sessB.ProfileID)
	require.NotEmpty(t, chatID)
	messageOne := fmt.Sprintf("a1-t055-%d-dm-one", n)
	messageTwo := fmt.Sprintf("a1-t055-%d-dm-two", n)
	messageOneID := sendComposeMessage(t, client, base, sessA.AccessToken, chatID, messageOne)
	messageTwoID := sendComposeMessage(t, client, base, sessA.AccessToken, chatID, messageTwo)
	require.NotEmpty(t, messageOneID)
	require.NotEmpty(t, messageTwoID)
	require.NotEqual(t, messageOneID, messageTwoID)

	mainB := listComposeChats(t, client, base, sessB.AccessToken, "main")
	var mainItem *composeChatListItem
	for i := range mainB {
		if mainB[i].ChatID == chatID {
			mainItem = &mainB[i]
			break
		}
	}
	require.NotNil(t, mainItem, "B main inbox must contain the DM: %+v", mainB)
	require.Equal(t, sessA.ProfileID, mainItem.DMPeerProfileID)
	require.Equal(t, "main", mainItem.Inbox)
	require.False(t, mainItem.IsStranger)
	require.Equal(t, 2, mainItem.UnreadCount, "B must have one unread count per fresh A→B message")

	getComposeMessagesContains(t, client, base, sessB.AccessToken, chatID, messageOneID, messageOne)
	getComposeMessagesContains(t, client, base, sessB.AccessToken, chatID, messageTwoID, messageTwo)
	markReadComposeMessage(t, client, base, sessB.AccessToken, chatID, messageTwoID)
	require.Equal(t, messageTwoID, getComposeReadState(t, client, base, sessB.AccessToken, chatID))

	mainBAfterRead := listComposeChats(t, client, base, sessB.AccessToken, "main")
	var mainItemAfterRead *composeChatListItem
	for i := range mainBAfterRead {
		if mainBAfterRead[i].ChatID == chatID {
			mainItemAfterRead = &mainBAfterRead[i]
			break
		}
	}
	require.NotNil(t, mainItemAfterRead, "read DM must remain in B main inbox: %+v", mainBAfterRead)
	require.Equal(t, "main", mainItemAfterRead.Inbox)
	require.False(t, mainItemAfterRead.IsStranger)
	require.Equal(t, 0, mainItemAfterRead.UnreadCount)
}

// TestComposeA1DailyMessagingREST_live is the REST-only RED proof for the
// A1 daily messaging path. It intentionally excludes Realtime, File,
// attachment restart, Flutter UI, and client cache concerns.
func TestComposeA1DailyMessagingREST_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()
	n := time.Now().UnixNano()
	const password = "VoiceQaTest1!"

	sessA := registerComposeUser(t, client, base, formatComposeEmail("a1-t055-phase2-a", n), password)
	sessB := registerComposeUser(t, client, base, formatComposeEmail("a1-t055-phase2-b", n), password)
	sessC := registerComposeUser(t, client, base, formatComposeEmail("a1-t055-phase2-c", n), password)
	require.NotEqual(t, sessA.AccountID, sessB.AccountID)
	require.NotEqual(t, sessA.AccountID, sessC.AccountID)
	require.NotEqual(t, sessB.AccountID, sessC.AccountID)
	require.NotEqual(t, sessA.ProfileID, sessB.ProfileID)
	require.NotEqual(t, sessA.ProfileID, sessC.ProfileID)
	require.NotEqual(t, sessB.ProfileID, sessC.ProfileID)

	// C and B stay strangers. B only opens its DM privacy gate so the first
	// C→B message must become B's request, not a friend/contact main row.
	setComposePrivacyAllowDmEveryone(t, client, base, sessB.AccessToken)
	dmID := createComposeDM(t, client, base, sessC.AccessToken, sessB.ProfileID)
	dmContent := fmt.Sprintf("a1-t055-phase2-%d-stranger-c-to-b", n)
	dmMessageID := sendComposeMessage(t, client, base, sessC.AccessToken, dmID, dmContent)

	require.Nil(t, composeA1ChatItem(listComposeChats(t, client, base, sessB.AccessToken, "main"), dmID),
		"stranger DM must not appear in B main inbox before Accept")
	requestItem := composeA1ChatItem(listComposeChats(t, client, base, sessB.AccessToken, "requests"), dmID)
	require.NotNil(t, requestItem, "stranger DM must appear in B requests inbox")
	require.Equal(t, "requests", requestItem.Inbox)
	require.True(t, requestItem.IsStranger)
	require.Equal(t, sessC.ProfileID, requestItem.DMPeerProfileID)
	getComposeMessagesContains(t, client, base, sessB.AccessToken, dmID, dmMessageID, dmContent)

	acceptComposeDMRequest(t, client, base, sessB.AccessToken, dmID)
	require.Nil(t, composeA1ChatItem(listComposeChats(t, client, base, sessB.AccessToken, "requests"), dmID),
		"accepted DM must leave B requests inbox")
	acceptedItem := composeA1ChatItem(listComposeChats(t, client, base, sessB.AccessToken, "main"), dmID)
	require.NotNil(t, acceptedItem, "accepted DM must appear in B main inbox")
	require.Equal(t, "main", acceptedItem.Inbox)
	require.False(t, acceptedItem.IsStranger)
	require.Equal(t, sessC.ProfileID, acceptedItem.DMPeerProfileID)
	require.NotNil(t, composeA1ChatItem(listComposeChats(t, client, base, sessC.AccessToken, "main"), dmID),
		"DM initiator must remain in C main inbox")

	groupID := createComposeGroup(t, client, base, sessA.AccessToken, fmt.Sprintf("a1-t055-phase2-group-%d", n))
	addComposeGroupMembersForInvitees(t, client, base, sessA.AccessToken, groupID, sessB, sessC)
	groupContent := fmt.Sprintf("a1-t055-phase2-%d-group-a-to-bc", n)
	groupMessageID := sendComposeMessage(t, client, base, sessA.AccessToken, groupID, groupContent)
	for name, session := range map[string]authSessionResponse{"A": sessA, "B": sessB, "C": sessC} {
		require.NotNil(t, composeA1ChatItem(listComposeChats(t, client, base, session.AccessToken, "main"), groupID),
			"group must appear in %s main inbox", name)
		getComposeMessagesContains(t, client, base, session.AccessToken, groupID, groupMessageID, groupContent)
	}

	spaceID := createComposeSpace(t, client, base, sessA.AccessToken,
		fmt.Sprintf("a1-t055-phase2-space-%d", n), "A1 REST daily messaging proof")
	invite := createComposeSpaceInvite(t, client, base, sessA.AccessToken, spaceID)
	joinComposeSpaceByInvite(t, client, base, sessB.AccessToken, invite.Code)
	joinComposeSpaceByInvite(t, client, base, sessC.AccessToken, invite.Code)
	channelID := createComposeSpaceChannel(t, client, base, sessA.AccessToken, spaceID,
		fmt.Sprintf("a1-t055-phase2-channel-%d", n))
	channelContent := fmt.Sprintf("a1-t055-phase2-%d-space-channel-a-to-bc", n)
	channelMessageID := sendComposeMessage(t, client, base, sessA.AccessToken, channelID, channelContent)
	for name, session := range map[string]authSessionResponse{"B": sessB, "C": sessC} {
		require.NotNil(t, composeA1ChatItem(listComposeChats(t, client, base, session.AccessToken, "main"), channelID),
			"space channel must appear in %s main inbox", name)
		getComposeMessagesContains(t, client, base, session.AccessToken, channelID, channelMessageID, channelContent)
	}

	composeArchiveChat(t, client, base, sessB.AccessToken, dmID, true)
	require.Nil(t, composeA1ChatItem(listComposeChats(t, client, base, sessB.AccessToken, "main"), dmID),
		"B archived DM must leave main inbox")
	require.NotNil(t, composeA1ChatItem(listComposeChats(t, client, base, sessB.AccessToken, "archive"), dmID),
		"B archived DM must appear in archive inbox")
	require.NotNil(t, composeA1ChatItem(listComposeChats(t, client, base, sessC.AccessToken, "main"), dmID),
		"B archive action must not move C's DM placement")
	require.Nil(t, composeA1ChatItem(listComposeChats(t, client, base, sessC.AccessToken, "archive"), dmID),
		"B archive action must not add C's DM to C archive inbox")

	composeArchiveChat(t, client, base, sessB.AccessToken, dmID, false)
	require.NotNil(t, composeA1ChatItem(listComposeChats(t, client, base, sessB.AccessToken, "main"), dmID),
		"B unarchived DM must return to main inbox")
	require.Nil(t, composeA1ChatItem(listComposeChats(t, client, base, sessB.AccessToken, "archive"), dmID),
		"B unarchived DM must leave archive inbox")
	require.NotNil(t, composeA1ChatItem(listComposeChats(t, client, base, sessC.AccessToken, "main"), dmID),
		"B unarchive action must leave C's DM in C main inbox")
	require.Nil(t, composeA1ChatItem(listComposeChats(t, client, base, sessC.AccessToken, "archive"), dmID),
		"B unarchive action must leave C's archive placement unchanged")
}

// TestComposeA1GroupReadIsolation_live proves that a group member's durable
// read cursor and main-inbox unread badge do not advance another member.
func TestComposeA1GroupReadIsolation_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()
	n := time.Now().UnixNano()
	const password = "VoiceQaTest1!"

	sessA := registerComposeUser(t, client, base, formatComposeEmail("a1-t055-group-author", n), password)
	sessB := registerComposeUser(t, client, base, formatComposeEmail("a1-t055-group-reader", n), password)
	sessC := registerComposeUser(t, client, base, formatComposeEmail("a1-t055-group-untouched", n), password)
	groupID := createComposeGroup(t, client, base, sessA.AccessToken, fmt.Sprintf("a1-t055-group-read-%d", n))
	addComposeGroupMembersForInvitees(t, client, base, sessA.AccessToken, groupID, sessB, sessC)

	assertComposeA1PerMemberReadIsolation(t, client, base, "group", groupID, sessA, sessB, sessC, n)
}

// TestComposeA1ChannelReadIsolation_live proves the same per-member read
// boundary for a Space channel, where membership is established by invite.
func TestComposeA1ChannelReadIsolation_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()
	n := time.Now().UnixNano()
	const password = "VoiceQaTest1!"

	sessA := registerComposeUser(t, client, base, formatComposeEmail("a1-t055-channel-author", n), password)
	sessB := registerComposeUser(t, client, base, formatComposeEmail("a1-t055-channel-reader", n), password)
	sessC := registerComposeUser(t, client, base, formatComposeEmail("a1-t055-channel-untouched", n), password)
	spaceID := createComposeSpace(t, client, base, sessA.AccessToken,
		fmt.Sprintf("a1-t055-channel-space-%d", n), "A1 per-member channel read proof")
	invite := createComposeSpaceInvite(t, client, base, sessA.AccessToken, spaceID)
	joinComposeSpaceByInvite(t, client, base, sessB.AccessToken, invite.Code)
	joinComposeSpaceByInvite(t, client, base, sessC.AccessToken, invite.Code)
	channelID := createComposeSpaceChannel(t, client, base, sessA.AccessToken, spaceID,
		fmt.Sprintf("a1-t055-channel-read-%d", n))

	assertComposeA1PerMemberReadIsolation(t, client, base, "channel", channelID, sessA, sessB, sessC, n)
}

// TestComposeA1BlockDMDenyBothDirections_live proves the documented block
// boundary against an already-established DM. It is deliberately REST-only:
// this slice asserts that denied mutations make no persisted history or inbox
// preview change, while Realtime event delivery has its own vertical proof.
func TestComposeA1BlockDMDenyBothDirections_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()
	n := time.Now().UnixNano()
	const password = "VoiceQaTest1!"

	sessA := registerComposeUser(t, client, base, formatComposeEmail("a1-t055-block-a", n), password)
	sessB := registerComposeUser(t, client, base, formatComposeEmail("a1-t055-block-b", n), password)
	dmID := createComposeDMBetween(t, client, base, sessA, sessB)
	baselineContent := fmt.Sprintf("a1-t055-block-baseline-%d", n)
	baselineMessageID := sendComposeMessage(t, client, base, sessA.AccessToken, dmID, baselineContent)
	acceptComposeDMRequest(t, client, base, sessB.AccessToken, dmID)

	var beforeA, beforeB composeChatListItem
	require.Eventually(t, func() bool {
		itemA := composeA1ChatItem(listComposeChats(t, client, base, sessA.AccessToken, "main"), dmID)
		itemB := composeA1ChatItem(listComposeChats(t, client, base, sessB.AccessToken, "main"), dmID)
		if itemA == nil || itemB == nil || itemA.LastPreview != baselineContent || itemB.LastPreview != baselineContent {
			return false
		}
		beforeA, beforeB = *itemA, *itemB
		return true
	}, 45*time.Second, 500*time.Millisecond,
		"baseline DM must be established in both main inboxes before block assertions")
	getComposeMessagesContains(t, client, base, sessA.AccessToken, dmID, baselineMessageID, baselineContent)
	getComposeMessagesContains(t, client, base, sessB.AccessToken, dmID, baselineMessageID, baselineContent)
	beforeHistoryA := composeA1MessageHistory(t, client, base, sessA.AccessToken, dmID)
	beforeHistoryB := composeA1MessageHistory(t, client, base, sessB.AccessToken, dmID)

	blockComposeAccount(t, client, base, sessA.AccessToken, sessB.AccountID)

	require.Equal(t, http.StatusForbidden,
		sendComposeMessageStatus(t, client, base, sessA.AccessToken, dmID, fmt.Sprintf("a1-t055-block-a-to-b-%d", n), ""),
		"A-to-B send after A blocks B must be a block-policy denial, not dependency degradation")
	require.Equal(t, http.StatusForbidden,
		sendComposeMessageStatus(t, client, base, sessB.AccessToken, dmID, fmt.Sprintf("a1-t055-block-b-to-a-%d", n), ""),
		"B-to-A send after A blocks B must be a block-policy denial, not dependency degradation")
	require.Equal(t, http.StatusForbidden,
		createComposeDMStatus(t, client, base, sessA.AccessToken, sessB.ProfileID),
		"A must not reopen a DM to the blocked account")
	require.Equal(t, http.StatusForbidden,
		createComposeDMStatus(t, client, base, sessB.AccessToken, sessA.ProfileID),
		"B must not reopen a DM to the blocking account")

	require.Equal(t, beforeHistoryA, composeA1MessageHistory(t, client, base, sessA.AccessToken, dmID),
		"denied A/B sends must not append or alter A's DM history")
	require.Equal(t, beforeHistoryB, composeA1MessageHistory(t, client, base, sessB.AccessToken, dmID),
		"denied A/B sends must not append or alter B's DM history")
	afterA := composeA1ChatItem(listComposeChats(t, client, base, sessA.AccessToken, "main"), dmID)
	afterB := composeA1ChatItem(listComposeChats(t, client, base, sessB.AccessToken, "main"), dmID)
	require.Equal(t, &beforeA, afterA, "denied A/B sends must not mutate A's DM list row")
	require.Equal(t, &beforeB, afterB, "denied A/B sends must not mutate B's DM list row")
}

type composeA1HistoryMessage struct {
	ID      string
	Content string
}

func composeA1MessageHistory(t *testing.T, client *http.Client, base, accessToken, chatID string) []composeA1HistoryMessage {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, base+"/api/v1/messages?chat_id="+url.QueryEscape(chatID), nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "GET messages body=%s", string(body))

	var parsed struct {
		MessageList struct {
			Messages []composeA1HistoryMessage `json:"messages"`
		} `json:"message_list"`
	}
	require.NoError(t, json.Unmarshal(body, &parsed))
	return parsed.MessageList.Messages
}

func assertComposeA1PerMemberReadIsolation(
	t *testing.T,
	client *http.Client,
	base, chatKind, chatID string,
	author, reader, untouched authSessionResponse,
	n int64,
) {
	t.Helper()
	messageContent := fmt.Sprintf("a1-t055-%s-read-isolation-%d", chatKind, n)
	messageID := sendComposeMessage(t, client, base, author.AccessToken, chatID, messageContent)

	require.Eventually(t, func() bool {
		return composeA1UnreadCount(t, client, base, reader.AccessToken, chatID) == 1 &&
			composeA1UnreadCount(t, client, base, untouched.AccessToken, chatID) == 1
	}, 45*time.Second, 500*time.Millisecond,
		"%s message must leave one unread main-inbox item for both reader and untouched member", chatKind)

	// The Gateway REST ChatListItem does not expose last_message_delivery_state;
	// the canonical Messaging contract requires group/channel ticks to remain NONE.
	require.Empty(t, getComposeReadState(t, client, base, untouched.AccessToken, chatID),
		"untouched %s member must not have a read cursor before B reads", chatKind)

	markReadComposeMessage(t, client, base, reader.AccessToken, chatID, messageID)
	require.Equal(t, messageID, getComposeReadState(t, client, base, reader.AccessToken, chatID),
		"reader must persist its own %s cursor", chatKind)
	require.Empty(t, getComposeReadState(t, client, base, untouched.AccessToken, chatID),
		"reader must not advance another member's %s cursor", chatKind)

	require.Eventually(t, func() bool {
		return composeA1UnreadCount(t, client, base, reader.AccessToken, chatID) == 0 &&
			composeA1UnreadCount(t, client, base, untouched.AccessToken, chatID) == 1
	}, 45*time.Second, 500*time.Millisecond,
		"reader's %s MarkRead must clear only reader's unread main-inbox badge", chatKind)
}

func composeA1UnreadCount(t *testing.T, client *http.Client, base, accessToken, chatID string) int {
	t.Helper()
	item := composeA1ChatItem(listComposeChats(t, client, base, accessToken, "main"), chatID)
	if item == nil || item.Inbox != "main" {
		return -1
	}
	return item.UnreadCount
}

func composeA1ChatItem(items []composeChatListItem, chatID string) *composeChatListItem {
	for i := range items {
		if items[i].ChatID == chatID {
			return &items[i]
		}
	}
	return nil
}
