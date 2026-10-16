package voiceuserprincipalruntime

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	callsv1 "voice.app/voice/calls/v1"
	"voice/backend/pkg/principal"
	"voice/backend/voice/internal/principalgrpc"
)

type diagnosticEpochChecker struct{ err error }

func (e *diagnosticEpochChecker) RequireCurrent(context.Context, string, int64) error { return e.err }

func TestColdGoTLSFetchFailureIsDiagnosedAndWarmCacheDoesNotFetch(t *testing.T) {
	var logs bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(old) })
	keys := make([]map[string]string, 0, 2)
	var signing *rsa.PrivateKey
	for _, kid := range []string{"current", "next"} {
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)
		if kid == "current" {
			signing = key
		}
		keys = append(keys, map[string]string{"kid": kid, "kty": "RSA", "use": "sig", "alg": "RS256", "n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()), "e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes())})
	}
	var calls atomic.Int32
	endpoint := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			<-r.Context().Done()
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": keys})
	}))
	defer endpoint.Close()
	replay := miniredis.RunT(t)
	epochs := &diagnosticEpochChecker{}
	runtime, err := New(context.Background(), bootstrapConfig(t, endpoint, replay), epochs)
	require.NoError(t, err)
	defer runtime.Close()
	start := time.Now()
	_, err = runtime.resolver.Resolve(context.Background(), "gateway", "current")
	require.Error(t, err)
	require.Equal(t, codes.Unavailable, status.Code(principalgrpc.VerificationStatus(err)))
	require.Less(t, time.Since(start), 3*time.Second)
	require.Contains(t, logs.String(), `"stage":"jwks_request"`)
	require.Contains(t, logs.String(), `"cause":"timeout"`)
	require.NotContains(t, logs.String(), endpoint.URL)
	time.Sleep(2 * time.Millisecond)
	_, err = runtime.resolver.Resolve(context.Background(), "gateway", "current")
	require.NoError(t, err)
	require.EqualValues(t, 2, calls.Load())
	endpoint.Close()
	_, err = runtime.resolver.Resolve(context.Background(), "gateway", "current")
	require.NoError(t, err)
	require.EqualValues(t, 2, calls.Load(), "warm cache must not make a network request")
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "current", PrivateKey: signing})
	require.NoError(t, err)
	request := &callsv1.GetJoinTokenRequest{RoomId: "room"}
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	method := callsv1.VoiceService_GetJoinToken_FullMethodName
	token, err := issuer.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "voice", RPC: method, RequestID: "request-canary", RequestHash: hash, AccountID: "account-canary", ProfileID: "profile-canary", SessionEpoch: 7, ClientExpiresAt: time.Now().Add(time.Minute)})
	require.NoError(t, err)
	epochs.err = status.Error(codes.Unavailable, "private-auth-error-canary")
	logs.Reset()
	_, err = runtime.Verify(context.Background(), token, method, "request-canary", hash)
	require.Equal(t, codes.Unavailable, status.Code(err))
	require.Contains(t, logs.String(), `"stage":"auth_floor"`)
	for _, secret := range []string{token, "private-auth-error-canary", "account-canary", "request-canary"} {
		require.NotContains(t, logs.String(), secret)
	}
	replay.Close()
	logs.Reset()
	err = runtime.recordReplay(context.Background(), "gateway", "secret-jti-canary", time.Now().Add(time.Second))
	require.Equal(t, codes.Unavailable, status.Code(principalgrpc.VerificationStatus(err)))
	require.Contains(t, logs.String(), `"stage":"replay_write"`)
	require.NotContains(t, logs.String(), "secret-jti-canary")
}
