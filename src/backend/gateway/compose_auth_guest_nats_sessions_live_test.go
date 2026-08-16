package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
)

func liveNATSURL() string {
	if u := strings.TrimSpace(os.Getenv("VOICE_NATS_URL")); u != "" {
		return u
	}
	return "nats://127.0.0.1:4222"
}

// TestComposeConvertGuestNATS_live subscribes to core NATS subject user.guest_converted
// and asserts Auth publishes after convert-guest (AU-14).
//
// Opt-in: VOICE_RUN_LIVE_COMPOSE=true VOICE_API_BASE_URL=http://127.0.0.1:18080
func TestComposeConvertGuestNATS_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	nc, err := nats.Connect(liveNATSURL(), nats.Timeout(5*time.Second))
	require.NoError(t, err)
	defer nc.Close()

	sub, err := nc.SubscribeSync("user.guest_converted")
	require.NoError(t, err)
	defer sub.Unsubscribe()

	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()

	const guestPassword = "VoiceQaTest1!"
	const newPassword = "VoiceQaNewPass1!"
	guestSess := registerComposeGuest(t, client, base, guestPassword)

	email := formatComposeEmail("guest-nats", time.Now().UnixNano())
	convertBody, err := json.Marshal(map[string]string{
		"email":    email,
		"password": newPassword,
	})
	require.NoError(t, err)
	convertReq, err := http.NewRequest(http.MethodPost, base+"/api/v1/auth/convert-guest", bytes.NewReader(convertBody))
	require.NoError(t, err)
	convertReq.Header.Set("Authorization", "Bearer "+guestSess.AccessToken)
	convertReq.Header.Set("Content-Type", "application/json")
	convertResp, err := client.Do(convertReq)
	require.NoError(t, err)
	defer convertResp.Body.Close()
	convertRaw, _ := io.ReadAll(convertResp.Body)
	require.Equal(t, http.StatusOK, convertResp.StatusCode, "body=%s", string(convertRaw))

	msg, err := sub.NextMsg(10 * time.Second)
	require.NoError(t, err, "expected user.guest_converted on core NATS")
	require.Contains(t, string(msg.Data), guestSess.AccountID)
}

// TestComposeAuthSessions_live: list sessions → revoke other → refresh fails (AU-12).
func TestComposeAuthSessions_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()
	n := time.Now().UnixNano()
	email := formatComposeEmail("auth-sess", n)
	password := "VoiceQaTest1!"

	first := registerComposeUser(t, client, base, email, password)
	second := loginComposeUser(t, client, base, email, password)

	listReq, err := http.NewRequest(http.MethodGet, base+"/api/v1/auth/sessions", nil)
	require.NoError(t, err)
	listReq.Header.Set("Authorization", "Bearer "+second.AccessToken)
	listResp, err := client.Do(listReq)
	require.NoError(t, err)
	defer listResp.Body.Close()
	listRaw, _ := io.ReadAll(listResp.Body)
	require.Equal(t, http.StatusOK, listResp.StatusCode, "body=%s", string(listRaw))

	var list struct {
		Sessions []struct {
			ID      string `json:"id"`
			Current bool   `json:"current"`
		} `json:"sessions"`
	}
	require.NoError(t, json.Unmarshal(listRaw, &list))
	require.GreaterOrEqual(t, len(list.Sessions), 2)

	var otherID string
	for _, s := range list.Sessions {
		if !s.Current {
			otherID = s.ID
			break
		}
	}
	require.NotEmpty(t, otherID)

	revokeReq, err := http.NewRequest(http.MethodPost, base+"/api/v1/auth/sessions/"+otherID+"/revoke", nil)
	require.NoError(t, err)
	revokeReq.Header.Set("Authorization", "Bearer "+second.AccessToken)
	revokeResp, err := client.Do(revokeReq)
	require.NoError(t, err)
	defer revokeResp.Body.Close()
	require.Equal(t, http.StatusNoContent, revokeResp.StatusCode)

	refreshPayload, err := json.Marshal(map[string]string{
		"refresh_token":    first.RefreshToken,
		"device_info_json": `{"platform":"go-live-test"}`,
	})
	require.NoError(t, err)
	refreshResp, err := client.Post(base+"/api/v1/auth/refresh", "application/json", bytes.NewReader(refreshPayload))
	require.NoError(t, err)
	defer refreshResp.Body.Close()
	refreshRaw, _ := io.ReadAll(refreshResp.Body)
	require.Equal(t, http.StatusUnauthorized, refreshResp.StatusCode, "body=%s", string(refreshRaw))
}

func loginComposeUser(t *testing.T, client *http.Client, base, email, password string) authSessionResponse {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"email":            email,
		"password":         password,
		"device_info_json": `{"platform":"go-live-test-b"}`,
	})
	require.NoError(t, err)
	resp, err := client.Post(base+"/api/v1/auth/login", "application/json", bytes.NewReader(payload))
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "login body=%s", string(raw))
	var envelope authSessionEnvelope
	require.NoError(t, json.Unmarshal(raw, &envelope))
	require.NotEmpty(t, envelope.Session.AccessToken)
	require.NotEmpty(t, envelope.Session.RefreshToken)
	return envelope.Session
}
