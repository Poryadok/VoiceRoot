package botmembershipprincipal

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/require"
	"voice/backend/pkg/principal"
)

// This component fixture supplies real mTLS and Redis but no installation,
// membership, account birth, or production provider qualification.
func membershipRuntimeFixture(t *testing.T) (Config, *httptest.Server, *atomic.Int64) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	ca := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "component CA"}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	der, err := x509.CreateCertificate(rand.Reader, ca, ca, &key.PublicKey, key)
	require.NoError(t, err)
	ca, err = x509.ParseCertificate(der)
	require.NoError(t, err)
	leaf := &x509.Certificate{SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: "component"}, DNSNames: []string{"localhost"}, IPAddresses: nil, NotBefore: ca.NotBefore, NotAfter: ca.NotAfter, KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}}
	// httptest connects to its loopback IP; include that identity explicitly.
	leaf.IPAddresses = append(leaf.IPAddresses, []byte{127, 0, 0, 1})
	leafDER, err := x509.CreateCertificate(rand.Reader, leaf, ca, &key.PublicKey, key)
	require.NoError(t, err)
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leafDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	identity, err := tls.X509KeyPair(certPEM, keyPEM)
	require.NoError(t, err)
	pool := x509.NewCertPool()
	pool.AddCert(ca)
	current, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	next, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	var calls atomic.Int64
	mux := http.NewServeMux()
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		principal.JWKSHandlerKeys([]principal.JWKSKey{{KeyID: "current", PublicKey: &current.PublicKey}, {KeyID: "next", PublicKey: &next.PublicKey}}).ServeHTTP(w, r)
	})
	mux.Handle("/one", principal.JWKSHandlerKeys([]principal.JWKSKey{{KeyID: "current", PublicKey: &current.PublicKey}}))
	mux.HandleFunc("/redirect", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/jwks", http.StatusFound) })
	server := httptest.NewUnstartedServer(mux)
	server.TLS = &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{identity}, ClientCAs: pool, ClientAuth: tls.RequireAndVerifyClientCert}
	server.StartTLS()
	t.Cleanup(server.Close)
	dir := t.TempDir()
	cert, keyFile, caFile := filepath.Join(dir, "leaf.crt"), filepath.Join(dir, "leaf.key"), filepath.Join(dir, "ca.crt")
	require.NoError(t, os.WriteFile(cert, certPEM, 0600))
	require.NoError(t, os.WriteFile(keyFile, keyPEM, 0600))
	require.NoError(t, os.WriteFile(caFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600))
	redis := miniredis.RunT(t)
	return Config{ListenAddr: "127.0.0.1:0", CertFile: cert, KeyFile: keyFile, ClientCAFile: caFile, ReplayAddr: redis.Addr(), JWKSURLs: map[string]string{"bot": server.URL + "/jwks"}, JWKSCAFile: caFile, JWKSClientCertFile: cert, JWKSClientKeyFile: keyFile, RefreshAfter: time.Second, HardExpiry: time.Minute, UnknownKIDCooldown: time.Second}, server, &calls
}

func TestMembershipRuntimeRequiresReadyTwoKeyJWKSBeforeReturning(t *testing.T) {
	c, _, calls := membershipRuntimeFixture(t)
	runtime, err := New(context.Background(), c)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, runtime.Close()) })
	require.Positive(t, calls.Load(), "constructor must fetch trusted complete JWKS before the listener can be published")
	require.NoError(t, runtime.Close())
	require.NoError(t, runtime.Close())
}

func TestMembershipRuntimeRefusesUnreadyProtectedJWKS(t *testing.T) {
	for _, name := range []string{"one key", "redirect", "missing client certificate", "untrusted CA", "unavailable"} {
		t.Run(name, func(t *testing.T) {
			c, server, _ := membershipRuntimeFixture(t)
			switch name {
			case "one key":
				c.JWKSURLs["bot"] = server.URL + "/one"
			case "redirect":
				c.JWKSURLs["bot"] = server.URL + "/redirect"
			case "missing client certificate":
				c.JWKSClientCertFile = ""
				c.JWKSClientKeyFile = ""
			case "untrusted CA":
				c.JWKSCAFile = ""
			case "unavailable":
				server.Close()
			}
			runtime, err := New(context.Background(), c)
			if runtime != nil {
				t.Cleanup(func() { _ = runtime.Close() })
			}
			require.Error(t, err, "startup must not publish a listener with an unqualified JWKS dependency")
			require.Nil(t, runtime)
		})
	}
}
