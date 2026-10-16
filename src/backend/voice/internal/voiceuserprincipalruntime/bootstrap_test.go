package voiceuserprincipalruntime

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"math/big"
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
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	callsv1 "voice.app/voice/calls/v1"
	"voice/backend/pkg/principal"
)

type bootstrapEpochChecker struct {
	calls     int
	accountID string
	epoch     int64
}

func (c *bootstrapEpochChecker) RequireCurrent(_ context.Context, accountID string, epoch int64) error {
	c.calls++
	c.accountID, c.epoch = accountID, epoch
	return nil
}

func bootstrapConfig(t *testing.T, endpoint *httptest.Server, replay *miniredis.Miniredis) Config {
	t.Helper()
	dir := t.TempDir()
	certPath, keyPath := filepath.Join(dir, "principal.crt"), filepath.Join(dir, "principal.key")
	cert := endpoint.TLS.Certificates[0]
	keyDER, err := x509.MarshalPKCS8PrivateKey(cert.PrivateKey)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Certificate[0]}), 0o600))
	require.NoError(t, os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}), 0o600))
	return Config{
		JWKSURL: endpoint.URL, JWKSCAFile: certPath, JWKSClientCertFile: certPath, JWKSClientKeyFile: keyPath,
		ReplayAddr: replay.Addr(), TLSCertFile: certPath, TLSKeyFile: keyPath, ClientCAFile: certPath,
		ListenAddr: ":0", RefreshAfter: time.Hour, HardExpiry: 2 * time.Hour, UnknownKIDCooldown: time.Millisecond,
	}
}

func TestRuntimeBootstrapsBeforeGatewayJWKSAndRejectsRequestsUntilKeysAreValid(t *testing.T) {
	current, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	next, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	jwk := func(kid string, key *rsa.PrivateKey) map[string]string {
		return map[string]string{
			"kid": kid, "kty": "RSA", "use": "sig", "alg": "RS256",
			"n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
			"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
		}
	}
	var endpointState, fetches atomic.Int32
	endpoint := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fetches.Add(1)
		switch endpointState.Load() {
		case 0:
			w.WriteHeader(http.StatusServiceUnavailable)
		case 1:
			// An incomplete rotation set must not admit a request either.
			_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{jwk("current", current)}})
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{jwk("current", current), jwk("next", next)}})
		}
	}))
	t.Cleanup(endpoint.Close)
	replay := miniredis.RunT(t)
	epochs := &bootstrapEpochChecker{}
	runtime, err := New(context.Background(), bootstrapConfig(t, endpoint, replay), epochs)
	require.NoError(t, err, "Gateway waits for this listener before it can serve JWKS")
	t.Cleanup(func() { require.NoError(t, runtime.Close()) })
	require.Zero(t, fetches.Load(), "startup must not wait on the Gateway it unblocks")

	request := &callsv1.JoinVoiceRoomRequest{VoiceRoomId: "room"}
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	method := callsv1.VoiceService_JoinVoiceRoom_FullMethodName
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "current", PrivateKey: current})
	require.NoError(t, err)
	token, err := issuer.IssueDelegatedUser(principal.DelegatedUserInput{
		Audience: "voice", RPC: method, RequestID: "bootstrap-request", RequestHash: hash,
		AccountID: "account-1", ProfileID: "profile-1", SessionEpoch: 7, ClientExpiresAt: time.Now().Add(time.Minute),
	})
	require.NoError(t, err)
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", "bootstrap-request"))
	handlerCalls := 0
	var accepted principal.Principal
	var hasPrincipal bool
	invoke := func() error {
		_, invokeErr := StrictUnaryInterceptor(runtime)(ctx, request, &grpc.UnaryServerInfo{FullMethod: method},
			func(ctx context.Context, _ any) (any, error) {
				handlerCalls++
				accepted, hasPrincipal = principal.FromContext(ctx)
				return &callsv1.JoinVoiceRoomResponse{}, nil
			})
		return invokeErr
	}
	require.Equal(t, codes.Unavailable, status.Code(invoke()))
	require.Zero(t, handlerCalls)
	require.Zero(t, epochs.calls)
	require.Empty(t, replay.Keys())

	endpointState.Store(1)
	require.Eventually(t, func() bool { return status.Code(invoke()) == codes.Unauthenticated }, time.Second, time.Millisecond)
	require.Zero(t, handlerCalls)
	require.Zero(t, epochs.calls)
	require.Empty(t, replay.Keys())

	endpointState.Store(2)
	require.Eventually(t, func() bool { return invoke() == nil }, time.Second, time.Millisecond)
	require.Equal(t, 1, handlerCalls)
	require.True(t, hasPrincipal)
	require.Equal(t, "profile-1", accepted.ProfileID)
	require.Equal(t, 1, epochs.calls)
	require.Equal(t, "account-1", epochs.accountID)
	require.EqualValues(t, 7, epochs.epoch)
	require.Len(t, replay.Keys(), 1)
	require.Equal(t, codes.Unauthenticated, status.Code(invoke()), "the successful request is still single-use")
	require.Equal(t, 1, handlerCalls)
}

func TestRuntimeBootstrapStillRequiresLocalCredentialsAndReplayStore(t *testing.T) {
	endpoint := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(endpoint.Close)
	replay := miniredis.RunT(t)
	config := bootstrapConfig(t, endpoint, replay)
	badTLS := config
	badTLS.TLSKeyFile = filepath.Join(t.TempDir(), "missing.key")
	runtime, err := New(context.Background(), badTLS, &bootstrapEpochChecker{})
	require.Error(t, err)
	require.Nil(t, runtime)

	replay.Close()
	runtime, err = New(context.Background(), config, &bootstrapEpochChecker{})
	require.Error(t, err)
	require.Nil(t, runtime)
}
