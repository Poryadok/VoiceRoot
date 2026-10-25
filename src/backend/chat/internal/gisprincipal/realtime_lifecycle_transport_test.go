package gisprincipal

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/pkg/principal"
)

type realtimeTransportServer struct {
	chatv1.UnimplementedChatRealtimeLifecycleServiceServer
	calls atomic.Int32
}

func (s *realtimeTransportServer) ResolveChatLifecycle(ctx context.Context, _ *chatv1.ResolveChatLifecycleRequest) (*chatv1.ResolveChatLifecycleResponse, error) {
	p, ok := principal.FromContext(ctx)
	if !ok || p.Kind != "service" || p.Issuer != "realtime" || p.Audience != "chat" || p.RPC != ResolveRealtimeLifecycleMethod {
		return nil, status.Error(codes.Internal, "verified Search page reader missing")
	}
	s.calls.Add(1)
	return &chatv1.ResolveChatLifecycleResponse{}, nil
}

type realtimeTestCA struct {
	certificate *x509.Certificate
	key         *rsa.PrivateKey
	pem         []byte
}

func newRealtimeTestCA(t *testing.T, serial int64) realtimeTestCA {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	template := &x509.Certificate{SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: "manifest transport CA"}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	return realtimeTestCA{cert, key, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})}
}
func (ca realtimeTestCA) leaf(t *testing.T, serial int64, usage x509.ExtKeyUsage) (tls.Certificate, string, string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	template := &x509.Certificate{SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: "manifest leaf"}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), BasicConstraintsValid: true, KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{usage}, DNSNames: []string{"localhost"}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}}
	der, err := x509.CreateCertificate(rand.Reader, template, ca.certificate, &key.PublicKey, ca.key)
	require.NoError(t, err)
	certPath, keyPath := filepath.Join(t.TempDir(), "leaf.crt"), filepath.Join(t.TempDir(), "leaf.key")
	require.NoError(t, os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600))
	require.NoError(t, os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}), 0600))
	certificate, err := tls.LoadX509KeyPair(certPath, keyPath)
	require.NoError(t, err)
	return certificate, certPath, keyPath
}

func TestRealtimeLifecycleRealTLSJWKSReplayAndNarrowAllowlist(t *testing.T) {
	serverCA, clientCA, foreignCA := newRealtimeTestCA(t, 1), newRealtimeTestCA(t, 2), newRealtimeTestCA(t, 3)
	serverCertificate, certFile, keyFile := serverCA.leaf(t, 10, x509.ExtKeyUsageServerAuth)
	clientCertificate, clientCertFile, clientKeyFile := clientCA.leaf(t, 11, x509.ExtKeyUsageClientAuth)
	foreignCertificate, _, _ := foreignCA.leaf(t, 12, x509.ExtKeyUsageClientAuth)
	serverRoots, clientRoots := x509.NewCertPool(), x509.NewCertPool()
	require.True(t, serverRoots.AppendCertsFromPEM(serverCA.pem))
	require.True(t, clientRoots.AppendCertsFromPEM(clientCA.pem))
	serverCAFile, clientCAFile := filepath.Join(t.TempDir(), "server-ca.crt"), filepath.Join(t.TempDir(), "client-ca.crt")
	require.NoError(t, os.WriteFile(serverCAFile, serverCA.pem, 0600))
	require.NoError(t, os.WriteFile(clientCAFile, clientCA.pem, 0600))
	current, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	next, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	keys := make([]map[string]string, 0, 2)
	for kid, key := range map[string]*rsa.PrivateKey{"current": current, "next": next} {
		keys = append(keys, map[string]string{"kty": "RSA", "kid": kid, "use": "sig", "alg": "RS256", "n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()), "e": "AQAB"})
	}
	jwks := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NotEmpty(t, r.TLS.VerifiedChains)
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"keys": keys}))
	}))
	jwks.TLS = &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{serverCertificate}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: clientRoots}
	jwks.StartTLS()
	defer jwks.Close()
	replay := miniredis.RunT(t)
	cfg := Config{Issuer: "realtime", Audience: Audience, AllowedMethods: []string{ResolveRealtimeLifecycleMethod}, TLSCertFile: certFile, TLSKeyFile: keyFile, ClientCAFile: clientCAFile, JWKSURL: jwks.URL, JWKSCAFile: serverCAFile, JWKSClientCertFile: clientCertFile, JWKSClientKeyFile: clientKeyFile, ReplayAddr: replay.Addr()}
	runtime, err := New(context.Background(), cfg)
	require.NoError(t, err)
	defer runtime.Close()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	service := &realtimeTransportServer{}
	server := grpc.NewServer(runtime.ServerOptions()...)
	chatv1.RegisterChatRealtimeLifecycleServiceServer(server, service)
	go func() { _ = server.Serve(listener) }()
	defer server.Stop()
	connect := func(certificates []tls.Certificate) *grpc.ClientConn {
		conn, err := grpc.NewClient(listener.Addr().String(), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: serverRoots, ServerName: "localhost", Certificates: certificates})))
		require.NoError(t, err)
		t.Cleanup(func() { _ = conn.Close() })
		return conn
	}
	connection := connect([]tls.Certificate{clientCertificate})
	request := &chatv1.ResolveChatLifecycleRequest{ProtocolVersion: 1, ChatId: "00000000-0000-4000-8000-000000000001", OperationId: "00000000-0000-4000-8000-000000000002"}
	sign := func(issuerName, method, requestID string) string {
		issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: issuerName, KeyID: "current", PrivateKey: current})
		require.NoError(t, err)
		hash, err := principal.RequestHash(request)
		require.NoError(t, err)
		token, err := issuer.IssueService(principal.ServiceInput{Audience: Audience, RPC: method, RequestID: requestID, RequestHash: hash})
		require.NoError(t, err)
		return token
	}
	call := func(conn *grpc.ClientConn, token, requestID string) (*chatv1.ResolveChatLifecycleResponse, error) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", requestID))
		return chatv1.NewChatRealtimeLifecycleServiceClient(conn).ResolveChatLifecycle(ctx, request)
	}
	token := sign("realtime", ResolveRealtimeLifecycleMethod, request.OperationId)
	_, err = call(connection, token, request.OperationId)
	require.NoError(t, err)
	require.EqualValues(t, 1, service.calls.Load())
	_, err = call(connection, token, request.OperationId)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	for _, issuerName := range []string{"space", "messaging", "gameintegration"} {
		_, err = call(connection, sign(issuerName, ResolveRealtimeLifecycleMethod, request.OperationId), request.OperationId)
		require.Equal(t, codes.Unauthenticated, status.Code(err))
	}
	for _, certificates := range [][]tls.Certificate{nil, {foreignCertificate}} {
		_, err = call(connect(certificates), sign("realtime", ResolveRealtimeLifecycleMethod, request.OperationId), request.OperationId)
		require.Error(t, err)
	}
	wrongID := "00000000-0000-4000-8000-000000000003"
	_, err = call(connection, sign("realtime", ResolveRealtimeLifecycleMethod, wrongID), wrongID)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	token = sign("realtime", ResolveRealtimeLifecycleMethod, request.OperationId)
	request.ChatId = "tampered"
	_, err = call(connection, token, request.OperationId)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	request.ChatId = "00000000-0000-4000-8000-000000000001"
	for _, method := range []string{PrepareManifestMethod, ApplyLifecycleMethod, PurgeSpaceMethod, ProvisionMethod} {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+sign("realtime", method, request.OperationId), "x-request-id", request.OperationId))
		err = connection.Invoke(ctx, method, request, &chatv1.ResolveChatLifecycleResponse{})
		cancel()
		require.Equal(t, codes.Unimplemented, status.Code(err), "other services absent from native resolver listener")
	}
	require.EqualValues(t, 1, service.calls.Load(), "denials must precede the page handler")
	_, err = call(connection, sign("realtime", ResolveRealtimeLifecycleMethod, request.OperationId), request.OperationId)
	require.NoError(t, err)
	require.EqualValues(t, 2, service.calls.Load(), "fresh JWT permits an exact page retry")
}

func TestRealtimeLifecycleConfigurationFailsClosed(t *testing.T) {
	keys := []string{"CHAT_REALTIME_LIFECYCLE_GRPC_LISTEN", "CHAT_REALTIME_LIFECYCLE_TLS_CERT_FILE", "CHAT_REALTIME_LIFECYCLE_TLS_KEY_FILE", "CHAT_REALTIME_LIFECYCLE_CLIENT_CA_FILE", "CHAT_REALTIME_PRINCIPAL_JWKS_URL", "CHAT_REALTIME_PRINCIPAL_JWKS_CA_FILE", "CHAT_REALTIME_PRINCIPAL_REPLAY_REDIS_ADDR", "CHAT_REALTIME_PRINCIPAL_REPLAY_REDIS_PASSWORD", "CHAT_REALTIME_PRINCIPAL_JWKS_CLIENT_CERT_FILE", "CHAT_REALTIME_PRINCIPAL_JWKS_CLIENT_KEY_FILE"}
	for _, key := range keys {
		t.Setenv(key, "")
	}
	_, _, configured, err := RealtimeLifecycleConfigFromEnv()
	require.NoError(t, err)
	require.False(t, configured)
	t.Setenv(keys[0], ":9093")
	_, _, configured, err = RealtimeLifecycleConfigFromEnv()
	require.Error(t, err)
	require.True(t, configured)
	for _, key := range keys[:7] {
		t.Setenv(key, "fixture")
	}
	cfg, listen, configured, err := RealtimeLifecycleConfigFromEnv()
	require.NoError(t, err)
	require.True(t, configured)
	require.Equal(t, "fixture", listen)
	require.Equal(t, "realtime", cfg.Issuer)
	require.Equal(t, []string{ResolveRealtimeLifecycleMethod}, cfg.AllowedMethods)
	t.Setenv("CHAT_REALTIME_PRINCIPAL_JWKS_CLIENT_CERT_FILE", "client.crt")
	_, _, _, err = RealtimeLifecycleConfigFromEnv()
	require.Error(t, err)
}
