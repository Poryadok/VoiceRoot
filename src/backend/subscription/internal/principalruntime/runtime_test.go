package principalruntime

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
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
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"voice/backend/pkg/principal"

	subscriptionv1 "voice.app/voice/subscription/v1"
)

const correctionRuntimeHash = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

type correctionRuntimeFixture struct {
	config Config
	key    *rsa.PrivateKey
	next   *rsa.PrivateKey
	redis  *miniredis.Miniredis
}

func newCorrectionRuntimeFixture(t *testing.T) correctionRuntimeFixture {
	t.Helper()
	current, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	next, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	jwk := func(kid string, key *rsa.PrivateKey) map[string]string {
		return map[string]string{"kid": kid, "kty": "RSA", "use": "sig", "alg": "RS256", "n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()), "e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes())}
	}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{jwk("current", current), jwk("next", next)}})
	}))
	t.Cleanup(server.Close)
	dir := t.TempDir()
	certFile, keyFile := filepath.Join(dir, "tls.crt"), filepath.Join(dir, "tls.key")
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.TLS.Certificates[0].Certificate[0]})
	require.NoError(t, os.WriteFile(certFile, certPEM, 0o600))
	keyDER, err := x509.MarshalPKCS8PrivateKey(server.TLS.Certificates[0].PrivateKey)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}), 0o600))
	replay := miniredis.RunT(t)
	return correctionRuntimeFixture{config: Config{
		JWKSURLs: map[string]string{"space": server.URL}, RefreshAfter: time.Minute, HardExpiry: 2 * time.Minute,
		UnknownKIDCooldown: time.Second, ReplayAddr: replay.Addr(), JWKSCAFile: certFile,
		TLSCertFile: certFile, TLSKeyFile: keyFile, ListenAddr: "127.0.0.1:0",
	}, key: current, next: next, redis: replay}
}

func correctionRuntimeToken(t *testing.T, key *rsa.PrivateKey, kid, audience, rpc, requestID, hash string) string {
	t.Helper()
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "space", KeyID: kid, PrivateKey: key})
	require.NoError(t, err)
	token, err := issuer.IssueService(principal.ServiceInput{Audience: audience, RPC: rpc, RequestID: requestID, RequestHash: hash})
	require.NoError(t, err)
	return token
}

func TestR23CorrectionRuntimeVerifiesExactLifecycleBindingAndRejectsReplay(t *testing.T) {
	f := newCorrectionRuntimeFixture(t)
	runtime, err := New(context.Background(), f.config)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, runtime.Close()) })
	for i, tc := range []struct {
		kid, method string
		key         *rsa.PrivateKey
	}{
		{"current", subscriptionv1.SubscriptionService_ApplySpaceLifecycleFence_FullMethodName, f.key},
		{"next", subscriptionv1.SubscriptionService_PurgeSpace_FullMethodName, f.next},
	} {
		requestID := "runtime-" + string(rune('a'+i))
		token := correctionRuntimeToken(t, tc.key, tc.kid, "subscription", tc.method, requestID, correctionRuntimeHash)
		verified, err := runtime.Verify(context.Background(), token, tc.method, requestID, correctionRuntimeHash)
		require.NoError(t, err)
		require.Equal(t, "service:space", verified.Subject)
		require.Equal(t, "subscription", verified.Audience)
		require.Equal(t, tc.method, verified.RPC)
		_, err = runtime.Verify(context.Background(), token, tc.method, requestID, correctionRuntimeHash)
		require.Error(t, err)
	}
	method := subscriptionv1.SubscriptionService_ApplySpaceLifecycleFence_FullMethodName
	for _, tc := range []struct{ name, audience, rpc, requestID, hash string }{
		{"audience", "role", method, "request", correctionRuntimeHash},
		{"rpc", "subscription", subscriptionv1.SubscriptionService_PurgeSpace_FullMethodName, "request", correctionRuntimeHash},
		{"request_id", "subscription", method, "different", correctionRuntimeHash},
		{"hash", "subscription", method, "request", "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			token := correctionRuntimeToken(t, f.key, "current", tc.audience, tc.rpc, tc.requestID, tc.hash)
			_, err := runtime.Verify(context.Background(), token, method, "request", correctionRuntimeHash)
			require.Error(t, err)
		})
	}
}

type correctionRuntimeServer struct {
	subscriptionv1.UnimplementedSubscriptionServiceServer
	principals chan principal.Principal
}

func (s *correctionRuntimeServer) ApplySpaceLifecycleFence(ctx context.Context, _ *subscriptionv1.ApplySpaceLifecycleFenceRequest) (*subscriptionv1.ApplySpaceLifecycleFenceResponse, error) {
	verified, _ := principal.FromContext(ctx)
	s.principals <- verified
	return &subscriptionv1.ApplySpaceLifecycleFenceResponse{}, nil
}

func TestR23CorrectionProtectedListenerAuthenticatesBeforeHandler(t *testing.T) {
	f := newCorrectionRuntimeFixture(t)
	runtime, err := New(context.Background(), f.config)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, runtime.Close()) })
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	server := grpc.NewServer(runtime.ServerOptions()...)
	recorder := &correctionRuntimeServer{principals: make(chan principal.Principal, 1)}
	subscriptionv1.RegisterSubscriptionServiceServer(server, recorder)
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	t.Cleanup(func() { server.Stop(); <-done })
	ca, err := os.ReadFile(f.config.TLSCertFile)
	require.NoError(t, err)
	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM(ca))
	conn, err := grpc.NewClient(listener.Addr().String(), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots})))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })
	client := subscriptionv1.NewSubscriptionServiceClient(conn)
	request := &subscriptionv1.ApplySpaceLifecycleFenceRequest{}
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	method := subscriptionv1.SubscriptionService_ApplySpaceLifecycleFence_FullMethodName
	token := correctionRuntimeToken(t, f.key, "current", "subscription", method, "listener", hash)
	ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", "listener"))
	_, err = client.ApplySpaceLifecycleFence(ctx, request)
	require.NoError(t, err)
	verified := <-recorder.principals
	require.Equal(t, "service:space", verified.Subject)

	_, err = client.ApplySpaceLifecycleFence(ctx, request)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	_, err = client.GetSubscription(ctx, &subscriptionv1.GetSubscriptionRequest{})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	raw := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+correctionRuntimeToken(t, f.key, "current", "subscription", method, "raw", hash), "x-request-id", "raw", "x-voice-service-id", "space"))
	_, err = client.ApplySpaceLifecycleFence(raw, request)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
}
