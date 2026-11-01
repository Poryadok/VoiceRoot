package identityassociation

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	identityv1 "voice.app/voice/identityerasure/v1"
	"voice/backend/pkg/principal"
)

func TestAuthParentRuntimeConfigAbsentOrComplete(t *testing.T) {
	values := map[string]string{}
	lookup := func(name string) (string, bool) { value, ok := values[name]; return value, ok }
	_, enabled, err := LoadAuthParentRuntimeConfig(lookup)
	require.NoError(t, err)
	require.False(t, enabled)
	values["USER_IDENTITY_AUTH_GRPC_ADDR"] = "auth.example:9099"
	_, _, err = LoadAuthParentRuntimeConfig(lookup)
	require.Error(t, err)
	values["USER_IDENTITY_AUTH_TLS_SERVER_NAME"] = "auth.example"
	values["USER_IDENTITY_AUTH_TLS_CA_FILE"] = "ca.pem"
	values["USER_IDENTITY_AUTH_TLS_CERT_FILE"] = "client.pem"
	values["USER_IDENTITY_AUTH_TLS_KEY_FILE"] = "client.key"
	_, enabled, err = LoadAuthParentRuntimeConfig(lookup)
	require.NoError(t, err)
	require.True(t, enabled)
	for key, value := range values {
		values[key] = ""
		_, _, err = LoadAuthParentRuntimeConfig(lookup)
		require.Error(t, err, key)
		values[key] = " " + value
		_, _, err = LoadAuthParentRuntimeConfig(lookup)
		require.Error(t, err, key)
		values[key] = value
	}
}

func TestOpenAuthParentRuntimeRequiresVerifiedReadyPeer(t *testing.T) {
	pki := makeParentTestPKI(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	server := grpc.NewServer(grpc.Creds(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{pki.server}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: pki.roots})))
	identityv1.RegisterAuthUserIdentityAuthorityServiceServer(server, &parentContractServer{})
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { server.Stop(); _ = listener.Close() })
	dir := t.TempDir()
	caFile, certFile, keyFile := filepath.Join(dir, "ca.pem"), filepath.Join(dir, "client.pem"), filepath.Join(dir, "client.key")
	require.NoError(t, os.WriteFile(caFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: pki.caDER}), 0600))
	require.NoError(t, os.WriteFile(certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: pki.client.Certificate[0]}), 0600))
	keyDER, err := x509.MarshalPKCS8PrivateKey(pki.client.PrivateKey)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}), 0600))
	signing, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "user", KeyID: "fixture", PrivateKey: signing})
	require.NoError(t, err)
	config := AuthParentRuntimeConfig{Address: listener.Addr().String(), ServerName: "auth.parent.test", CAFile: caFile, CertFile: certFile, KeyFile: keyFile}
	runtime, err := OpenAuthParentRuntime(context.Background(), config, issuer)
	require.NoError(t, err)
	defer runtime.Close()
	_, err = runtime.Client.ObserveParent(context.Background(), uuid.New())
	require.NoError(t, err)
	config.ServerName = "wrong.parent.test"
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	_, err = OpenAuthParentRuntime(ctx, config, issuer)
	require.Error(t, err)
}
