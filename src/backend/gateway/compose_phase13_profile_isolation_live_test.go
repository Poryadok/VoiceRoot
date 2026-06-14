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

// TestComposePhase13_ProfileFriendIsolation_live documents friends are scoped per profile (no cross-leak).
func TestComposePhase13_ProfileFriendIsolation_live(t *testing.T) {
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
