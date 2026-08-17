package main

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func newDeepLinksContractGateway(t *testing.T, rec *recordingSpaceInvites) http.Handler {
	t.Helper()
	spaceClient, cleanup := startBufconnSpaceInvitesClient(t, rec)
	t.Cleanup(cleanup)
	return newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{space: spaceClient}},
		restUpstreams: map[string]http.Handler{
			"spaces": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }),
		},
	})
}

func TestDeepLinkInviteHTMLRedirect(t *testing.T) {
	t.Parallel()

	h := newDeepLinksContractGateway(t, &recordingSpaceInvites{})
	resp := performRequest(h, http.MethodGet, "/invite/secret-code", "", nil)

	require.Equal(t, http.StatusOK, resp.Code)
	require.Contains(t, resp.Header().Get("Content-Type"), "text/html")
	body := resp.Body.String()
	require.Contains(t, body, "voice://invite/secret-code")
}

func TestDeepLinkResolveRequiresAuth(t *testing.T) {
	t.Parallel()

	h := newDeepLinksContractGateway(t, &recordingSpaceInvites{})
	target := url.QueryEscape("https://voice.gg/invite/secret-code")
	resp := performRequest(h, http.MethodGet, "/api/v1/links/resolve?url="+target, "", nil)

	require.Equal(t, http.StatusUnauthorized, resp.Code)
}

func TestDeepLinkResolveInvite(t *testing.T) {
	t.Parallel()

	rec := &recordingSpaceInvites{}
	h := newDeepLinksContractGateway(t, rec)
	target := url.QueryEscape("https://voice.gg/invite/secret-code")
	resp := performRequest(h, http.MethodGet, "/api/v1/links/resolve?url="+target, "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})

	require.Equal(t, http.StatusOK, resp.Code)
	require.NotNil(t, rec.lastGet)
	require.Equal(t, "secret-code", rec.lastGet.GetCode())

	var payload resolveDeepLinkResponse
	decodeJSON(t, resp.Body, &payload)
	require.Equal(t, string(DeepLinkKindInvite), payload.Kind)
	require.Equal(t, "secret-code", payload.InviteCode)
	require.Equal(t, "space-1", payload.SpaceID)
}

func TestDeepLinkResolveInvalidURLParam(t *testing.T) {
	t.Parallel()

	h := newDeepLinksContractGateway(t, &recordingSpaceInvites{})

	t.Run("missing url", func(t *testing.T) {
		t.Parallel()
		resp := performRequest(h, http.MethodGet, "/api/v1/links/resolve", "", map[string]string{
			"Authorization": "Bearer valid-user-token",
		})
		require.Equal(t, http.StatusBadRequest, resp.Code)
	})

	t.Run("unparseable url", func(t *testing.T) {
		t.Parallel()
		resp := performRequest(h, http.MethodGet, "/api/v1/links/resolve?url="+url.QueryEscape("not-a-deeplink"), "", map[string]string{
			"Authorization": "Bearer valid-user-token",
		})
		require.Equal(t, http.StatusBadRequest, resp.Code)
		require.True(t, strings.Contains(resp.Body.String(), "error"))
	})
}

func TestWellKnownAppleAppSiteAssociation_DefaultPlaceholders(t *testing.T) {
	h := newDeepLinksContractGateway(t, &recordingSpaceInvites{})
	resp := performRequest(h, http.MethodGet, "/.well-known/apple-app-site-association", "", nil)

	require.Equal(t, http.StatusOK, resp.Code)
	require.Contains(t, resp.Header().Get("Content-Type"), "application/json")

	var payload map[string]any
	decodeJSON(t, resp.Body, &payload)
	details := wellKnownAASADetails(t, payload)
	require.Equal(t, "TEAMID.gg.voice.app", details["appID"])
	paths, ok := details["paths"].([]any)
	require.True(t, ok)
	require.Contains(t, paths, "/invite/*")
	require.Contains(t, paths, "/s/*")
	require.Contains(t, paths, "/ch/*")
	require.Contains(t, paths, "/u/*")
	require.Contains(t, paths, "/dm/*")
}

func TestWellKnownAppleAppSiteAssociation_EnvOverride(t *testing.T) {
	t.Setenv("GATEWAY_IOS_TEAM_ID", "ABCD1234")
	t.Setenv("GATEWAY_IOS_BUNDLE_ID", "voice.app.voiceFrontend")

	h := newDeepLinksContractGateway(t, &recordingSpaceInvites{})
	resp := performRequest(h, http.MethodGet, "/.well-known/apple-app-site-association", "", nil)

	require.Equal(t, http.StatusOK, resp.Code)
	var payload map[string]any
	decodeJSON(t, resp.Body, &payload)
	details := wellKnownAASADetails(t, payload)
	require.Equal(t, "ABCD1234.voice.app.voiceFrontend", details["appID"])
}

func TestWellKnownAssetLinks_DefaultPlaceholders(t *testing.T) {
	h := newDeepLinksContractGateway(t, &recordingSpaceInvites{})
	resp := performRequest(h, http.MethodGet, "/.well-known/assetlinks.json", "", nil)

	require.Equal(t, http.StatusOK, resp.Code)
	require.Contains(t, resp.Header().Get("Content-Type"), "application/json")

	target := wellKnownAssetLinksTarget(t, resp.Body)
	require.Equal(t, "android_app", target["namespace"])
	require.Equal(t, "gg.voice.app", target["package_name"])
	fps, ok := target["sha256_cert_fingerprints"].([]any)
	require.True(t, ok)
	require.Equal(t, []any{"PLACEHOLDER"}, fps)
}

func TestWellKnownAssetLinks_EnvOverride(t *testing.T) {
	t.Setenv("GATEWAY_ANDROID_PACKAGE_NAME", "voice.app.voice_frontend")
	t.Setenv("GATEWAY_ANDROID_SHA256_CERT_FINGERPRINTS", "AA:BB:CC:DD,EE:FF:00:11")

	h := newDeepLinksContractGateway(t, &recordingSpaceInvites{})
	resp := performRequest(h, http.MethodGet, "/.well-known/assetlinks.json", "", nil)

	require.Equal(t, http.StatusOK, resp.Code)
	target := wellKnownAssetLinksTarget(t, resp.Body)
	require.Equal(t, "voice.app.voice_frontend", target["package_name"])
	fps, ok := target["sha256_cert_fingerprints"].([]any)
	require.True(t, ok)
	require.Equal(t, []any{"AA:BB:CC:DD", "EE:FF:00:11"}, fps)
}

func wellKnownAASADetails(t *testing.T, payload map[string]any) map[string]any {
	t.Helper()
	applinks, ok := payload["applinks"].(map[string]any)
	require.True(t, ok)
	details, ok := applinks["details"].([]any)
	require.True(t, ok)
	require.NotEmpty(t, details)
	first, ok := details[0].(map[string]any)
	require.True(t, ok)
	return first
}

func wellKnownAssetLinksTarget(t *testing.T, body io.Reader) map[string]any {
	t.Helper()
	var entries []map[string]any
	decodeJSON(t, body, &entries)
	require.NotEmpty(t, entries)
	target, ok := entries[0]["target"].(map[string]any)
	require.True(t, ok)
	return target
}
