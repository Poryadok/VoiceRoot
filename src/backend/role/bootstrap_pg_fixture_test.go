package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"github.com/stretchr/testify/require"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
	"voice/backend/role/internal/principalruntime"
)

// Test-only PKI mirrors the existing main fixture, but uses actual owned Redis.
func newBootstrapPGTransport(t *testing.T, replayAddr string) (*principalruntime.Runtime, *x509.CertPool, *rsa.PrivateKey, tls.Certificate) {
	t.Helper()
	current, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	next, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	jwk := func(kid string, key *rsa.PrivateKey) map[string]string {
		return map[string]string{"kid": kid, "kty": "RSA", "use": "sig", "alg": "RS256", "n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()), "e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes())}
	}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{jwk("current", current), jwk("next", next)}})
	}))
	t.Cleanup(server.Close)
	dir := t.TempDir()
	certFile, keyFile := filepath.Join(dir, "tls.crt"), filepath.Join(dir, "tls.key")
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.TLS.Certificates[0].Certificate[0]})
	require.NoError(t, os.WriteFile(certFile, certPEM, 0o600))
	keyDER, err := x509.MarshalPKCS8PrivateKey(server.TLS.Certificates[0].PrivateKey)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}), 0o600))
	clientKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	clientTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(9911), NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour),
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment | x509.KeyUsageCertSign,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}, BasicConstraintsValid: true, IsCA: true,
	}
	clientDER, err := x509.CreateCertificate(rand.Reader, clientTemplate, clientTemplate, &clientKey.PublicKey, clientKey)
	require.NoError(t, err)
	clientCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: clientDER})
	clientKeyDER, err := x509.MarshalPKCS8PrivateKey(clientKey)
	require.NoError(t, err)
	clientCAFile := filepath.Join(dir, "role-client-ca.pem")
	require.NoError(t, os.WriteFile(clientCAFile, clientCertPEM, 0o600))
	clientCertificate, err := tls.X509KeyPair(clientCertPEM, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: clientKeyDER}))
	require.NoError(t, err)
	sourceEnabled := false
	urls := map[string]string{"space": server.URL, "gameintegration": server.URL, "voice": server.URL}
	if sourceEnabled {
		urls["federation"] = server.URL
	}
	runtime, err := principalruntime.New(context.Background(), principalruntime.Config{
		JWKSURLs: urls, RefreshAfter: time.Minute, HardExpiry: 2 * time.Minute, AuthoritySourceEnabled: sourceEnabled,
		UnknownKIDCooldown: time.Second, ReplayAddr: replayAddr, JWKSCAFile: certFile,
		TLSCertFile: certFile, TLSKeyFile: keyFile, ClientCAFile: clientCAFile, ListenAddr: "127.0.0.1:0",
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, runtime.Close()) })
	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM(certPEM))
	return runtime, roots, current, clientCertificate
}
