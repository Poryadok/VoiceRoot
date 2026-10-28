package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
	filev1 "voice.app/voice/file/v1"
	storyv1 "voice.app/voice/story/v1"
)

type storyMediaMTLSProbe struct {
	filev1.UnimplementedFileServiceServer
	calls atomic.Int32
}

func (p *storyMediaMTLSProbe) ValidateStoryMedia(context.Context, *filev1.ValidateStoryMediaRequest) (*filev1.ValidateStoryMediaResponse, error) {
	p.calls.Add(1)
	return &filev1.ValidateStoryMediaResponse{}, nil
}

func TestStoryMediaClientPresentsTrustedCertificateToProtectedFile(t *testing.T) {
	ca, caKey, caPEM := storyMediaTestCA(t)
	serverPair, _, _ := storyMediaTestLeaf(t, ca, caKey, x509.ExtKeyUsageServerAuth, []string{"file"})
	_, clientPEM, clientKeyPEM := storyMediaTestLeaf(t, ca, caKey, x509.ExtKeyUsageClientAuth, nil)
	foreignCA, foreignKey, _ := storyMediaTestCA(t)
	_, foreignPEM, foreignKeyPEM := storyMediaTestLeaf(t, foreignCA, foreignKey, x509.ExtKeyUsageClientAuth, nil)
	rootPool := x509.NewCertPool()
	require.True(t, rootPool.AppendCertsFromPEM(caPEM))
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	probe := &storyMediaMTLSProbe{}
	server := grpc.NewServer(grpc.Creds(credentials.NewTLS(&tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{serverPair},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    rootPool,
	})))
	filev1.RegisterFileServiceServer(server, probe)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)

	write := func(name string, data []byte) string {
		path := filepath.Join(t.TempDir(), name)
		require.NoError(t, os.WriteFile(path, data, 0600))
		return path
	}
	caPath := write("ca.pem", caPEM)
	clientCertPath := write("client.crt", clientPEM)
	clientKeyPath := write("client.key", clientKeyPEM)
	foreignCertPath := write("foreign.crt", foreignPEM)
	foreignKeyPath := write("foreign.key", foreignKeyPEM)
	signingDir := t.TempDir()
	for _, kid := range []string{"current", "next"} {
		key, keyErr := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, keyErr)
		writeStoryTestKey(t, filepath.Join(signingDir, kid+".pem"), key)
	}
	t.Setenv("STORY_PRINCIPAL_SIGNING_KEYS_DIR", signingDir)
	t.Setenv("STORY_PRINCIPAL_ACTIVE_KID", "current")
	t.Setenv("FILE_PRINCIPAL_GRPC_ADDR", listener.Addr().String())
	t.Setenv("FILE_PRINCIPAL_TLS_CA_FILE", caPath)
	t.Setenv("FILE_PRINCIPAL_TLS_SERVER_NAME", "file")
	t.Setenv("FILE_PRINCIPAL_CLIENT_CERT_FILE", clientCertPath)
	t.Setenv("FILE_PRINCIPAL_CLIENT_KEY_FILE", clientKeyPath)

	call := func(conn *grpc.ClientConn) error {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_, callErr := filev1.NewFileServiceClient(conn).ValidateStoryMedia(ctx, &filev1.ValidateStoryMediaRequest{})
		return callErr
	}
	t.Run("missing certificate cannot cross TLS before RPC", func(t *testing.T) {
		conn, dialErr := grpc.NewClient(listener.Addr().String(), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{
			MinVersion: tls.VersionTLS12, RootCAs: rootPool, ServerName: "file",
		})))
		require.NoError(t, dialErr)
		defer conn.Close()
		require.Equal(t, codes.Unavailable, status.Code(call(conn)))
		require.Zero(t, probe.calls.Load())
	})
	t.Run("configured certificate reaches protected RPC", func(t *testing.T) {
		client, conn, _, loadErr := loadSignedStoryMediaClientFromEnv()
		require.NoError(t, loadErr)
		defer conn.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		require.NoError(t, client.ValidateStoryMedia(ctx, uuid.New(), uuid.New(), storyv1.StoryMediaType_STORY_MEDIA_TYPE_PHOTO))
		require.EqualValues(t, 1, probe.calls.Load())
	})
	t.Run("missing configured pair fails before dial", func(t *testing.T) {
		t.Setenv("FILE_PRINCIPAL_CLIENT_CERT_FILE", "")
		t.Setenv("FILE_PRINCIPAL_CLIENT_KEY_FILE", "")
		_, conn, _, loadErr := loadSignedStoryMediaClientFromEnv()
		if conn != nil {
			_ = conn.Close()
		}
		require.Error(t, loadErr)
	})
	t.Run("untrusted client certificate cannot reach RPC", func(t *testing.T) {
		t.Setenv("FILE_PRINCIPAL_CLIENT_CERT_FILE", foreignCertPath)
		t.Setenv("FILE_PRINCIPAL_CLIENT_KEY_FILE", foreignKeyPath)
		_, conn, _, loadErr := loadSignedStoryMediaClientFromEnv()
		require.NoError(t, loadErr)
		defer conn.Close()
		require.Equal(t, codes.Unavailable, status.Code(call(conn)))
		require.EqualValues(t, 1, probe.calls.Load())
	})
	t.Run("mismatched certificate and key fail before dial", func(t *testing.T) {
		t.Setenv("FILE_PRINCIPAL_CLIENT_CERT_FILE", clientCertPath)
		t.Setenv("FILE_PRINCIPAL_CLIENT_KEY_FILE", foreignKeyPath)
		_, conn, _, loadErr := loadSignedStoryMediaClientFromEnv()
		if conn != nil {
			_ = conn.Close()
		}
		require.Error(t, loadErr)
	})
}

func storyMediaTestCA(t *testing.T) (*x509.Certificate, *ecdsa.PrivateKey, []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	template := &x509.Certificate{SerialNumber: big.NewInt(time.Now().UnixNano()), Subject: pkix.Name{CommonName: "test CA"},
		NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	return cert, key, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

func storyMediaTestLeaf(t *testing.T, ca *x509.Certificate, caKey *ecdsa.PrivateKey, use x509.ExtKeyUsage, dns []string) (tls.Certificate, []byte, []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	template := &x509.Certificate{SerialNumber: big.NewInt(time.Now().UnixNano()), DNSNames: dns,
		NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{use}, BasicConstraintsValid: true}
	der, err := x509.CreateCertificate(rand.Reader, template, ca, &key.PublicKey, caKey)
	require.NoError(t, err)
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	pair, err := tls.X509KeyPair(certPEM, keyPEM)
	require.NoError(t, err)
	return pair, certPEM, keyPEM
}
