package main

import (
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestComposeDeveloperPortal_live documents developer portal SPA routes + OAuth authorize wiring on compose.
func TestComposeDeveloperPortal_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}

	client := &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	portalBase := liveDeveloperPortalBaseURL()
	gatewayBase := liveGatewayBaseURL()

	rootResp, err := client.Get(portalBase + "/")
	require.NoError(t, err)
	rootResp.Body.Close()
	if rootResp.StatusCode != http.StatusOK {
		t.Skipf("developer portal unavailable at %s (status=%d)", portalBase, rootResp.StatusCode)
	}

	callbackResp, err := client.Get(portalBase + "/callback")
	require.NoError(t, err)
	callbackResp.Body.Close()
	require.Equal(t, http.StatusOK, callbackResp.StatusCode, "portal /callback SPA route must be served")

	clientID := strings.TrimSpace(os.Getenv("DEVELOPER_PORTAL_OAUTH_CLIENT_ID"))
	if clientID == "" {
		clientID = "voice-developer-portal"
	}
	redirectURI := portalBase + "/callback"
	authURL := gatewayBase + "/api/v1/auth/oauth2/authorize?" + url.Values{
		"response_type":         {"code"},
		"client_id":             {clientID},
		"redirect_uri":          {redirectURI},
		"state":                 {"compose-portal-smoke"},
		"code_challenge":        {"E9Metb8Jw90bMxjf2c79N8StNyRUHJsrP5jdz0h_AhA"},
		"code_challenge_method": {"S256"},
	}.Encode()
	oauthResp, err := client.Get(authURL)
	require.NoError(t, err)
	oauthResp.Body.Close()
	require.Contains(t, []int{http.StatusOK, http.StatusFound}, oauthResp.StatusCode,
		"OAuth authorize for developer portal client must be wired (status=%d)", oauthResp.StatusCode)
}
