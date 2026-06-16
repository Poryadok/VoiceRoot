package main

import (
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
