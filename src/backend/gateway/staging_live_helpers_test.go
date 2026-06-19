package main

import (
	"os"
	"strings"
	"testing"
)

// liveStagingEnabled is true when VOICE_STAGING_API_URL is set (opt-in staging E2E).
func liveStagingEnabled() bool {
	return strings.TrimSpace(os.Getenv("VOICE_STAGING_API_URL")) != ""
}

func liveStagingBaseURL() string {
	return strings.TrimRight(strings.TrimSpace(os.Getenv("VOICE_STAGING_API_URL")), "/")
}

// liveStagingWebhookPingURL must be reachable from the staging Bot pod and return {"content":"pong"}.
func liveStagingWebhookPingURL(t *testing.T) string {
	t.Helper()
	u := strings.TrimSpace(os.Getenv("VOICE_STAGING_WEBHOOK_PING_URL"))
	if u == "" {
		t.Skip("set VOICE_STAGING_WEBHOOK_PING_URL to a URL reachable from staging voice-bot (handler returns {\"content\":\"pong\"})")
	}
	return u
}
