package main

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestComposeGuestReminder_live documents AU-07 server cadence:
// guest GET should_show → mark → same-day should_show false.
func TestComposeGuestReminder_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()

	guest := registerComposeGuest(t, client, base, "VoiceQaTest1!")

	first := getComposeGuestReminder(t, client, base, guest.AccessToken)
	require.True(t, first.ShouldShow, "unset guest_reminder_last_shown_at must allow show")

	markComposeGuestReminder(t, client, base, guest.AccessToken)

	second := getComposeGuestReminder(t, client, base, guest.AccessToken)
	require.False(t, second.ShouldShow, "same-day mark must suppress reminder")
}

type composeGuestReminder struct {
	ShouldShow bool `json:"should_show"`
}

func getComposeGuestReminder(t *testing.T, client *http.Client, base, accessToken string) composeGuestReminder {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, base+"/api/v1/auth/guest-reminder", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "body=%s", string(raw))
	var out composeGuestReminder
	require.NoError(t, json.Unmarshal(raw, &out))
	return out
}

func markComposeGuestReminder(t *testing.T, client *http.Client, base, accessToken string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/auth/guest-reminder/mark", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent,
		"mark status=%d", resp.StatusCode)
}
