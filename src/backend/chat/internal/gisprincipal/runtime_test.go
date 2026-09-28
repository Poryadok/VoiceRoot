package gisprincipal

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/pkg/principal"
)

func TestConfigFromEnvRejectsPartialPrincipalConfig(t *testing.T) {
	t.Setenv("CHAT_GIS_TLS_CERT_FILE", "cert.pem")
	t.Setenv("CHAT_GIS_TLS_KEY_FILE", "")
	t.Setenv("CHAT_GIS_CLIENT_CA_FILE", "ca.pem")
	t.Setenv("GAME_INTEGRATION_PRINCIPAL_JWKS_URL", "https://gis.test/jwks")
	t.Setenv("GAME_INTEGRATION_PRINCIPAL_REPLAY_REDIS_ADDR", "redis:6379")
	_, configured, err := ConfigFromEnv()
	require.True(t, configured)
	require.Error(t, err)
}

func TestStrictInterceptorBindsExactGISRequestAndRejectsReplay(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: Issuer, KeyID: "gis-key", PrivateKey: key})
	require.NoError(t, err)
	request := &chatv1.ProvisionManagedChatRequest{ApplicationId: "a", EnvironmentId: "e", OperationId: "00000000-0000-4000-8000-000000000001", ExternalChatKey: "party-1", Name: "Party"}
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	requestID := request.GetOperationId()
	token, err := issuer.IssueService(principal.ServiceInput{Audience: Audience, RPC: ProvisionMethod, RequestID: requestID, RequestHash: hash})
	require.NoError(t, err)
	used := false
	runtime := &Runtime{
		keyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil },
		replayGuard: func(context.Context, string, string, time.Time) error {
			if used {
				return errors.New("replay")
			}
			used = true
			return nil
		},
	}
	interceptor := strictInterceptor{runtime: runtime}.intercept
	info := &grpc.UnaryServerInfo{FullMethod: ProvisionMethod}
	calls := 0
	handler := func(context.Context, any) (any, error) { calls++; return "ok", nil }
	call := func(req *chatv1.ProvisionManagedChatRequest, id, bearer string) (any, error) {
		ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+bearer, "x-request-id", id))
		return interceptor(ctx, req, info, handler)
	}
	_, err = call(request, requestID, token)
	require.NoError(t, err)
	require.Equal(t, 1, calls)
	_, err = call(request, requestID, token)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	require.Equal(t, 1, calls)

	used = false
	wrongRPC, err := issuer.IssueService(principal.ServiceInput{Audience: Audience, RPC: "/voice.chat.v1.ChatService/CreateChat", RequestID: requestID, RequestHash: hash})
	require.NoError(t, err)
	_, err = call(request, requestID, wrongRPC)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	wrongRequestID, err := issuer.IssueService(principal.ServiceInput{Audience: Audience, RPC: ProvisionMethod, RequestID: "00000000-0000-4000-8000-000000000002", RequestHash: hash})
	require.NoError(t, err)
	_, err = call(request, requestID, wrongRequestID)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	_, err = call(request, "00000000-0000-4000-8000-000000000002", token)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	request.Name = "changed after signing"
	_, err = call(request, requestID, token)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	request.Name = "Party"
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", requestID, "x-profile-id", "raw-user"))
	_, err = interceptor(ctx, request, info, handler)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestGISAllowlistIsExact(t *testing.T) {
	require.True(t, AllowsMethod(ProvisionMethod))
	require.True(t, AllowsMethod(SyncMembersMethod))
	require.False(t, AllowsMethod("/voice.chat.v1.ChatService/CreateChat"))
	require.False(t, AllowsMethod("/voice.chat.v1.GameIntegrationChatService/DeleteManagedChat"))
}

func TestNoGISConfigDisablesProtectedListener(t *testing.T) {
	for _, key := range []string{"CHAT_GIS_TLS_CERT_FILE", "CHAT_GIS_TLS_KEY_FILE", "CHAT_GIS_CLIENT_CA_FILE", "GAME_INTEGRATION_PRINCIPAL_JWKS_URL", "GAME_INTEGRATION_PRINCIPAL_JWKS_CA_FILE", "GAME_INTEGRATION_PRINCIPAL_REPLAY_REDIS_ADDR", "GAME_INTEGRATION_PRINCIPAL_REPLAY_REDIS_PASSWORD"} {
		t.Setenv(key, "")
	}
	_, configured, err := ConfigFromEnv()
	require.NoError(t, err)
	require.False(t, configured)
}

func TestRuntimeRejectsNonTLSJWKSURLBeforeLoadingSecrets(t *testing.T) {
	_, err := New(context.Background(), Config{TLSCertFile: "unused", TLSKeyFile: "unused", ClientCAFile: "unused", JWKSURL: "http://gis.test/jwks", ReplayAddr: "unused"})
	require.ErrorContains(t, err, "HTTPS endpoint")
}

func TestGISListenerRequiresVerifiedClientCertificate(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	now := time.Now()
	certificateDER, err := x509.CreateCertificate(rand.Reader, &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "test-ca"},
		NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour), IsCA: true,
		BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}, DNSNames: []string{"localhost"},
	}, &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "test-ca"}, IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature}, &key.PublicKey, key)
	require.NoError(t, err)
	dir := t.TempDir()
	certPath, keyPath, caPath := filepath.Join(dir, "server.crt"), filepath.Join(dir, "server.key"), filepath.Join(dir, "client-ca.crt")
	require.NoError(t, os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificateDER}), 0600))
	require.NoError(t, os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}), 0600))
	require.NoError(t, os.WriteFile(caPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificateDER}), 0600))
	serverConfig, err := loadServerTLS(certPath, keyPath, caPath)
	require.NoError(t, err)
	require.Equal(t, tls.RequireAndVerifyClientCert, serverConfig.ClientAuth)
	require.Equal(t, uint16(tls.VersionTLS12), serverConfig.MinVersion)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()
	serverDone := make(chan error, 1)
	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			serverDone <- acceptErr
			return
		}
		server := tls.Server(conn, serverConfig)
		serverDone <- server.Handshake()
		_ = server.Close()
	}()
	roots := x509.NewCertPool()
	parsedCA, err := x509.ParseCertificate(certificateDER)
	require.NoError(t, err)
	roots.AddCert(parsedCA)
	client, err := tls.Dial("tcp", listener.Addr().String(), &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: "localhost"})
	clientErr := err
	if client != nil {
		_ = client.Close()
	}
	serverErr := <-serverDone
	require.Error(t, serverErr, "server must reject a client without a GIS certificate")
	t.Logf("server rejected client without certificate: %v", serverErr)
	_ = clientErr // Some TLS versions deliver the server's certificate alert after the client handshake returns.
}
