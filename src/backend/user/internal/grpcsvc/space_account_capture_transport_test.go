package grpcsvc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"
	userv1 "voice.app/voice/user/v1"
	"voice/backend/pkg/principal"
	"voice/backend/pkg/socialprincipal"
	"voice/backend/pkg/spaceaccountcapture"
)

// This is transport authority proof against real TLS/JWKS/Redis. The empty
// cohort authority is a fixture; it is not a PostgreSQL association producer.
func TestSpaceCaptureNativeProtectedTransport(t *testing.T) {
	replayAddr := os.Getenv("USER_ASSOCIATION_TEST_REDIS")
	if replayAddr == "" {
		t.Skip("set USER_ASSOCIATION_TEST_REDIS to owned real Redis")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	certPath := filepath.Join(repoRoot(t), "src/backend/auth/src/test/resources/principal-tls/server-cert.pem")
	keyPath := filepath.Join(repoRoot(t), "src/backend/auth/src/test/resources/principal-tls/server-key.pem")
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	require.NoError(t, err)
	pem, err := os.ReadFile(certPath)
	require.NoError(t, err)
	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM(pem))
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	next, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	jwks, err := json.Marshal(map[string]any{"keys": []map[string]string{sdkPrincipalJWK(key, "current"), sdkPrincipalJWK(next, "next")}})
	require.NoError(t, err)
	jwksServer := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(jwks)
	}))
	jwksServer.TLS = &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{cert}, ClientCAs: roots, ClientAuth: tls.RequireAndVerifyClientCert}
	jwksServer.StartTLS()
	defer jwksServer.Close()
	cfg := socialprincipal.Config{Target: "user", Capability: "space-account-capture", JWKSURLs: map[string]string{"space": strings.Replace(jwksServer.URL, "127.0.0.1", "localhost", 1)}, JWKSCAFile: certPath,
		JWKSClientCertFile: certPath, JWKSClientKeyFile: keyPath, ClientCAFile: certPath, ClientCAs: roots, RefreshAfter: 30 * time.Second, HardExpiry: 2 * time.Minute, UnknownKIDCooldown: time.Second,
		ReplayAddr: replayAddr, TLSCertFile: certPath, TLSKeyFile: keyPath, ListenAddr: ":0"}
	var calls atomic.Int32
	authority := captureAuthorityFunc(func(_ context.Context, r *userv1.CaptureSpaceMemberAccountBindingsRequest) (*userv1.CaptureSpaceMemberAccountBindingsResponse, error) {
		calls.Add(1)
		return &userv1.CaptureSpaceMemberAccountBindingsResponse{Protocol: r.Protocol, OperationId: r.OperationId, Purpose: r.Purpose, SpaceId: r.SpaceId, CohortDigest: r.CohortDigest, RequestDigest: spaceaccountcapture.RequestDigest(r), OwnerSourceRevision: 17, UserSourceGeneration: "11111111-1111-4111-8111-111111111111", IdentityCut: captureTestIdentityCut()}, nil
	})
	start := func() (userv1.UserServiceClient, func()) {
		runtime, err := socialprincipal.New(ctx, cfg)
		require.NoError(t, err)
		listener := bufconn.Listen(1 << 20)
		server := grpc.NewServer(runtime.ServerOptions()...)
		RegisterSpaceAccountCaptureServer(server, authority)
		go func() { _ = server.Serve(listener) }()
		conn, err := grpc.NewClient("passthrough:///capture", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }),
			grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: "localhost", Certificates: []tls.Certificate{cert}})))
		require.NoError(t, err)
		return userv1.NewUserServiceClient(conn), func() { _ = conn.Close(); server.Stop(); _ = listener.Close(); require.NoError(t, runtime.Close()) }
	}
	r := captureTestRequest()
	tokenContext := func(issuerName, audience, method string) context.Context {
		issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: issuerName, KeyID: "current", PrivateKey: key})
		require.NoError(t, err)
		hash, err := principal.RequestHash(r)
		require.NoError(t, err)
		requestID := uuid.NewString()
		token, err := issuer.IssueService(principal.ServiceInput{Audience: audience, RPC: method, RequestID: requestID, RequestHash: hash})
		require.NoError(t, err)
		return metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", requestID))
	}
	method := userv1.UserService_CaptureSpaceMemberAccountBindings_FullMethodName
	client, closeFirst := start()
	_, err = client.CaptureSpaceMemberAccountBindings(metadata.NewOutgoingContext(ctx, metadata.Pairs("x-voice-internal-caller", "space")), r)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	for _, issuer := range []string{"auth", "role", "gateway", "moderation"} {
		t.Run(issuer, func(t *testing.T) {
			_, err := client.CaptureSpaceMemberAccountBindings(tokenContext(issuer, "user", method), r)
			require.Equal(t, codes.Unauthenticated, status.Code(err))
		})
	}
	_, err = client.CaptureSpaceMemberAccountBindings(tokenContext("space", "space", method), r)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	_, err = client.CaptureSpaceMemberAccountBindings(tokenContext("space", "user", userv1.UserService_GetSpaceAdmissionProfileEligibility_FullMethodName), r)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	changed := proto.Clone(r).(*userv1.CaptureSpaceMemberAccountBindingsRequest)
	changed.OperationId = uuid.NewString()
	_, err = client.CaptureSpaceMemberAccountBindings(tokenContext("space", "user", method), changed)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	require.Zero(t, calls.Load())
	original := tokenContext("space", "user", method)
	out, err := client.CaptureSpaceMemberAccountBindings(original, r)
	require.NoError(t, err)
	require.NoError(t, spaceaccountcapture.ValidateResponse(r, out))
	_, err = client.CaptureSpaceMemberAccountBindings(original, r)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	_, err = client.GetSpaceAdmissionProfileEligibility(tokenContext("space", "user", method), &userv1.GetSpaceAdmissionProfileEligibilityRequest{})
	require.Equal(t, codes.Unimplemented, status.Code(err))
	closeFirst()
	client, closeSecond := start()
	defer closeSecond()
	_, err = client.CaptureSpaceMemberAccountBindings(original, r)
	require.Equal(t, codes.Unauthenticated, status.Code(err), "shared replay remains effective after listener restart")
	_, err = client.CaptureSpaceMemberAccountBindings(tokenContext("space", "user", method), r)
	require.NoError(t, err)
	require.EqualValues(t, 2, calls.Load())
}
