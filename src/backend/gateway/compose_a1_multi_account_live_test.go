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
	require.GreaterOrEqual(t, mainItem.UnreadCount, 1, "B must have unread DM metadata")

	getComposeMessagesContains(t, client, base, sessB.AccessToken, chatID, messageOneID, messageOne)
	getComposeMessagesContains(t, client, base, sessB.AccessToken, chatID, messageTwoID, messageTwo)
	markReadComposeMessage(t, client, base, sessB.AccessToken, chatID, messageTwoID)
	require.Equal(t, messageTwoID, getComposeReadState(t, client, base, sessB.AccessToken, chatID))
}
