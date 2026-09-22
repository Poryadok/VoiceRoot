package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestComposePhoneSync_live documents that phone-book discovery is post-alpha and
// therefore rejected consistently by the public gateway contract.
func TestComposePhoneSync_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 90 * time.Second}
	base := liveGatewayBaseURL()
	n := time.Now().UnixNano()

	phoneHash := formatComposePhoneHash("p11-phone", n)
	caller := registerComposeUserWithPhone(t, client, base, formatComposeEmail("p11-phone", n), phoneHash, "VoiceQaTest1!")
	syncComposePhoneContactsUnavailable(t, client, base, caller.AccessToken, phoneHash)
}

func formatComposePhoneHash(prefix string, n int64) string {
	// auth_db.accounts.phone is VARCHAR(32); store deterministic test hash.
	hash := fmt.Sprintf("%s-%016x", prefix, uint64(n))
	if len(hash) > 32 {
		return hash[:32]
	}
	return hash
}

func registerComposeUserWithPhone(t *testing.T, client *http.Client, base, email, phone, password string) authSessionResponse {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"email":            email,
		"phone":            phone,
		"password":         password,
		"guest":            false,
		"device_info_json": `{"platform":"go-live-test"}`,
	})
	require.NoError(t, err)

	resp, err := client.Post(base+"/api/v1/auth/register", "application/json", bytes.NewReader(payload))
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode,
		"register %s: status=%d body=%s", email, resp.StatusCode, string(raw))

	var envelope authSessionEnvelope
	require.NoError(t, json.Unmarshal(raw, &envelope))
	sess := envelope.Session
	require.NotEmpty(t, sess.AccessToken)
	require.NotEmpty(t, sess.ProfileID)
	return sess
}

func syncComposePhoneContactsUnavailable(t *testing.T, client *http.Client, base, accessToken, phoneHash string) {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"hashed_phone_numbers": []string{phoneHash},
	})
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/friends/contacts/sync", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusConflict, resp.StatusCode, "sync phone contacts body=%s", string(raw))
	require.JSONEq(t, `{"error_code":"phone_contact_sync_unavailable","message":"phone_contact_sync_unavailable"}`, string(raw))
}
