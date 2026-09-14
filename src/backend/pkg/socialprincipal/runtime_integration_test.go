package socialprincipal

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"voice/backend/pkg/principal"
)

// Hosted integration: real Redis shared by independent receiver runtimes,
// HTTPS JWKS and ephemeral TLS. Never execute locally under the fleet restriction.
func TestRuntimeTLSJWKSRedisIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("hosted TLS/JWKS/Redis integration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{ContainerRequest: testcontainers.ContainerRequest{Image: "redis:7-alpine", ExposedPorts: []string{"6379/tcp"}, WaitingFor: wait.ForListeningPort("6379/tcp")}, Started: true})
	require.NoError(t, err)
	t.Cleanup(func() { _ = container.Terminate(context.Background()) })
	host, err := container.Host(ctx)
	require.NoError(t, err)
	port, err := container.MappedPort(ctx, "6379/tcp")
	require.NoError(t, err)
	cert, certPEM, keyPEM := ephemeralTLS(t)
	dir := t.TempDir()
	certPath, keyPath := filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem")
	require.NoError(t, os.WriteFile(certPath, certPEM, 0600))
	require.NoError(t, os.WriteFile(keyPath, keyPEM, 0600))
	first, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	next, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	doc := jwksDocument(t, map[string]*rsa.PublicKey{"current": &first.PublicKey, "next": &next.PublicKey})
	var unavailable atomic.Bool
	jwks := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if unavailable.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(doc)
	}))
	jwks.TLS = &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{cert}}
	jwks.StartTLS()
	defer jwks.Close()
	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM(certPEM))
	for _, target := range []string{"user", "space"} {
		t.Run(target, func(t *testing.T) {
			cfg := Config{Target: target, JWKSURLs: map[string]string{"social": jwks.URL}, JWKSCAFile: certPath, RefreshAfter: 30 * time.Second, HardExpiry: 2 * time.Minute, UnknownKIDCooldown: 5 * time.Second, ReplayAddr: net.JoinHostPort(host, port.Port()), TLSCertFile: certPath, TLSKeyFile: keyPath, ListenAddr: ":9091"}
			a, err := New(ctx, cfg)
			require.NoError(t, err)
			defer func() { _ = a.Close() }()
			b, err := New(ctx, cfg)
			require.NoError(t, err)
			defer func() { _ = b.Close() }()
			var callsA, callsB, interceptA, interceptB atomic.Int32
			connA := testPrivacyConnection(t, a, roots, &callsA, &interceptA)
			connB := testPrivacyConnection(t, b, roots, &callsB, &interceptB)
			req := wrapperspb.String("profile")
			hash, err := principal.RequestHash(req)
			require.NoError(t, err)
			issue := func(kid string, key *rsa.PrivateKey) context.Context {
				issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "social", KeyID: kid, PrivateKey: key})
				require.NoError(t, err)
				token, err := issuer.IssueService(principal.ServiceInput{Audience: target, RPC: Method(target), RequestID: "request", RequestHash: hash})
				require.NoError(t, err)
				return metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", "request"))
			}
			callCtx := issue("current", first)
			require.NoError(t, connA.Invoke(callCtx, Method(target), req, new(wrapperspb.StringValue)))
			err = connB.Invoke(callCtx, Method(target), req, new(wrapperspb.StringValue))
			require.Equal(t, codes.Unauthenticated, status.Code(err))
			require.EqualValues(t, 1, callsA.Load())
			require.Zero(t, callsB.Load())
			// Cached next key remains usable while the HTTPS publisher is unavailable.
			unavailable.Store(true)
			require.NoError(t, connB.Invoke(issue("next", next), Method(target), req, new(wrapperspb.StringValue)))
			unavailable.Store(false)
			// Replay keys are receiver-scoped and written with a bounded Redis TTL.
			keys, err := a.replay.Keys(ctx, target+":principal:replay:*").Result()
			require.NoError(t, err)
			require.Len(t, keys, 2)
			for _, key := range keys {
				ttl, err := a.replay.PTTL(ctx, key).Result()
				require.NoError(t, err)
				require.Positive(t, ttl)
				require.LessOrEqual(t, ttl, 35*time.Second)
			}
			var wrongCalls, wrongInterceptions atomic.Int32
			wrong := testPrivacyConnection(t, a, x509.NewCertPool(), &wrongCalls, &wrongInterceptions)
			short, cancel := context.WithTimeout(issue("current", first), time.Second)
			defer cancel()
			err = wrong.Invoke(short, Method(target), req, new(wrapperspb.StringValue))
			require.Error(t, err)
			require.Zero(t, wrongCalls.Load())
			require.Zero(t, wrongInterceptions.Load())
		})
	}
}
