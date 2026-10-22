package lifecycleprincipal

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
)

type creatorTransportServer struct {
	spacev1.UnimplementedSpaceServiceServer
	calls atomic.Int32
}

func (s *creatorTransportServer) CreateSpace(ctx context.Context, req *spacev1.CreateSpaceRequest) (*spacev1.CreateSpaceResponse, error) {
	creator, err := RequireCreator(ctx, req)
	if err != nil {
		return nil, err
	}
	actor, err := creator.ActorFor(req)
	if err != nil {
		return nil, err
	}
	s.calls.Add(1)
	return &spacev1.CreateSpaceResponse{Space: &spacev1.Space{Name: req.Name, OwnerProfileId: actor.ProfileID}}, nil
}

// These callbacks deliberately model authority responses only. This proves the
// production transport/capability boundary, not live Auth or PostgreSQL readiness.
func TestCreatorRuntimeTLSAndDedicatedCallbacks(t *testing.T) {
	serverCA, clientCA := newTransportCA(t, 101), newTransportCA(t, 102)
	serverCert, certFile, keyFile := serverCA.leaf(t, 103, x509.ExtKeyUsageServerAuth)
	clientCert, _, _ := clientCA.leaf(t, 104, x509.ExtKeyUsageClientAuth)
	root := t.TempDir()
	caFile, clientCAFile := filepath.Join(root, "server.crt"), filepath.Join(root, "client.crt")
	require.NoError(t, os.WriteFile(caFile, serverCA.pem, 0600))
	require.NoError(t, os.WriteFile(clientCAFile, clientCA.pem, 0600))
	keys := map[string]*rsa.PrivateKey{}
	document := []map[string]string{}
	for _, kid := range []string{"current", "next"} {
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)
		keys[kid] = key
		document = append(document, map[string]string{"kty": "RSA", "kid": kid, "alg": "RS256", "use": "sig", "n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()), "e": "AQAB"})
	}
	jwks := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": document})
	}))
	jwks.TLS = &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{serverCert}}
	jwks.StartTLS()
	t.Cleanup(jwks.Close)
	redis := miniredis.RunT(t)
	runtime, err := New(context.Background(), Config{Listen: "127.0.0.1:0", CertFile: certFile, KeyFile: keyFile, ClientCAFile: clientCAFile, JWKSURL: jwks.URL, JWKSCAFile: caFile, RedisAddr: redis.Addr()})
	require.NoError(t, err)
	t.Cleanup(func() { _ = runtime.Close() })
	// No legacy Redis epoch exists. A successful call must use the dedicated
	// callback; supplying the Redis epoch would hide an accidental fallback.
	var revoked, unavailable, replayUnavailable atomic.Bool
	var floorCalls atomic.Int32
	var mutex sync.Mutex
	seen := map[string]bool{}
	deps := CreatorDependencies{
		Epoch: func(context.Context, string, int64) error {
			floorCalls.Add(1)
			if unavailable.Load() {
				return ErrDependencyUnavailable
			}
			if revoked.Load() {
				return errors.New("revoked")
			}
			return nil
		},
		Replay: func(_ context.Context, issuer, jti string, _ time.Time) error {
			if replayUnavailable.Load() {
				return ErrDependencyUnavailable
			}
			mutex.Lock()
			defer mutex.Unlock()
			key := issuer + ":" + jti
			if seen[key] {
				return errors.New("replayed")
			}
			seen[key] = true
			return nil
		},
	}
	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM(serverCA.pem))
	start := func(deps CreatorDependencies) (spacev1.SpaceServiceClient, *creatorTransportServer) {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		service := &creatorTransportServer{}
		server := grpc.NewServer(runtime.ServerOptionsWithCreator(deps)...)
		spacev1.RegisterSpaceServiceServer(server, service)
		go func() { _ = server.Serve(listener) }()
		t.Cleanup(server.Stop)
		conn, err := grpc.NewClient(listener.Addr().String(), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: "localhost", Certificates: []tls.Certificate{clientCert}})))
		require.NoError(t, err)
		t.Cleanup(func() { _ = conn.Close() })
		return spacev1.NewSpaceServiceClient(conn), service
	}
	client, service := start(deps)
	request := &spacev1.CreateSpaceRequest{Name: "Signed creator"}
	profile := uuid.NewString()
	sign := func(kid string) context.Context {
		issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: kid, PrivateKey: keys[kid]})
		require.NoError(t, err)
		hash, err := principal.RequestHash(request)
		require.NoError(t, err)
		requestID := uuid.NewString()
		token, err := issuer.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "space", RPC: spacev1.SpaceService_CreateSpace_FullMethodName, RequestID: requestID, RequestHash: hash, AccountID: uuid.NewString(), ProfileID: profile, SessionEpoch: 7, AccountType: "regular", ClientExpiresAt: time.Now().Add(time.Minute)})
		require.NoError(t, err)
		return metadata.NewOutgoingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", requestID))
	}
	for _, kid := range []string{"current", "next"} {
		ctx := sign(kid)
		response, err := client.CreateSpace(ctx, request)
		require.NoError(t, err)
		require.Equal(t, profile, response.Space.OwnerProfileId)
		_, err = client.CreateSpace(ctx, request)
		require.Equal(t, codes.Unauthenticated, status.Code(err))
	}
	require.EqualValues(t, 4, floorCalls.Load(), "fresh ingress plus pre-write checks")
	_, err = client.CreateSpace(sign("current"), &spacev1.CreateSpaceRequest{Name: "Changed"})
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	_, err = client.CreateSpace(metadata.AppendToOutgoingContext(sign("current"), "x-voice-profile-id", profile), request)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	revoked.Store(true)
	_, err = client.CreateSpace(sign("current"), request)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	revoked.Store(false)
	unavailable.Store(true)
	_, err = client.CreateSpace(sign("current"), request)
	require.Equal(t, codes.Unavailable, status.Code(err))
	unavailable.Store(false)
	replayUnavailable.Store(true)
	_, err = client.CreateSpace(sign("current"), request)
	require.Equal(t, codes.Unavailable, status.Code(err))
	replayUnavailable.Store(false)
	require.EqualValues(t, 2, service.calls.Load())
	for _, missing := range []CreatorDependencies{{}, {Epoch: deps.Epoch}, {Replay: deps.Replay}} {
		closed, service := start(missing)
		_, err = closed.CreateSpace(sign("current"), request)
		require.Equal(t, codes.Unavailable, status.Code(err))
		require.Zero(t, service.calls.Load())
	}
}
