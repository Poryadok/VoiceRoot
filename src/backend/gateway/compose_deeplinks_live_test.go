package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestComposeDeepLinks_live: universal invite link HTML redirect and authenticated resolve.
func TestComposeDeepLinks_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()
	n := time.Now().UnixNano()

	aasaReq, err := http.NewRequest(http.MethodGet, base+"/.well-known/apple-app-site-association", nil)
	require.NoError(t, err)
	aasaResp, err := client.Do(aasaReq)
	require.NoError(t, err)
	defer aasaResp.Body.Close()
	require.Equal(t, http.StatusOK, aasaResp.StatusCode)
	require.Contains(t, aasaResp.Header.Get("Content-Type"), "application/json")
	var aasa map[string]any
	require.NoError(t, json.NewDecoder(aasaResp.Body).Decode(&aasa))
	applinks, ok := aasa["applinks"].(map[string]any)
	require.True(t, ok)
	details, ok := applinks["details"].([]any)
	require.True(t, ok)
	require.NotEmpty(t, details)
	appID, _ := details[0].(map[string]any)["appID"].(string)
	require.Contains(t, appID, ".")

	assetReq, err := http.NewRequest(http.MethodGet, base+"/.well-known/assetlinks.json", nil)
	require.NoError(t, err)
	assetResp, err := client.Do(assetReq)
	require.NoError(t, err)
	defer assetResp.Body.Close()
	require.Equal(t, http.StatusOK, assetResp.StatusCode)
	require.Contains(t, assetResp.Header.Get("Content-Type"), "application/json")
	var assets []map[string]any
	require.NoError(t, json.NewDecoder(assetResp.Body).Decode(&assets))
	require.NotEmpty(t, assets)
	target, ok := assets[0]["target"].(map[string]any)
	require.True(t, ok)
	require.NotEmpty(t, target["package_name"])
	fps, ok := target["sha256_cert_fingerprints"].([]any)
	require.True(t, ok)
	require.NotEmpty(t, fps)

	ownerSess := registerComposeUser(t, client, base, formatComposeEmail("p18-deeplink-owner", n), "VoiceQaTest1!")
	spaceID := createComposeSpace(t, client, base, ownerSess.AccessToken, "Deep Link QA", "deep-links")
	inv := createComposeSpaceInvite(t, client, base, ownerSess.AccessToken, spaceID)
	require.NotEmpty(t, inv.Code)

	invitePageURL := base + "/invite/" + inv.Code
	inviteReq, err := http.NewRequest(http.MethodGet, invitePageURL, nil)
	require.NoError(t, err)
	inviteResp, err := client.Do(inviteReq)
	require.NoError(t, err)
	defer inviteResp.Body.Close()
	inviteBody, err := io.ReadAll(inviteResp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, inviteResp.StatusCode)
	require.Contains(t, inviteResp.Header.Get("Content-Type"), "text/html")
	require.Contains(t, string(inviteBody), "voice://invite/"+inv.Code)

	resolveURL := base + "/api/v1/links/resolve?url=" + url.QueryEscape("https://voice.gg/invite/"+inv.Code)
	resolveReq, err := http.NewRequest(http.MethodGet, resolveURL, nil)
	require.NoError(t, err)
	resolveReq.Header.Set("Authorization", "Bearer "+ownerSess.AccessToken)
	resolveResp, err := client.Do(resolveReq)
	require.NoError(t, err)
	defer resolveResp.Body.Close()
	require.Equal(t, http.StatusOK, resolveResp.StatusCode)

	var resolved map[string]any
	require.NoError(t, json.NewDecoder(resolveResp.Body).Decode(&resolved))
	require.Equal(t, "invite", resolved["kind"])
	require.Equal(t, inv.Code, resolved["invite_code"])
	require.Equal(t, spaceID, resolved["space_id"])
}
