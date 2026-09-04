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

func composeA1ChatItem(items []composeChatListItem, chatID string) *composeChatListItem {
	for i := range items {
		if items[i].ChatID == chatID {
			return &items[i]
		}
	}
	return nil
}
