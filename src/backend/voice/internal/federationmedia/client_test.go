package federationmedia

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/federation/mediaauthority"
)

func TestVoiceClientResolvesTrustedScopeAndExchangesOnlyNarrowGrant(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	now := time.Now()
	input := mediaauthority.RouteRequest{AccountID: uuid.NewString(), ProfileID: uuid.NewString(), SpaceID: uuid.NewString(), ResourceID: uuid.NewString(), RoomName: "canonical-room", SessionEpoch: 5, CanPublish: true}
	node := uuid.NewString()
	resolved := input.Request()
	resolved.RoutingGeneration = 7
	resolved.ApplicationID = uuid.NewString()
	resolved.EnvironmentID = uuid.NewString()
	resolved.BindingID = uuid.NewString()
	resolved.InstallationID = uuid.NewString()
	grant := mediaauthority.Grant{Version: 1, Issuer: "master", Audience: "voice-node-media", Environment: "sandbox", NodeID: node, SpaceID: input.SpaceID, Generation: 1, AuthorityEpoch: 1, AccountID: input.AccountID, ProfileID: input.ProfileID, ResourceID: input.ResourceID, SessionEpoch: 5, RoomName: input.RoomName, CanPublish: true, RoutingGeneration: 7, Nonce: uuid.NewString(), ApplicationID: resolved.ApplicationID, EnvironmentID: resolved.EnvironmentID, BindingID: resolved.BindingID, InstallationID: resolved.InstallationID, IssuedAt: now.UnixMilli(), ExpiresAt: now.Add(30 * time.Second).UnixMilli()}
	credential, err := mediaauthority.Sign(private, "current", grant, now)
	require.NoError(t, err)
	var exchanged bool
	edge := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Empty(t, r.Header.Get("Authorization"), "master user tokens and credentials never reach node")
		require.Len(t, r.TLS.PeerCertificates, 0, "trusted master Voice client certificate is not sent to node")
		var request mediaauthority.ExchangeRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
		require.True(t, request.Credential == credential)
		require.Equal(t, input.ProfileID, request.ProfileID)
		require.Equal(t, input.RoomName, request.RoomName)
		exchanged = true
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(mediaauthority.ExchangeResult{JWT: "opaque-node-jwt", LivekitURL: "wss://media.node.test", ExpiresAt: grant.ExpiresAt - 1000})
	}))
	defer edge.Close()
	status := 200
	mode := ""
	master := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if status != 200 {
			w.WriteHeader(status)
			return
		}
		switch r.URL.Path {
		case "/internal/v1/media-routes":
			var request mediaauthority.RouteRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
			require.Equal(t, input, request)
			if mode == "not-hosted" {
				_ = json.NewEncoder(w).Encode(mediaauthority.RouteResult{Version: 1, Hosted: false})
				return
			}
			if mode == "missing-version" {
				_ = json.NewEncoder(w).Encode(mediaauthority.RouteResult{})
				return
			}
			if mode == "wrong-profile" {
				other := resolved
				other.ProfileID = uuid.NewString()
				_ = json.NewEncoder(w).Encode(mediaauthority.RouteResult{Version: 1, Hosted: true, Request: &other})
				return
			}
			_ = json.NewEncoder(w).Encode(mediaauthority.RouteResult{Version: 1, Hosted: true, Request: &resolved})
		case "/internal/v1/media-grants":
			var request mediaauthority.Request
			require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
			require.Equal(t, resolved, request)
			_ = json.NewEncoder(w).Encode(mediaauthority.Result{Credential: credential, NodeID: node, NodeEndpoint: edge.URL, SpaceID: input.SpaceID, ResourceID: input.ResourceID, RoomName: input.RoomName, RoutingGeneration: 7, ExpiresAt: grant.ExpiresAt})
		default:
			w.WriteHeader(404)
		}
	}))
	defer master.Close()
	mclient := master.Client()
	mclient.Timeout = 2 * time.Second
	// A real deployment has a distinct client leaf. This fixture cert is enough
	// to exercise client configuration; master issuer role mTLS is PG-tested.
	mclient.Transport.(*http.Transport).TLSClientConfig.Certificates = []tls.Certificate{master.TLS.Certificates[0]}
	nclient := edge.Client()
	nclient.Timeout = 2 * time.Second
	client, err := New(Config{MasterURL: master.URL, Issuer: grant.Issuer, Environment: grant.Environment, Keys: map[string]ed25519.PublicKey{"current": public}, MasterClient: mclient, NodeClient: nclient})
	require.NoError(t, err)
	result, err := client.JoinToken(context.Background(), input)
	require.NoError(t, err)
	require.True(t, exchanged)
	require.True(t, result.JWT != "")
	status = 403
	_, err = client.JoinToken(context.Background(), input)
	require.ErrorIs(t, err, ErrDenied)
	status = 404
	_, err = client.JoinToken(context.Background(), input)
	require.ErrorIs(t, err, ErrUnavailable)
	status = 503
	_, err = client.JoinToken(context.Background(), input)
	require.ErrorIs(t, err, ErrUnavailable)
	status = 200
	mode = "missing-version"
	_, err = client.JoinToken(context.Background(), input)
	require.ErrorIs(t, err, ErrUnavailable, "unknown wire response cannot enable hosted fallback")
	mode = "wrong-profile"
	_, err = client.JoinToken(context.Background(), input)
	require.ErrorIs(t, err, ErrDenied)
	mode = "not-hosted"
	_, err = client.JoinToken(context.Background(), input)
	require.ErrorIs(t, err, ErrNotHosted)
	nclient.Transport.(*http.Transport).TLSClientConfig.Certificates = []tls.Certificate{master.TLS.Certificates[0]}
	_, err = New(Config{MasterURL: master.URL, Issuer: grant.Issuer, Environment: grant.Environment, Keys: map[string]ed25519.PublicKey{"current": public}, MasterClient: mclient, NodeClient: nclient})
	require.ErrorIs(t, err, ErrUnavailable, "node transport must never carry a master client certificate")
}

func TestFederatedMediaRuntimeIsOptInAndRejectsIncompleteConfiguration(t *testing.T) {
	client, err := LoadFromEnv(func(string) string { return "" })
	require.NoError(t, err)
	require.Nil(t, client)
	client, err = LoadFromEnv(func(string) string { return "missing-config-file" })
	require.ErrorIs(t, err, ErrUnavailable)
	require.Nil(t, client)
}
