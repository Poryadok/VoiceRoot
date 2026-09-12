package principalruntime

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
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	commonv1 "voice.app/voice/common/v1"
	searchv1 "voice.app/voice/search/v1"
	"voice/backend/pkg/principal"
)

func TestReplayGuardIsAtomicSharedAndFailsClosed(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	runtime := &Runtime{replay: client}
	expiry := time.Now().Add(20 * time.Second)
	require.NoError(t, runtime.recordReplay(context.Background(), "space", "jti-1", expiry))
	require.Error(t, runtime.recordReplay(context.Background(), "space", "jti-1", expiry), "the same credential is one-time across instances")
	keys := server.Keys()
	require.Len(t, keys, 1)
	require.Contains(t, keys[0], "search:principal:replay:")
	require.Positive(t, server.TTL(keys[0]))
	require.NoError(t, client.Close())
	require.Error(t, runtime.recordReplay(context.Background(), "space", "jti-2", expiry), "Redis failure must fail closed")
}

type protectedSearchFixture struct {
	searchv1.UnimplementedSearchServiceServer
}

func (protectedSearchFixture) ApplySpaceLifecycleFence(context.Context, *searchv1.ApplySpaceLifecycleFenceRequest) (*searchv1.ApplySpaceLifecycleFenceResponse, error) {
	return &searchv1.ApplySpaceLifecycleFenceResponse{}, nil
}

func TestProtectedListenerRequiresTLSVerifiesExactPrincipalAndRejectsReplay(t *testing.T) {
	active, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	next, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	jwk := func(kid string, key *rsa.PrivateKey) map[string]string {
		return map[string]string{"kty": "RSA", "kid": kid, "use": "sig", "alg": "RS256", "n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()), "e": "AQAB"}
	}
	document, err := json.Marshal(map[string]any{"keys": []any{jwk("current", active), jwk("next", next)}})
	require.NoError(t, err)
	jwksServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(document)
	}))
	defer jwksServer.Close()
	temp := t.TempDir()
	jwksCA := filepath.Join(temp, "jwks-ca.pem")
	require.NoError(t, os.WriteFile(jwksCA, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: jwksServer.Certificate().Raw}), 0o600))
	serverKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	require.NoError(t, err)
	template := &x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: "localhost"}, DNSNames: []string{"localhost"}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &serverKey.PublicKey, serverKey)
	require.NoError(t, err)
	certPath, keyPath := filepath.Join(temp, "server.pem"), filepath.Join(temp, "server-key.pem")
	require.NoError(t, os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600))
	encodedKey, err := x509.MarshalPKCS8PrivateKey(serverKey)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: encodedKey}), 0o600))
	redisServer := miniredis.RunT(t)
	runtime, err := New(context.Background(), Config{JWKSURLs: map[string]string{"space": jwksServer.URL}, RefreshAfter: 30 * time.Second, HardExpiry: 2 * time.Minute, UnknownKIDCooldown: 5 * time.Second, ReplayAddr: redisServer.Addr(), JWKSCAFile: jwksCA, TLSCertFile: certPath, TLSKeyFile: keyPath, ListenAddr: "127.0.0.1:0"})
	require.NoError(t, err)
	defer func() { require.NoError(t, runtime.Close()) }()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	server := grpc.NewServer(runtime.ServerOptions()...)
	searchv1.RegisterSearchServiceServer(server, protectedSearchFixture{})
	go func() { _ = server.Serve(listener) }()
	defer server.Stop()
	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})))
	conn, err := grpc.NewClient(listener.Addr().String(), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: "localhost"})))
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()
	client := searchv1.NewSearchServiceClient(conn)
	req := &searchv1.ApplySpaceLifecycleFenceRequest{Fence: &commonv1.SpaceLifecycleFenceRequest{ProtocolVersion: 1}}
	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "space", KeyID: "current", PrivateKey: active})
	require.NoError(t, err)
	token, err := issuer.IssueService(principal.ServiceInput{Audience: "search", RPC: searchv1.SearchService_ApplySpaceLifecycleFence_FullMethodName, RequestID: "request-1", RequestHash: hash})
	require.NoError(t, err)
	ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", "request-1"))
	_, err = client.ApplySpaceLifecycleFence(ctx, req)
	require.NoError(t, err)
	_, err = client.ApplySpaceLifecycleFence(ctx, req)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
}
