package sourcecustody

import (
	"context"
	"crypto/tls"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestUserJWKSConfigIsAbsentOrComplete(t *testing.T) {
	values := map[string]string{}
	lookup := func(name string) (string, bool) { value, ok := values[name]; return value, ok }
	_, enabled, err := LoadUserJWKSConfig(lookup)
	require.NoError(t, err)
	require.False(t, enabled)
	values["USER_PRINCIPAL_JWKS_LISTEN"] = "127.0.0.1:9443"
	_, _, err = LoadUserJWKSConfig(lookup)
	require.Error(t, err)
	values["USER_PRINCIPAL_JWKS_TLS_CERT_FILE"] = "cert.pem"
	values["USER_PRINCIPAL_JWKS_TLS_KEY_FILE"] = "key.pem"
	_, enabled, err = LoadUserJWKSConfig(lookup)
	require.NoError(t, err)
	require.True(t, enabled)
	for key, value := range values {
		values[key] = ""
		_, _, err = LoadUserJWKSConfig(lookup)
		require.Error(t, err, key)
		values[key] = " " + value
		_, _, err = LoadUserJWKSConfig(lookup)
		require.Error(t, err, key)
		values[key] = value
	}
	values["USER_PRINCIPAL_JWKS_LISTEN"] = "127.0.0.1:0"
	_, _, err = LoadUserJWKSConfig(lookup)
	require.Error(t, err)
}

func TestUserJWKSRuntimeServesTLSAndStopsOnCancellation(t *testing.T) {
	dir := t.TempDir()
	_, current := testPKCS8Key(t, 2048)
	_, next := testPKCS8Key(t, 2048)
	writeCustodyKey(t, dir, "current.pem", current)
	writeCustodyKey(t, dir, "next.pem", next)
	_, public, err := LoadUserSigner(dir, "current")
	require.NoError(t, err)
	pki := processTLS(t, t.TempDir(), "user-jwks")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, err = StartUserJWKS(ctx, "127.0.0.1:0", pki.serverCert, pki.serverKey, public)
	require.Error(t, err, "production listener must have an explicit fixed port")
	runtime, err := startUserJWKS(ctx, "127.0.0.1:0", pki.serverCert, pki.serverKey, public, true)
	require.NoError(t, err)
	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: pki.roots, ServerName: "localhost"}}, Timeout: 2 * time.Second}
	response, err := client.Get("https://" + runtime.Address() + "/.well-known/jwks.json")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode)
	require.NoError(t, response.Body.Close())
	cancel()
	select {
	case <-runtime.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("JWKS listener did not stop after cancellation")
	}
	closeCtx, stop := context.WithTimeout(context.Background(), 2*time.Second)
	defer stop()
	require.NoError(t, runtime.Close(closeCtx))
}
