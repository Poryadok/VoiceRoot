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

type manifestTransportServer struct {
	chatv1.UnimplementedChatServiceServer
	calls atomic.Int32
}

func (s *manifestTransportServer) GetSpacePurgeManifestPage(ctx context.Context, _ *chatv1.GetSpacePurgeManifestPageRequest) (*chatv1.GetSpacePurgeManifestPageResponse, error) {
	p, ok := principal.FromContext(ctx)
	if !ok || p.Kind != "service" || p.Issuer != "search" || p.Audience != "chat" || p.RPC != GetManifestPageMethod {
		return nil, status.Error(codes.Internal, "verified Search page reader missing")
	}
	s.calls.Add(1)
	return &chatv1.GetSpacePurgeManifestPageResponse{}, nil
}

type manifestTestCA struct {
	certificate *x509.Certificate
	key         *rsa.PrivateKey
	pem         []byte
}

func newManifestTestCA(t *testing.T, serial int64) manifestTestCA {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	template := &x509.Certificate{SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: "manifest transport CA"}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	return manifestTestCA{cert, key, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})}
}
func (ca manifestTestCA) leaf(t *testing.T, serial int64, usage x509.ExtKeyUsage) (tls.Certificate, string, string) {
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

func TestSearchManifestRealTLSJWKSReplayAndNarrowAllowlist(t *testing.T) {
	serverCA, clientCA, foreignCA := newManifestTestCA(t, 1), newManifestTestCA(t, 2), newManifestTestCA(t, 3)
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
	cfg := Config{Issuer: "search", Audience: Audience, AllowedMethods: []string{GetManifestPageMethod}, TLSCertFile: certFile, TLSKeyFile: keyFile, ClientCAFile: clientCAFile, JWKSURL: jwks.URL, JWKSCAFile: serverCAFile, JWKSClientCertFile: clientCertFile, JWKSClientKeyFile: clientKeyFile, ReplayAddr: replay.Addr()}
	runtime, err := New(context.Background(), cfg)
	require.NoError(t, err)
	defer runtime.Close()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	service := &manifestTransportServer{}
	server := grpc.NewServer(runtime.ServerOptions()...)
	chatv1.RegisterChatServiceServer(server, service)
	go func() { _ = server.Serve(listener) }()
	defer server.Stop()
	connect := func(certificates []tls.Certificate) *grpc.ClientConn {
		conn, err := grpc.NewClient(listener.Addr().String(), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: serverRoots, ServerName: "localhost", Certificates: certificates})))
		require.NoError(t, err)
		t.Cleanup(func() { _ = conn.Close() })
		return conn
	}
	connection := connect([]tls.Certificate{clientCertificate})
	request := &chatv1.GetSpacePurgeManifestPageRequest{ProtocolVersion: 1, SpaceId: "00000000-0000-4000-8000-000000000001", DeletionOperationId: "00000000-0000-4000-8000-000000000002", ManifestId: "sealed", Generation: 1}
	sign := func(issuerName, method, requestID string) string {
		issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: issuerName, KeyID: "current", PrivateKey: current})
		require.NoError(t, err)
		hash, err := principal.RequestHash(request)
		require.NoError(t, err)
		token, err := issuer.IssueService(principal.ServiceInput{Audience: Audience, RPC: method, RequestID: requestID, RequestHash: hash})
		require.NoError(t, err)
		return token
	}
	call := func(conn *grpc.ClientConn, token, requestID string) (*chatv1.GetSpacePurgeManifestPageResponse, error) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", requestID))
		return chatv1.NewChatServiceClient(conn).GetSpacePurgeManifestPage(ctx, request)
	}
	token := sign("search", GetManifestPageMethod, request.DeletionOperationId)
	_, err = call(connection, token, request.DeletionOperationId)
	require.NoError(t, err)
	require.EqualValues(t, 1, service.calls.Load())
	_, err = call(connection, token, request.DeletionOperationId)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	for _, issuerName := range []string{"space", "messaging", "gameintegration"} {
		_, err = call(connection, sign(issuerName, GetManifestPageMethod, request.DeletionOperationId), request.DeletionOperationId)
		require.Equal(t, codes.Unauthenticated, status.Code(err))
	}
	for _, certificates := range [][]tls.Certificate{nil, {foreignCertificate}} {
		_, err = call(connect(certificates), sign("search", GetManifestPageMethod, request.DeletionOperationId), request.DeletionOperationId)
		require.Error(t, err)
	}
	wrongID := "00000000-0000-4000-8000-000000000003"
	_, err = call(connection, sign("search", GetManifestPageMethod, wrongID), wrongID)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	token = sign("search", GetManifestPageMethod, request.DeletionOperationId)
	request.PageToken = "tampered"
	_, err = call(connection, token, request.DeletionOperationId)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	request.PageToken = ""
	for _, method := range []string{PrepareManifestMethod, ApplyLifecycleMethod, PurgeSpaceMethod, ProvisionMethod} {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+sign("search", method, request.DeletionOperationId), "x-request-id", request.DeletionOperationId))
		err = connection.Invoke(ctx, method, request, &chatv1.GetSpacePurgeManifestPageResponse{})
		cancel()
		if method == ProvisionMethod {
			require.Equal(t, codes.Unimplemented, status.Code(err), "GIS service is absent from the Search listener")
		} else {
			require.Equal(t, codes.PermissionDenied, status.Code(err))
		}
	}
	require.EqualValues(t, 1, service.calls.Load(), "denials must precede the page handler")
	_, err = call(connection, sign("search", GetManifestPageMethod, request.DeletionOperationId), request.DeletionOperationId)
	require.NoError(t, err)
	require.EqualValues(t, 2, service.calls.Load(), "fresh JWT permits an exact page retry")
}

func TestSearchManifestConfigurationFailsClosed(t *testing.T) {
	keys := []string{"CHAT_SEARCH_MANIFEST_GRPC_LISTEN", "CHAT_SEARCH_MANIFEST_TLS_CERT_FILE", "CHAT_SEARCH_MANIFEST_TLS_KEY_FILE", "CHAT_SEARCH_MANIFEST_CLIENT_CA_FILE", "CHAT_SEARCH_PRINCIPAL_JWKS_URL", "CHAT_SEARCH_PRINCIPAL_JWKS_CA_FILE", "CHAT_SEARCH_PRINCIPAL_REPLAY_REDIS_ADDR", "CHAT_SEARCH_PRINCIPAL_REPLAY_REDIS_PASSWORD", "CHAT_SEARCH_PRINCIPAL_JWKS_CLIENT_CERT_FILE", "CHAT_SEARCH_PRINCIPAL_JWKS_CLIENT_KEY_FILE"}
	for _, key := range keys {
		t.Setenv(key, "")
	}
	_, _, configured, err := SearchManifestConfigFromEnv()
	require.NoError(t, err)
	require.False(t, configured)
	t.Setenv(keys[0], ":9093")
	_, _, configured, err = SearchManifestConfigFromEnv()
	require.Error(t, err)
	require.True(t, configured)
	for _, key := range keys[:7] {
		t.Setenv(key, "fixture")
	}
	cfg, listen, configured, err := SearchManifestConfigFromEnv()
	require.NoError(t, err)
	require.True(t, configured)
	require.Equal(t, "fixture", listen)
	require.Equal(t, "search", cfg.Issuer)
	require.Equal(t, []string{GetManifestPageMethod}, cfg.AllowedMethods)
	t.Setenv("CHAT_SEARCH_PRINCIPAL_JWKS_CLIENT_CERT_FILE", "client.crt")
	_, _, _, err = SearchManifestConfigFromEnv()
	require.Error(t, err)
}
