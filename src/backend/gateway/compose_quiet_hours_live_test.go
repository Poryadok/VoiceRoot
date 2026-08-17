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

// TestComposeQuietHours_live documents NT-04:
// SetQuietHours / GetQuietHours round-trip via Gateway.
// Full voice_member_joined push suppression under quiet hours is covered by
// Notification unit delivery tests; compose asserts persistence contract.
func TestComposeQuietHours_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()
	n := time.Now().UnixNano()
	sess := registerComposeUser(t, client, base, formatComposeEmail("nt04-qh", n), "VoiceQaTest1!")

	putBody, err := json.Marshal(map[string]any{
		"enabled":           true,
		"start_time":        "22:00",
		"end_time":          "08:00",
		"timezone":          "UTC",
		"override_mentions": false,
	})
	require.NoError(t, err)
	putReq, err := http.NewRequest(http.MethodPut, base+"/api/v1/notifications/quiet-hours", bytes.NewReader(putBody))
	require.NoError(t, err)
	putReq.Header.Set("Authorization", "Bearer "+sess.AccessToken)
	putReq.Header.Set("Content-Type", "application/json")
	putResp, err := client.Do(putReq)
	require.NoError(t, err)
	defer putResp.Body.Close()
	putRaw, _ := io.ReadAll(putResp.Body)
	require.True(t, putResp.StatusCode == http.StatusOK || putResp.StatusCode == http.StatusNoContent,
		"set quiet hours body=%s", string(putRaw))

	getReq, err := http.NewRequest(http.MethodGet, base+"/api/v1/notifications/quiet-hours", nil)
	require.NoError(t, err)
	getReq.Header.Set("Authorization", "Bearer "+sess.AccessToken)
	getResp, err := client.Do(getReq)
	require.NoError(t, err)
	defer getResp.Body.Close()
	getRaw, _ := io.ReadAll(getResp.Body)
	require.Equal(t, http.StatusOK, getResp.StatusCode, "get quiet hours body=%s", string(getRaw))

	var parsed struct {
		QuietHours struct {
			Enabled          bool   `json:"enabled"`
			StartTime        string `json:"start_time"`
			EndTime          string `json:"end_time"`
			Timezone         string `json:"timezone"`
			OverrideMentions bool   `json:"override_mentions"`
		} `json:"quiet_hours"`
	}
	require.NoError(t, json.Unmarshal(getRaw, &parsed))
	require.True(t, parsed.QuietHours.Enabled)
	require.Equal(t, "22:00", parsed.QuietHours.StartTime)
	require.Equal(t, "08:00", parsed.QuietHours.EndTime)
	require.Equal(t, "UTC", parsed.QuietHours.Timezone)
	require.False(t, parsed.QuietHours.OverrideMentions)
}
