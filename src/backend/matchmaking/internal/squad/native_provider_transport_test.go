package squad

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
	"voice/backend/matchmaking/internal/store"
)

func TestRequiredProviderRuntimePreparesBothActualTLSConnectionsWithoutCreate(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	cert := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "owned-provider-fixture"}, DNSNames: []string{"provider-fixture"}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}}
	der, err := x509.CreateCertificate(rand.Reader, cert, cert, &key.PublicKey, key)
	require.NoError(t, err)
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	dir := t.TempDir()
	certPath, keyPath := filepath.Join(dir, "tls.crt"), filepath.Join(dir, "tls.key")
	require.NoError(t, os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600))
	require.NoError(t, os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}), 0600))
	identity, err := tls.LoadX509KeyPair(certPath, keyPath)
	require.NoError(t, err)
	roots := x509.NewCertPool()
	parsed, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	roots.AddCert(parsed)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	var domainCalls atomic.Int32
	server := grpc.NewServer(grpc.Creds(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{identity}, ClientCAs: roots, ClientAuth: tls.RequireAndVerifyClientCert})), grpc.UnknownServiceHandler(func(_ any, _ grpc.ServerStream) error { domainCalls.Add(1); return nil }))
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { server.Stop(); _ = listener.Close() })
	signing := t.TempDir()
	writePrincipalRotationKeys(t, signing)
	values := map[string]string{
		"MATCHMAKING_MATCH_SQUAD_CHAT_GRPC_ADDR": listener.Addr().String(), "MATCHMAKING_MATCH_SQUAD_VOICE_GRPC_ADDR": listener.Addr().String(),
		"MATCHMAKING_MATCH_SQUAD_CHAT_CA_FILE": certPath, "MATCHMAKING_MATCH_SQUAD_VOICE_CA_FILE": certPath, "MATCHMAKING_MATCH_SQUAD_CLIENT_CERT_FILE": certPath, "MATCHMAKING_MATCH_SQUAD_CLIENT_KEY_FILE": keyPath,
		"MATCHMAKING_MATCH_SQUAD_CHAT_SERVER_NAME": "provider-fixture", "MATCHMAKING_MATCH_SQUAD_VOICE_SERVER_NAME": "provider-fixture",
		"MATCHMAKING_PRINCIPAL_SIGNING_KEYS_DIR": signing, "MATCHMAKING_PRINCIPAL_ACTIVE_KID": "current", "MATCHMAKING_PRINCIPAL_JWKS_LISTEN_ADDR": ":8443", "MATCHMAKING_PRINCIPAL_JWKS_TLS_CERT_FILE": certPath, "MATCHMAKING_PRINCIPAL_JWKS_TLS_KEY_FILE": keyPath}
	runtime, closeRuntime, err := LoadRequiredProtectedProviderRuntime(&store.MatchStore{}, func(k string) string { return values[k] })
	require.NoError(t, err)
	defer closeRuntime()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, runtime.CheckTransports(ctx))
	require.Zero(t, domainCalls.Load())
	values["MATCHMAKING_MATCH_SQUAD_VOICE_SERVER_NAME"] = "wrong-provider"
	wrong, closeWrong, err := LoadRequiredProtectedProviderRuntime(&store.MatchStore{}, func(k string) string { return values[k] })
	require.NoError(t, err)
	defer closeWrong()
	bounded, done := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer done()
	require.Error(t, wrong.CheckTransports(bounded))
	require.Zero(t, domainCalls.Load())
	empty := func(string) string { return "" }
	_, _, err = LoadRequiredProtectedProviderRuntime(&store.MatchStore{}, empty)
	require.Error(t, err)
	old, closeOld, configured, err := LoadProtectedProviderWorker(&store.MatchStore{}, empty)
	require.NoError(t, err)
	require.Nil(t, old)
	require.False(t, configured)
	require.NoError(t, closeOld())
}
