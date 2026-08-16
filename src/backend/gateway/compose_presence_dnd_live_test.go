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

// TestComposePresenceDNDInvisible_live covers PR-02: DND + custom status visible to friends;
// invisible appears offline to peers (docs/features/presence.md).
func TestComposePresenceDNDInvisible_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()

	n := time.Now().UnixNano()
	sessA := registerComposeUser(t, client, base, formatComposeEmail("presence-dnd-a", n), "VoiceQaTest1!")
	sessB := registerComposeUser(t, client, base, formatComposeEmail("presence-dnd-b", n), "VoiceQaTest1!")

	sendComposeFriendInvitation(t, client, base, sessA.AccessToken, sessB.ProfileID)
	acceptComposeFriendInvitation(t, client, base, sessB.AccessToken, sessA.ProfileID)

	wsA := dialComposeRealtimeWS(t, base, sessA.AccessToken)
	waitComposeWSHello(t, wsA)
	wsB := dialComposeRealtimeWS(t, base, sessB.AccessToken)
	waitComposeWSHello(t, wsB)

	composePatchPresence(t, client, base, sessB.AccessToken, "dnd", "focus mode")
	require.Eventually(t, func() bool {
		st := composeGetPresence(t, client, base, sessA.AccessToken, sessB.ProfileID)
		return st.Status == "dnd" && st.CustomStatus == "focus mode"
	}, 20*time.Second, 400*time.Millisecond, "friend should see DND + custom status")

	// Friend B should receive live presence_update for DND without sharing a chat subscription.
	_ = wsA.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
	for {
		if _, _, err := wsA.ReadMessage(); err != nil {
			break
		}
	}
	composePatchPresence(t, client, base, sessB.AccessToken, "online", "")
	deadline := time.Now().Add(15 * time.Second)
	gotOnlineWS := false
	for time.Now().Before(deadline) {
		_ = wsA.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, data, err := wsA.ReadMessage()
		if err != nil {
			continue
		}
		var env struct {
			Op string          `json:"op"`
			D  json.RawMessage `json:"d"`
		}
		if json.Unmarshal(data, &env) != nil || env.Op != "presence_update" {
			continue
		}
		var d struct {
			ProfileID string `json:"profile_id"`
			Status    string `json:"status"`
		}
		if json.Unmarshal(env.D, &d) != nil {
			continue
		}
		if d.ProfileID == sessB.ProfileID && d.Status == "online" {
			gotOnlineWS = true
			break
		}
	}
	require.True(t, gotOnlineWS, "friend A WS should receive presence_update for B without shared chat")

	composePatchPresence(t, client, base, sessB.AccessToken, "invisible", "hidden")
	require.Eventually(t, func() bool {
		st := composeGetPresence(t, client, base, sessA.AccessToken, sessB.ProfileID)
		return st.Status == "" && st.CustomStatus == ""
	}, 20*time.Second, 400*time.Millisecond, "invisible must look offline to friend")

	self := composeGetPresence(t, client, base, sessB.AccessToken, sessB.ProfileID)
	require.Equal(t, "invisible", self.Status, "self should still see invisible")
}

type composePresenceView struct {
	Status       string
	CustomStatus string
}

func composeGetPresence(t *testing.T, client *http.Client, base, accessToken, profileID string) composePresenceView {
	t.Helper()
	url := base + "/api/v1/users/profiles/" + profileID + "/presence"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "presence body=%s", string(body))

	var parsed struct {
		PresenceStatus struct {
			Status       string `json:"status"`
			CustomStatus string `json:"custom_status"`
		} `json:"presence_status"`
	}
	require.NoError(t, json.Unmarshal(body, &parsed))
	return composePresenceView{
		Status:       parsed.PresenceStatus.Status,
		CustomStatus: parsed.PresenceStatus.CustomStatus,
	}
}

func composePatchPresence(t *testing.T, client *http.Client, base, accessToken, status, customStatus string) {
	t.Helper()
	payload := map[string]any{"status": status}
	if customStatus != "" {
		payload["custom_status"] = customStatus
	}
	body, err := json.Marshal(payload)
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPatch, base+"/api/v1/users/me/presence", bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "PATCH me/presence body=%s", string(respBody))
}
