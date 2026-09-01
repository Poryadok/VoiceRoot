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

// TestComposeDMArchiveMute_live documents TC-DM-08:
// ArchiveChat hides DM from caller's ListChats; MuteChat sets muted_until.
func TestComposeDMArchiveMute_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()
	n := time.Now().UnixNano()

	a := registerComposeUser(t, client, base, formatComposeEmail("dm08-a", n), "VoiceQaTest1!")
	b := registerComposeUser(t, client, base, formatComposeEmail("dm08-b", n), "VoiceQaTest1!")

	dmID := createComposeDMBetween(t, client, base, a, b)
	sendComposeMessage(t, client, base, a.AccessToken, dmID, "before-archive")

	listA := listComposeChats(t, client, base, a.AccessToken, "")
	require.True(t, composeChatListContains(listA, dmID))

	composeArchiveChat(t, client, base, a.AccessToken, dmID, true)
	listA = listComposeChats(t, client, base, a.AccessToken, "")
	require.False(t, composeChatListContains(listA, dmID), "archived DM must leave caller's main list")

	listArchive := listComposeChats(t, client, base, a.AccessToken, "archive")
	require.True(t, composeChatListContains(listArchive, dmID), "archived DM must appear in archive inbox")

	listB := listComposeChats(t, client, base, b.AccessToken, "")
	require.True(t, composeChatListContains(listB, dmID), "peer must still see the DM")

	composeArchiveChat(t, client, base, a.AccessToken, dmID, false)
	listA = listComposeChats(t, client, base, a.AccessToken, "")
	require.True(t, composeChatListContains(listA, dmID), "unarchive restores DM")

	mutedUntil := time.Now().UTC().Add(2 * time.Hour).Format(time.RFC3339)
	composeMuteChat(t, client, base, a.AccessToken, dmID, mutedUntil)

	members := composeListChatMembers(t, client, base, a.AccessToken, dmID)
	var muted bool
	for _, m := range members {
		if stringFromAny(m["profile_id"], m["profileId"]) == a.ProfileID {
			if stringFromAny(m["muted_until"], m["mutedUntil"]) != "" {
				muted = true
			}
		}
	}
	require.True(t, muted, "MuteChat must set muted_until on caller's membership")

	composeMuteChat(t, client, base, a.AccessToken, dmID, "")
	members = composeListChatMembers(t, client, base, a.AccessToken, dmID)
	for _, m := range members {
		if stringFromAny(m["profile_id"], m["profileId"]) == a.ProfileID {
			require.Empty(t, stringFromAny(m["muted_until"], m["mutedUntil"]), "omitted muted_until unmutes")
		}
	}
}

func composeChatListContains(items []composeChatListItem, chatID string) bool {
	for _, it := range items {
		if it.ChatID == chatID {
			return true
		}
	}
	return false
}

func composeArchiveChat(t *testing.T, client *http.Client, base, token, chatID string, archived bool) {
	t.Helper()
	payload, err := json.Marshal(map[string]any{"archived": archived})
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/chats/"+chatID+"/archive", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	require.True(t, resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusOK,
		"archive body=%s", string(raw))
}

func composeMuteChat(t *testing.T, client *http.Client, base, token, chatID, mutedUntilRFC3339 string) {
	t.Helper()
	body := map[string]any{}
	if mutedUntilRFC3339 != "" {
		body["muted_until"] = mutedUntilRFC3339
	}
	payload, err := json.Marshal(body)
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/chats/"+chatID+"/mute", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	require.True(t, resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusOK,
		"mute body=%s", string(raw))
}

func composeListChatMembers(t *testing.T, client *http.Client, base, token, chatID string) []map[string]any {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, base+"/api/v1/chats/"+chatID+"/members", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "list members body=%s", string(raw))
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(raw, &parsed))
	list, _ := parsed["member_list"].(map[string]any)
	if list == nil {
		list, _ = parsed["memberList"].(map[string]any)
	}
	require.NotNil(t, list)
	members, _ := list["members"].([]any)
	out := make([]map[string]any, 0, len(members))
	for _, m := range members {
		if mm, ok := m.(map[string]any); ok {
			out = append(out, mm)
		}
	}
	return out
}
