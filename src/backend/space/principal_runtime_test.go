package main

import (
	"crypto/tls"
	"encoding/pem"
	"github.com/stretchr/testify/require"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestOwnershipRoleTLSConfigFailsClosed(t *testing.T) {
	t.Setenv("ROLE_PRINCIPAL_TLS_CA_FILE", "")
	t.Setenv("ROLE_PRINCIPAL_TLS_SERVER_NAME", "role.internal")
	config, err := ownershipRoleTLSFromEnv()
	require.NoError(t, err)
	require.False(t, config.InsecureSkipVerify)
	require.Equal(t, "role.internal", config.ServerName)
	require.GreaterOrEqual(t, config.MinVersion, uint16(tls.VersionTLS12))
	t.Setenv("ROLE_PRINCIPAL_TLS_CA_FILE", t.TempDir()+"/absent.pem")
	_, err = ownershipRoleTLSFromEnv()
	require.Error(t, err)
	invalid := filepath.Join(t.TempDir(), "invalid.pem")
	require.NoError(t, os.WriteFile(invalid, []byte("not a CA"), 0600))
	t.Setenv("ROLE_PRINCIPAL_TLS_CA_FILE", invalid)
	_, err = ownershipRoleTLSFromEnv()
	require.Error(t, err)
}

func TestSpacePrincipalJWKSRouteIsPublicReadOnlyAndAbsentWhenDisabled(t *testing.T) {
	disabled := httptest.NewRecorder()
	spaceHTTPHandler("space", principalJWKS{}).ServeHTTP(disabled, httptest.NewRequest(http.MethodGet, "/.well-known/jwks.json", nil))
	require.Equal(t, http.StatusNotFound, disabled.Code)
	handler := spaceHTTPHandler("space", principalJWKS{Keys: []principalJWK{{Kty: "RSA", Kid: "current", Use: "sig", Alg: "RS256", N: "public-n", E: "AQAB"}, {Kty: "RSA", Kid: "next", Use: "sig", Alg: "RS256", N: "other-public-n", E: "AQAB"}}})
	get := httptest.NewRecorder()
	handler.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/.well-known/jwks.json", nil))
	require.Equal(t, http.StatusOK, get.Code)
	require.Contains(t, get.Header().Get("Content-Type"), "application/json")
	require.Contains(t, get.Body.String(), `"kid":"current"`)
	require.Contains(t, get.Body.String(), `"kid":"next"`)
	post := httptest.NewRecorder()
	handler.ServeHTTP(post, httptest.NewRequest(http.MethodPost, "/.well-known/jwks.json", nil))
	require.Equal(t, http.StatusMethodNotAllowed, post.Code)
	health := httptest.NewRecorder()
	handler.ServeHTTP(health, httptest.NewRequest(http.MethodGet, "/health", nil))
	require.Equal(t, http.StatusOK, health.Code)
}

func TestOwnershipRoleTLSConfig_VerifiesRealPeerChainAndHostname(t *testing.T) {
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	server.Config.ErrorLog = log.New(io.Discard, "", 0)
	server.StartTLS()
	defer server.Close()
	caFile := filepath.Join(t.TempDir(), "trusted-ca.pem")
	require.NoError(t, os.WriteFile(caFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw}), 0600))
	for _, tc := range []struct {
		name, ca, serverName string
		allowed              bool
	}{
		{"trusted expected peer", caFile, "", true},
		{"untrusted peer", "", "", false},
		{"wrong hostname", caFile, "different.invalid", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("ROLE_PRINCIPAL_TLS_CA_FILE", tc.ca)
			t.Setenv("ROLE_PRINCIPAL_TLS_SERVER_NAME", tc.serverName)
			config, err := ownershipRoleTLSFromEnv()
			require.NoError(t, err)
			transport := &http.Transport{TLSClientConfig: config}
			defer transport.CloseIdleConnections()
			client := &http.Client{Transport: transport, Timeout: 3 * time.Second}
			resp, err := client.Get(server.URL)
			if tc.allowed {
				require.NoError(t, err)
				defer resp.Body.Close()
				require.Equal(t, http.StatusNoContent, resp.StatusCode)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestSpacePrincipalDisabledRetainsHealth(t *testing.T) {
	response := httptest.NewRecorder()
	spaceHTTPHandler("space", principalJWKS{}).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/health", nil))
	require.Equal(t, http.StatusOK, response.Code)
}
