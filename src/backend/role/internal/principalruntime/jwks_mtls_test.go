package principalruntime

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	rolev1 "voice.app/voice/role/v1"
)

func TestRuntimeJWKSUsesConfiguredClientIdentity(t *testing.T) {
	f := newRuntimeFixture(t, 2)
	caPEM, err := os.ReadFile(f.config.ClientCAFile)
	require.NoError(t, err)
	clientCAs := x509.NewCertPool()
	require.True(t, clientCAs.AppendCertsFromPEM(caPEM))
	serverCert, err := tls.LoadX509KeyPair(f.config.TLSCertFile, f.config.TLSKeyFile)
	require.NoError(t, err)
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NotEmpty(t, r.TLS.VerifiedChains)
		_, _ = w.Write(f.jwks.response.Load().body)
	}))
	server.TLS = &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{serverCert}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: clientCAs}
	server.StartTLS()
	t.Cleanup(server.Close)
	keyDER, err := x509.MarshalPKCS8PrivateKey(f.clientCertificate.PrivateKey)
	require.NoError(t, err)
	keyPath := filepath.Join(t.TempDir(), "client.key")
	require.NoError(t, os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}), 0600))
	validRuntimeEnv(t)
	urls, err := json.Marshal(map[string]string{"space": server.URL, "gameintegration": server.URL, "voice": server.URL})
	require.NoError(t, err)
	t.Setenv("S2S_JWKS_URLS_JSON", string(urls))
	t.Setenv("S2S_JWKS_CA_FILE", f.config.JWKSCAFile)
	t.Setenv("S2S_JWKS_CLIENT_CERT_FILE", f.config.ClientCAFile)
	t.Setenv("S2S_JWKS_CLIENT_KEY_FILE", keyPath)
	t.Setenv("ROLE_PRINCIPAL_TLS_CERT_FILE", f.config.TLSCertFile)
	t.Setenv("ROLE_PRINCIPAL_TLS_KEY_FILE", f.config.TLSKeyFile)
	t.Setenv("ROLE_PRINCIPAL_CLIENT_CA_FILE", f.config.ClientCAFile)
	t.Setenv("ROLE_PRINCIPAL_REPLAY_REDIS_ADDR", f.config.ReplayAddr)
	cfg, enabled, err := LoadFromEnv()
	require.NoError(t, err)
	require.True(t, enabled)
	runtime, err := New(context.Background(), cfg)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, runtime.Close()) })
	method := rolev1.RoleService_CheckPermission_FullMethodName
	token := issueRuntimeToken(t, f.key, "space", "current", "role", method, "mtls-request", runtimeHash)
	_, err = runtime.Verify(context.Background(), token, method, "mtls-request", runtimeHash)
	require.NoError(t, err)
}

func TestLoadFromEnvRejectsPartialJWKSClientIdentity(t *testing.T) {
	for _, name := range []string{"S2S_JWKS_CLIENT_CERT_FILE", "S2S_JWKS_CLIENT_KEY_FILE"} {
		t.Run(name, func(t *testing.T) {
			validRuntimeEnv(t)
			t.Setenv(name, "configured.pem")
			_, _, err := LoadFromEnv()
			require.Error(t, err)
		})
	}
}
