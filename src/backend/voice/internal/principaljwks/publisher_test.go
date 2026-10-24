package principaljwks

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"voice/backend/pkg/principal"
)

func publisherConfig(t *testing.T) (Config, *x509.CertPool) {
	t.Helper()
	current, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	next, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	leaf := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "voice.test"},
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, NotBefore: time.Now().Add(-time.Minute),
		NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true,
		KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, leaf, leaf, &current.PublicKey, current)
	require.NoError(t, err)
	cert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	dir := t.TempDir()
	certPath, keyPath := filepath.Join(dir, "tls.crt"), filepath.Join(dir, "tls.key")
	require.NoError(t, os.WriteFile(certPath, cert, 0600))
	require.NoError(t, os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(current)}), 0600))
	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM(cert))
	return Config{ListenAddr: "127.0.0.1:0", KeyID: "current", SigningKey: current, NextKeyID: "next", NextSigningKey: next, TLSCert: certPath, TLSKey: keyPath}, roots
}

func TestPublisherServesRealKeysImmediatelyAndCloses(t *testing.T) {
	cfg, roots := publisherConfig(t)
	publisher, err := Start(cfg, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = publisher.Close() })
	transport := &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots}}
	t.Cleanup(transport.CloseIdleConnections)
	client := &http.Client{Transport: transport, Timeout: time.Second}
	// There is no retry/sleep or readiness placeholder between Start and fetch.
	response, err := client.Get("https://" + publisher.Addr().String() + "/internal/v1/principal/jwks.json")
	require.NoError(t, err)
	document, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
	require.Equal(t, http.StatusOK, response.StatusCode)
	keys, err := principal.ParseJWKS(document)
	require.NoError(t, err)
	require.Len(t, keys, 2)
	require.True(t, cfg.SigningKey.PublicKey.Equal(keys[cfg.KeyID]))
	require.True(t, cfg.NextSigningKey.PublicKey.Equal(keys[cfg.NextKeyID]))
	response, err = client.Get("https://" + publisher.Addr().String() + "/ready")
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, response.StatusCode, "public publisher must not admit readiness/mutations")
	require.NoError(t, response.Body.Close())
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	require.NoError(t, publisher.Shutdown(ctx))
	require.ErrorIs(t, <-publisher.Errors(), http.ErrServerClosed)
}

func TestPublisherTLSAndBindErrorsAreSynchronous(t *testing.T) {
	cfg, _ := publisherConfig(t)
	bad := cfg
	bad.TLSKey = filepath.Join(t.TempDir(), "absent.key")
	_, err := Start(bad, nil)
	require.Error(t, err)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = listener.Close() }()
	cfg.ListenAddr = listener.Addr().String()
	_, err = Start(cfg, nil)
	require.Error(t, err, "caller cannot enter recovery after failed publisher bind")
}

func TestPublisherReportsUnexpectedServeFailure(t *testing.T) {
	cfg, _ := publisherConfig(t)
	publisher, err := Start(cfg, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = publisher.Close() })
	require.NoError(t, publisher.listener.Close())
	select {
	case err := <-publisher.Errors():
		require.Error(t, err)
		require.NotErrorIs(t, err, http.ErrServerClosed)
	case <-time.After(time.Second):
		t.Fatal("publisher must report unexpected listener failure to startup owner")
	}
}
