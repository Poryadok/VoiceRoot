package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
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
)

func TestStoryJWKSHTTPSConfigurationAndServing(t *testing.T) {
	const addr = "STORY_PRINCIPAL_JWKS_HTTPS_LISTEN"
	const cert = "STORY_PRINCIPAL_JWKS_TLS_CERT_FILE"
	const key = "STORY_PRINCIPAL_JWKS_TLS_KEY_FILE"
	doc := []byte(`{"keys":[]}`)
	lookup := func(values map[string]string) func(string) (string, bool) {
		return func(name string) (string, bool) { value, ok := values[name]; return value, ok }
	}
	server, enabled, err := newStoryJWKSHTTPSServer(lookup(nil), doc)
	require.NoError(t, err)
	require.False(t, enabled)
	require.Nil(t, server)
	for _, values := range []map[string]string{
		{addr: "127.0.0.1:0"}, {addr: "127.0.0.1:0", cert: "missing"},
		{addr: "", cert: "missing", key: "missing"},
		{cert: "missing"}, {key: "missing"},
		{addr: "127.0.0.1:0", cert: "", key: "missing"},
		{addr: "127.0.0.1:0", cert: "missing", key: ""},
		{addr: "127.0.0.1:0", cert: "missing", key: "missing"},
	} {
		_, enabled, err = newStoryJWKSHTTPSServer(lookup(values), doc)
		require.True(t, enabled)
		require.Error(t, err)
	}
	ca, caKey, caPEM := storyMediaTestCA(t)
	_, certPEM, keyPEM := storyMediaTestLeaf(t, ca, caKey, x509.ExtKeyUsageServerAuth, []string{"story"})
	dir := t.TempDir()
	certPath, keyPath := filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem")
	require.NoError(t, os.WriteFile(certPath, certPEM, 0600))
	require.NoError(t, os.WriteFile(keyPath, keyPEM, 0600))
	values := map[string]string{addr: "127.0.0.1:0", cert: certPath, key: keyPath}
	_, _, otherKey := storyMediaTestLeaf(t, ca, caKey, x509.ExtKeyUsageServerAuth, []string{"story"})
	otherKeyPath := filepath.Join(dir, "other-key.pem")
	require.NoError(t, os.WriteFile(otherKeyPath, otherKey, 0600))
	_, _, err = newStoryJWKSHTTPSServer(lookup(map[string]string{addr: "127.0.0.1:0", cert: certPath, key: otherKeyPath}), doc)
	require.Error(t, err)
	for _, window := range [][2]time.Time{
		{time.Now().Add(-2 * time.Hour), time.Now().Add(-time.Hour)},
		{time.Now().Add(time.Hour), time.Now().Add(2 * time.Hour)},
	} {
		badCert, badKey := storyJWKSLeaf(t, ca, caKey, window[0], window[1], x509.ExtKeyUsageServerAuth)
		require.NoError(t, os.WriteFile(certPath, badCert, 0600))
		require.NoError(t, os.WriteFile(keyPath, badKey, 0600))
		_, _, err = newStoryJWKSHTTPSServer(lookup(values), doc)
		require.Error(t, err)
	}
	clientCert, clientKey := storyJWKSLeaf(t, ca, caKey, time.Now().Add(-time.Minute), time.Now().Add(time.Hour), x509.ExtKeyUsageClientAuth)
	require.NoError(t, os.WriteFile(certPath, clientCert, 0600))
	require.NoError(t, os.WriteFile(keyPath, clientKey, 0600))
	_, _, err = newStoryJWKSHTTPSServer(lookup(values), doc)
	require.Error(t, err)
	require.NoError(t, os.WriteFile(certPath, certPEM, 0600))
	require.NoError(t, os.WriteFile(keyPath, keyPEM, 0600))
	_, _, err = newStoryJWKSHTTPSServer(lookup(values), nil)
	require.Error(t, err)
	server, enabled, err = newStoryJWKSHTTPSServer(lookup(values), doc)
	require.NoError(t, err)
	require.True(t, enabled)
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = occupied.Close() })
	server.Addr = occupied.Addr().String()
	_, err = listenStoryJWKSHTTPS(server)
	require.Error(t, err)
	require.NoError(t, occupied.Close())
	server.Addr = "127.0.0.1:0"
	listener, err := listenStoryJWKSHTTPS(server)
	require.NoError(t, err)
	done := make(chan error, 1)
	go func() { done <- server.Serve(tls.NewListener(listener, server.TLSConfig)) }()
	t.Cleanup(func() { _ = server.Close() })
	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM(caPEM))
	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: roots, ServerName: "story"}}, Timeout: 3 * time.Second}
	response, err := client.Get("https://" + listener.Addr().String() + "/.well-known/jwks.json")
	require.NoError(t, err)
	data, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
	require.Equal(t, http.StatusOK, response.StatusCode)
	require.Equal(t, doc, data)
	response, err = client.Get("https://" + listener.Addr().String() + "/other")
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, response.StatusCode)
	require.NoError(t, response.Body.Close())
	response, err = client.Post("https://"+listener.Addr().String()+"/.well-known/jwks.json", "application/json", nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusMethodNotAllowed, response.StatusCode)
	require.NoError(t, response.Body.Close())
	plainClient := &http.Client{Timeout: 3 * time.Second}
	plain, err := plainClient.Get("http://" + listener.Addr().String() + "/.well-known/jwks.json")
	if err == nil {
		require.NotEqual(t, http.StatusOK, plain.StatusCode)
		require.NoError(t, plain.Body.Close())
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, server.Shutdown(ctx))
	require.ErrorIs(t, <-done, http.ErrServerClosed)
}

func storyJWKSLeaf(t *testing.T, ca *x509.Certificate, caKey *ecdsa.PrivateKey, before, after time.Time, usage x509.ExtKeyUsage) ([]byte, []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	template := &x509.Certificate{SerialNumber: big.NewInt(time.Now().UnixNano()), DNSNames: []string{"story"},
		NotBefore: before, NotAfter: after, KeyUsage: x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{usage}, BasicConstraintsValid: true}
	der, err := x509.CreateCertificate(rand.Reader, template, ca, &key.PublicKey, caKey)
	require.NoError(t, err)
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
}
