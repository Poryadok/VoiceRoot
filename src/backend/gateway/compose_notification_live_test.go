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

// TestComposeNotificationRegisterDevice_live asserts Gateway exposes POST /api/v1/notifications/register-device.
func TestComposeNotificationRegisterDevice_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()

	n := time.Now().UnixNano()
	sess := registerComposeUser(t, client, base, formatComposeEmail("notify-reg", n), "VoiceQaTest1!")

	body, err := json.Marshal(map[string]string{
		"platform":     "web",
		"token":        "qa-fcm-token-" + formatComposeEmail("tok", n),
		"push_service": "fcm",
	})
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/notifications/register-device", bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+sess.AccessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	require.NotEqual(t, http.StatusNotFound, resp.StatusCode,
		"register-device must be wired when notification service is in compose; body=%s", string(raw))
}
