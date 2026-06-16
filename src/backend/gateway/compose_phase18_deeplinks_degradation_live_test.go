package main

import (
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestComposePhase18DeepLinksResolveWhenSearchDown_live ensures link resolve works without Search.
func TestComposePhase18DeepLinksResolveWhenSearchDown_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	if os.Getenv("VOICE_SEARCH_DEGRADATION_TEST") != "true" {
		t.Skip("set VOICE_SEARCH_DEGRADATION_TEST=true with search container stopped")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()
	n := time.Now().UnixNano()

	ownerSess := registerComposeUser(t, client, base, formatComposeEmail("p18-deg-owner", n), "VoiceQaTest1!")
	spaceID := createComposeSpace(t, client, base, ownerSess.AccessToken, "Deg Link QA", "phase 18 deg")
	inv := createComposeSpaceInvite(t, client, base, ownerSess.AccessToken, spaceID)

	resolveURL := base + "/api/v1/links/resolve?url=" + url.QueryEscape("https://voice.gg/invite/"+inv.Code)
	req, err := http.NewRequest(http.MethodGet, resolveURL, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+ownerSess.AccessToken)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
}
