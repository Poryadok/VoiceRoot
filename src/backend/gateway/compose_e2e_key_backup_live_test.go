package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const composeE2EKeyBackupMaxBlobBytes = 512 * 1024

// TestComposeE2EKeyBackup_live documents PUT/GET /api/v1/auth/e2e-key-backup via gateway.
func TestComposeE2EKeyBackup_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 30 * time.Second}
	base := liveGatewayBaseURL()

	n := time.Now().UnixNano()
	sess := registerComposeUser(t, client, base, formatComposeEmail("p15-kb", n), "VoiceQaTest1!")

	blob := fmt.Sprintf("phase15-compose-key-backup-%d", n)
	hint := "qa hint"
	putComposeE2EKeyBackup(t, client, base, sess.AccessToken, blob, hint)

	gotBlob, gotHint := getComposeE2EKeyBackup(t, client, base, sess.AccessToken)
	require.Equal(t, blob, gotBlob)
	require.Equal(t, hint, gotHint)
}

// TestComposeE2EKeyBackup_OversizedRejected_live documents oversized blob is rejected.
func TestComposeE2EKeyBackup_OversizedRejected_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 30 * time.Second}
	base := liveGatewayBaseURL()

	n := time.Now().UnixNano()
	sess := registerComposeUser(t, client, base, formatComposeEmail("p15-kb-big", n), "VoiceQaTest1!")

	oversized := strings.Repeat("x", composeE2EKeyBackupMaxBlobBytes+1)
	status := putComposeE2EKeyBackupStatus(t, client, base, sess.AccessToken, oversized, "")
	require.NotEqual(t, http.StatusOK, status)
	require.NotEqual(t, http.StatusNoContent, status)
}

func putComposeE2EKeyBackup(t *testing.T, client *http.Client, base, accessToken, blob, hint string) {
	t.Helper()
	status := putComposeE2EKeyBackupStatus(t, client, base, accessToken, blob, hint)
	require.Contains(t, []int{http.StatusOK, http.StatusNoContent}, status, "PUT e2e-key-backup")
}

func putComposeE2EKeyBackupStatus(t *testing.T, client *http.Client, base, accessToken, blob, hint string) int {
	t.Helper()
	body := map[string]string{"encrypted_blob": blob}
	if hint != "" {
		body["password_hint"] = hint
	}
	payload, err := json.Marshal(body)
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPut, base+"/api/v1/auth/e2e-key-backup", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	_, _ = io.ReadAll(resp.Body)
	return resp.StatusCode
}

func getComposeE2EKeyBackup(t *testing.T, client *http.Client, base, accessToken string) (blob, hint string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, base+"/api/v1/auth/e2e-key-backup", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "GET e2e-key-backup body=%s", string(raw))

	var parsed struct {
		EncryptedBlob string `json:"encrypted_blob"`
		PasswordHint  string `json:"password_hint"`
	}
	require.NoError(t, json.Unmarshal(raw, &parsed))
	return parsed.EncryptedBlob, parsed.PasswordHint
}
