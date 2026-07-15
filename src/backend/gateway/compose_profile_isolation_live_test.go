package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestComposeProfileFriendIsolation_live documents friends are scoped per profile (no cross-leak).
func TestComposeProfileFriendIsolation_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 90 * time.Second}
	base := liveGatewayBaseURL()
	n := time.Now().UnixNano()

	sess := registerComposeUser(t, client, base, formatComposeEmail("p13-isolation", n), "VoiceQaTest1!")

	altResp := composePostJSON(t, client, base+"/api/v1/users/profiles", sess.AccessToken,
		`{"display_name":"Work Persona"}`)
	require.Equal(t, http.StatusOK, altResp.StatusCode, composeReadBody(t, altResp))
	altProfileID := composeNestedJSONString(t, altResp, "profile", "id")

	switchResp := composePostJSON(t, client, base+"/api/v1/auth/switch-profile", sess.AccessToken,
		`{"profile_id":"`+altProfileID+`"}`)
	require.Equal(t, http.StatusOK, switchResp.StatusCode, composeReadBody(t, switchResp))
	var switchBody struct {
		AccessToken string `json:"access_token"`
	}
	composeDecodeJSON(t, switchResp.Body, &switchBody)
	require.NotEmpty(t, switchBody.AccessToken)

	peer := registerComposeUser(t, client, base, formatComposeEmail("p13-peer", n), "VoiceQaTest1!")
	sendComposeFriendInvitation(t, client, base, switchBody.AccessToken, peer.ProfileID)
	acceptComposeFriendInvitation(t, client, base, peer.AccessToken, altProfileID)

	primaryFriends := composeFriendIDs(t, client, base, sess.AccessToken)
	altFriends := composeFriendIDs(t, client, base, switchBody.AccessToken)

	require.False(t, slices.Contains(primaryFriends, peer.ProfileID),
		"primary profile must not see alt profile's friends")
	require.True(t, slices.Contains(altFriends, peer.ProfileID),
		"alt profile must list its own friends")
}

// TestComposeProfileChatIsolation_live documents chat lists are scoped per profile.
func TestComposeProfileChatIsolation_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 90 * time.Second}
	base := liveGatewayBaseURL()
	n := time.Now().UnixNano()

	sess := registerComposeUser(t, client, base, formatComposeEmail("p13-chat-iso", n), "VoiceQaTest1!")
	altToken, altProfileID := composeCreateAltProfile(t, client, base, sess.AccessToken, "Work Persona", "work")

	peer := registerComposeUser(t, client, base, formatComposeEmail("p13-chat-peer", n), "VoiceQaTest1!")
	altDMChatID := createComposeDM(t, client, base, altToken, peer.ProfileID)

	primaryChats := listComposeChats(t, client, base, sess.AccessToken, "main")
	altChats := listComposeChats(t, client, base, altToken, "main")

	primaryIDs := chatIDsFromList(primaryChats)
	altIDs := chatIDsFromList(altChats)

	require.False(t, slices.Contains(primaryIDs, altDMChatID),
		"primary profile must not see alt profile's DM")
	require.True(t, slices.Contains(altIDs, altDMChatID),
		"alt profile must list its own DM")
	_ = altProfileID
}

// TestComposeProfileCreatePreset_live documents create profile applies privacy preset.
func TestComposeProfileCreatePreset_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 90 * time.Second}
	base := liveGatewayBaseURL()
	n := time.Now().UnixNano()

	sess := registerComposeUser(t, client, base, formatComposeEmail("p13-preset", n), "VoiceQaTest1!")
	altToken, _ := composeCreateAltProfile(t, client, base, sess.AccessToken, "Gaming Alt", "gaming")

	req, err := http.NewRequest(http.MethodGet, base+"/api/v1/users/me/privacy", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+altToken)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "privacy body=%s", string(body))

	var parsed struct {
		PrivacySettings struct {
			Preset string `json:"preset"`
		} `json:"privacy_settings"`
	}
	require.NoError(t, json.Unmarshal(body, &parsed))
	require.Equal(t, "gaming", parsed.PrivacySettings.Preset)
}

// TestComposeProfileVoiceSwitch_live documents active call stays on original profile after switch.
func TestComposeProfileVoiceSwitch_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 90 * time.Second}
	base := liveGatewayBaseURL()
	n := time.Now().UnixNano()

	sessA := registerComposeUser(t, client, base, formatComposeEmail("p13-voice-a", n), "VoiceQaTest1!")
	sessB := registerComposeUser(t, client, base, formatComposeEmail("p13-voice-b", n), "VoiceQaTest1!")
	primaryProfileID := sessA.ProfileID

	chatID := createComposeDMBetween(t, client, base, sessA, sessB)
	call := startComposeCall(t, client, base, sessA.AccessToken, chatID, sessB.ProfileID)
	_ = acceptComposeCall(t, client, base, sessB.AccessToken, call.RoomID)

	activePrimary := getComposeActiveCall(t, client, base, sessA.AccessToken)
	require.NotNil(t, activePrimary)
	require.Equal(t, call.RoomID, activePrimary.RoomID)

	altToken, _ := composeCreateAltProfile(t, client, base, sessA.AccessToken, "Alt Voice", "personal")
	activeAlt := getComposeActiveCall(t, client, base, altToken)
	require.Nil(t, activeAlt, "switched profile must not inherit active call")

	switchBackResp := composePostJSON(t, client, base+"/api/v1/auth/switch-profile", altToken,
		`{"profile_id":"`+primaryProfileID+`"}`)
	require.Equal(t, http.StatusOK, switchBackResp.StatusCode, composeReadBody(t, switchBackResp))
	var switchBackBody struct {
		AccessToken string `json:"access_token"`
	}
	composeDecodeJSON(t, switchBackResp.Body, &switchBackBody)
	require.NotEmpty(t, switchBackBody.AccessToken)

	activeAfterSwitchBack := getComposeActiveCall(t, client, base, switchBackBody.AccessToken)
	require.NotNil(t, activeAfterSwitchBack)
	require.Equal(t, call.RoomID, activeAfterSwitchBack.RoomID)

	endComposeCall(t, client, base, switchBackBody.AccessToken, call.RoomID)
}

// TestComposeProfileFreeLimit_live documents third profile is rejected on free tier.
func TestComposeProfileFreeLimit_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 90 * time.Second}
	base := liveGatewayBaseURL()
	n := time.Now().UnixNano()

	sess := registerComposeUser(t, client, base, formatComposeEmail("p13-limit", n), "VoiceQaTest1!")
	_, _ = composeCreateAltProfile(t, client, base, sess.AccessToken, "Alt One", "personal")

	thirdResp := composePostJSON(t, client, base+"/api/v1/users/profiles", sess.AccessToken,
		`{"display_name":"Alt Two"}`)
	require.Equal(t, http.StatusTooManyRequests, thirdResp.StatusCode, composeReadBody(t, thirdResp))
}

func composeCreateAltProfile(t *testing.T, client *http.Client, base, accessToken, displayName, preset string) (string, string) {
	t.Helper()
	altResp := composePostJSON(t, client, base+"/api/v1/users/profiles", accessToken,
		`{"display_name":"`+displayName+`","preset":"`+preset+`"}`)
	require.Equal(t, http.StatusOK, altResp.StatusCode, composeReadBody(t, altResp))
	altProfileID := composeNestedJSONString(t, altResp, "profile", "id")

	switchResp := composePostJSON(t, client, base+"/api/v1/auth/switch-profile", accessToken,
		`{"profile_id":"`+altProfileID+`"}`)
	require.Equal(t, http.StatusOK, switchResp.StatusCode, composeReadBody(t, switchResp))
	var switchBody struct {
		AccessToken string `json:"access_token"`
	}
	composeDecodeJSON(t, switchResp.Body, &switchBody)
	require.NotEmpty(t, switchBody.AccessToken)
	return switchBody.AccessToken, altProfileID
}

func chatIDsFromList(items []composeChatListItem) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.ChatID)
	}
	return out
}

func composePostJSON(t *testing.T, client *http.Client, url, accessToken, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader([]byte(body)))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := client.Do(req)
	require.NoError(t, err)
	return resp
}

func composeReadBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return string(b)
}

func composeDecodeJSON(t *testing.T, r io.Reader, dst any) {
	t.Helper()
	require.NoError(t, json.NewDecoder(r).Decode(dst))
}

func composeNestedJSONString(t *testing.T, resp *http.Response, keys ...string) string {
	t.Helper()
	defer resp.Body.Close()
	var raw map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&raw))
	cur := raw
	for i, k := range keys {
		if i == len(keys)-1 {
			return cur[k].(string)
		}
		cur = cur[k].(map[string]any)
	}
	t.Fatal("unreachable")
	return ""
}
