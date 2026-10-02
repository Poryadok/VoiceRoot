package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"github.com/stretchr/testify/require"
	"io"
	"log"
	"math/big"
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
	t.Setenv("SPACE_ROLE_CLIENT_CERT_FILE", "")
	t.Setenv("SPACE_ROLE_CLIENT_KEY_FILE", "")
	config, err := ownershipRoleTLSFromEnv()
	require.Error(t, err, "private Role listener requires a Space client certificate")
	require.Nil(t, config)

	certFile, keyFile := writeClientCertificate(t)
	t.Setenv("SPACE_ROLE_CLIENT_CERT_FILE", certFile)
	t.Setenv("SPACE_ROLE_CLIENT_KEY_FILE", keyFile)
	config, err = ownershipRoleTLSFromEnv()
	require.NoError(t, err)
	require.False(t, config.InsecureSkipVerify)
	require.Equal(t, "role.internal", config.ServerName)
	require.GreaterOrEqual(t, config.MinVersion, uint16(tls.VersionTLS12))
	require.Len(t, config.Certificates, 1)
	t.Setenv("SPACE_ROLE_CLIENT_KEY_FILE", keyFile+".missing")
	_, err = ownershipRoleTLSFromEnv()
	require.Error(t, err)
	t.Setenv("ROLE_PRINCIPAL_TLS_CA_FILE", t.TempDir()+"/absent.pem")
	_, err = ownershipRoleTLSFromEnv()
	require.Error(t, err)
	invalid := filepath.Join(t.TempDir(), "invalid.pem")
	require.NoError(t, os.WriteFile(invalid, []byte("not a CA"), 0600))
	t.Setenv("ROLE_PRINCIPAL_TLS_CA_FILE", invalid)
	_, err = ownershipRoleTLSFromEnv()
	require.Error(t, err)
}

func TestOwnershipAuthTLSConfigFailsClosed(t *testing.T) {
	t.Setenv("AUTH_PRINCIPAL_TLS_CA_FILE", "")
	t.Setenv("AUTH_PRINCIPAL_TLS_SERVER_NAME", "auth.internal")
	config, err := ownershipAuthTLSFromEnv()
	require.NoError(t, err)
	require.False(t, config.InsecureSkipVerify)
	require.Equal(t, "auth.internal", config.ServerName)
	t.Setenv("AUTH_PRINCIPAL_TLS_CA_FILE", t.TempDir()+"/absent.pem")
	_, err = ownershipAuthTLSFromEnv()
	require.Error(t, err)
}

func TestSpacePrincipalJWKSIsAbsentFromPlainHTTPAndReadOnlyOnTLSHandler(t *testing.T) {
	disabled := httptest.NewRecorder()
	spaceHTTPHandler("space", principalJWKS{}).ServeHTTP(disabled, httptest.NewRequest(http.MethodGet, "/.well-known/jwks.json", nil))
	require.Equal(t, http.StatusNotFound, disabled.Code)
	keys := principalJWKS{Keys: []principalJWK{{Kty: "RSA", Kid: "current", Use: "sig", Alg: "RS256", N: "public-n", E: "AQAB"}, {Kty: "RSA", Kid: "next", Use: "sig", Alg: "RS256", N: "other-public-n", E: "AQAB"}}}
	plain := httptest.NewRecorder()
	spaceHTTPHandler("space", principalJWKS{}).ServeHTTP(plain, httptest.NewRequest(http.MethodGet, "/.well-known/jwks.json", nil))
	require.Equal(t, http.StatusNotFound, plain.Code, "signing keys are only served over the dedicated TLS endpoint")
	handler := spaceJWKSHandler(keys)
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
	server.TLS = &tls.Config{MinVersion: tls.VersionTLS12, ClientAuth: tls.RequireAnyClientCert}
	server.StartTLS()
	defer server.Close()
	caFile := filepath.Join(t.TempDir(), "trusted-ca.pem")
	require.NoError(t, os.WriteFile(caFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw}), 0600))
	clientCertFile, clientKeyFile := writeClientCertificate(t)
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
			t.Setenv("SPACE_ROLE_CLIENT_CERT_FILE", clientCertFile)
			t.Setenv("SPACE_ROLE_CLIENT_KEY_FILE", clientKeyFile)
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

func writeClientCertificate(t *testing.T) (string, string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 120))
	require.NoError(t, err)
	template := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "space"},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)
	privateDER, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	certFile := filepath.Join(t.TempDir(), "space-client.crt")
	keyFile := filepath.Join(t.TempDir(), "space-client.key")
	require.NoError(t, os.WriteFile(certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600))
	require.NoError(t, os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateDER}), 0600))
	return certFile, keyFile
}

func TestSpacePrincipalDisabledRetainsHealth(t *testing.T) {
	response := httptest.NewRecorder()
	spaceHTTPHandler("space", principalJWKS{}).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/health", nil))
	require.Equal(t, http.StatusOK, response.Code)
}
