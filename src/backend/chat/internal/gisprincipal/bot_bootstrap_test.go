package gisprincipal

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/pkg/principal"
)

func TestBotScopeDeferredJWKSBootstrapFailsClosedThenRecovers(t *testing.T) {
	ca := newManifestTestCA(t, 87501)
	certificate, certFile, keyFile := ca.leaf(t, 87502, x509.ExtKeyUsageServerAuth)
	_, clientCert, clientKey := ca.leaf(t, 87503, x509.ExtKeyUsageClientAuth)
	caFile := filepath.Join(t.TempDir(), "ca.crt")
	require.NoError(t, os.WriteFile(caFile, ca.pem, 0600))
	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM(ca.pem))
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	next, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	var ready atomic.Bool
	var fetches atomic.Int64
	publisher := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fetches.Add(1)
		if !ready.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		principal.JWKSHandlerKeys([]principal.JWKSKey{{KeyID: "current", PublicKey: &key.PublicKey}, {KeyID: "next", PublicKey: &next.PublicKey}}).ServeHTTP(w, r)
	}))
	publisher.TLS = &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{certificate}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: roots}
	publisher.StartTLS()
	defer publisher.Close()
	replay := miniredis.RunT(t)
	cfg := Config{Issuer: "bot", Audience: Audience, AllowedMethods: []string{BotScopeAuthorityMethod}, DeferInitialJWKS: true, TLSCertFile: certFile, TLSKeyFile: keyFile, ClientCAFile: caFile, JWKSURL: publisher.URL, JWKSCAFile: caFile, JWKSClientCertFile: clientCert, JWKSClientKeyFile: clientKey, ReplayAddr: replay.Addr(), RefreshAfter: time.Hour, HardExpiry: 2 * time.Hour}
	runtime, err := New(t.Context(), cfg)
	require.NoError(t, err, "Bot publisher may start after healthy Chat")
	defer runtime.Close()
	require.Zero(t, fetches.Load(), "constructor must not contact Bot publisher")
	req := &chatv1.GetBotChatScopeAuthorityRequest{ProtocolVersion: 1, OperationId: uuid.NewString(), ChatId: uuid.NewString(), ExpectedSpaceId: uuid.NewString()}
	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	signer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "bot", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	token, err := signer.IssueService(principal.ServiceInput{Audience: Audience, RPC: BotScopeAuthorityMethod, RequestID: req.OperationId, RequestHash: hash})
	require.NoError(t, err)
	calls := 0
	invoke := func() error {
		ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", req.OperationId))
		_, err := (strictInterceptor{runtime: runtime}).intercept(ctx, req, &grpc.UnaryServerInfo{FullMethod: BotScopeAuthorityMethod}, func(context.Context, any) (any, error) { calls++; return nil, nil })
		return err
	}
	require.Error(t, invoke())
	require.Zero(t, calls)
	require.Positive(t, fetches.Load())
	ready.Store(true)
	require.Eventually(t, func() bool { return invoke() == nil }, 7*time.Second, 20*time.Millisecond)
	require.Equal(t, 1, calls)
	require.Error(t, invoke(), "recovered verifier still rejects JTI replay")
	require.Equal(t, 1, calls)
	ready.Store(false)
	eager := cfg
	eager.Issuer = Issuer
	eager.AllowedMethods = []string{ProvisionMethod}
	eager.DeferInitialJWKS = false
	denied, err := New(t.Context(), eager)
	if denied != nil {
		_ = denied.Close()
	}
	require.Error(t, err, "default GIS startup must retain eager readiness")
	for _, invalid := range []Config{
		func() Config { c := eager; c.DeferInitialJWKS = true; return c }(),
		func() Config {
			c := cfg
			c.AllowedMethods = []string{BotScopeAuthorityMethod, BotScopeAuthorityMethod}
			return c
		}(),
		func() Config { c := cfg; c.Audience = "space"; return c }(),
	} {
		r, err := New(t.Context(), invalid)
		if r != nil {
			_ = r.Close()
		}
		require.Error(t, err, "deferred mode is exclusive to exact Bot scope receiver")
	}
}
