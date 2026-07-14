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

// TestComposeConvertGuest_live registers a guest via gateway, then converts to a regular account.
//
// Opt-in: VOICE_RUN_LIVE_COMPOSE=true VOICE_API_BASE_URL=http://127.0.0.1:18080
func TestComposeConvertGuest_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()

	const guestPassword = "VoiceQaTest1!"
	const newPassword = "VoiceQaNewPass1!"
	guestSess := registerComposeGuest(t, client, base, guestPassword)
	require.Equal(t, http.StatusOK, composeProtectedRouteStatus(t, client, base, guestSess.AccessToken))

	email := formatComposeEmail("guest-convert", time.Now().UnixNano())
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

	var envelope authSessionEnvelope
	require.NoError(t, json.Unmarshal(convertRaw, &envelope))
	converted := envelope.Session
	require.NotEmpty(t, converted.AccessToken)
	require.Equal(t, guestSess.AccountID, converted.AccountID, "convert-guest must keep account_id")
	require.Equal(t, guestSess.ProfileID, converted.ProfileID, "convert-guest must keep profile_id")
	require.NotEqual(t, guestSess.AccessToken, converted.AccessToken)

	require.Equal(t, http.StatusOK, composeProtectedRouteStatus(t, client, base, converted.AccessToken))

	loginPayload, err := json.Marshal(map[string]any{
		"email":            email,
		"password":         newPassword,
		"device_info_json": `{"platform":"go-live-test"}`,
	})
	require.NoError(t, err)
	loginResp, err := client.Post(base+"/api/v1/auth/login", "application/json", bytes.NewReader(loginPayload))
	require.NoError(t, err)
	defer loginResp.Body.Close()
	loginRaw, _ := io.ReadAll(loginResp.Body)
	require.Equal(t, http.StatusOK, loginResp.StatusCode, "login body=%s", string(loginRaw))
	var loginEnvelope authSessionEnvelope
	require.NoError(t, json.Unmarshal(loginRaw, &loginEnvelope))
	require.Equal(t, guestSess.AccountID, loginEnvelope.Session.AccountID)
}

func registerComposeGuest(t *testing.T, client *http.Client, base, password string) authSessionResponse {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"password":         password,
		"guest":            true,
		"device_info_json": `{"platform":"go-live-test"}`,
	})
	require.NoError(t, err)

	resp, err := client.Post(base+"/api/v1/auth/register", "application/json", bytes.NewReader(payload))
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "guest register body=%s", string(raw))

	var envelope authSessionEnvelope
	require.NoError(t, json.Unmarshal(raw, &envelope))
	sess := envelope.Session
	require.NotEmpty(t, sess.AccessToken)
	require.NotEmpty(t, sess.ProfileID)
	require.NotEmpty(t, sess.AccountID)
	return sess
}
