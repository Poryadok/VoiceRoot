package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestMessagingPrivateServerBindsAndShutsDownCleanly(t *testing.T) {
	certFile, keyFile, caFile := writeMessagingTLSFiles(t)
	server, listener, err := newMessagingPrivateServer(config{
		MessagingListenAddr: "127.0.0.1:0", MessagingTLSCertFile: certFile,
		MessagingTLSKeyFile: keyFile, MessagingClientCAFile: caFile,
	}, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	require.NoError(t, err)
	require.NotNil(t, listener)
	serveResult := make(chan error, 1)
	go func() { serveResult <- server.Serve(listener) }()
	connection, err := net.DialTimeout("tcp", listener.Addr().String(), time.Second)
	require.NoError(t, err, "the configured private address is bound")
	require.NoError(t, connection.Close())

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, server.Shutdown(ctx))
	require.ErrorIs(t, <-serveResult, http.ErrServerClosed)
}

func writeMessagingTLSFiles(t *testing.T) (string, string, string) {
	t.Helper()
	dir := t.TempDir()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	now := time.Now()
	caTemplate := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Messaging test CA"},
		NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour), IsCA: true, BasicConstraintsValid: true,
		KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &key.PublicKey, key)
	require.NoError(t, err)
	serverKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	serverTemplate := &x509.Certificate{SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: "GIS test server"},
		NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, DNSNames: []string{"gis.internal"}}
	serverDER, err := x509.CreateCertificate(rand.Reader, serverTemplate, caTemplate, &serverKey.PublicKey, key)
	require.NoError(t, err)
	caPath := filepath.Join(dir, "messaging-ca.pem")
	certPath, keyPath := filepath.Join(dir, "gis.pem"), filepath.Join(dir, "gis-key.pem")
	require.NoError(t, os.WriteFile(caPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER}), 0o600))
	require.NoError(t, os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: serverDER}), 0o600))
	keyDER, err := x509.MarshalPKCS8PrivateKey(serverKey)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}), 0o600))
	return certPath, keyPath, caPath
}
