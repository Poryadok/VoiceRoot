package principalruntime

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/principal"
)

func startFixtureRuntime(t *testing.T, f runtimeFixture) *Runtime {
	t.Helper()
	runtime, err := New(context.Background(), f.config)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, runtime.Close()) })
	// Warm through the public verifier, not New: issuer availability is not a
	// startup precondition because Space itself waits for Role health.
	method := rolev1.RoleService_ApplyOwnershipTransfer_FullMethodName
	token := issueRuntimeToken(t, f.key, "space", "current", "role", method, "fixture-prime", runtimeHash)
	_, err = runtime.Verify(context.Background(), token, method, "fixture-prime", runtimeHash)
	require.NoError(t, err)
	return runtime
}

func TestRuntimeReplaySharedAcrossInstancesExpiresAtSignedExpiry(t *testing.T) {
	f := newRuntimeFixture(t, 2)
	first := startFixtureRuntime(t, f)
	second := startFixtureRuntime(t, f)
	method := rolev1.RoleService_ApplyOwnershipTransfer_FullMethodName
	token := issueRuntimeToken(t, f.key, "space", "current", "role", method, "shared-request", runtimeHash)
	priorKeys := make(map[string]bool)
	for _, key := range f.redis.Keys() {
		priorKeys[key] = true
	}
	before := time.Now()
	verified, err := first.Verify(context.Background(), token, method, "shared-request", runtimeHash)
	require.NoError(t, err)
	after := time.Now()
	_, err = second.Verify(context.Background(), token, method, "shared-request", runtimeHash)
	require.Error(t, err, "a separate Role instance must share the replay decision")
	var keys []string
	for _, key := range f.redis.Keys() {
		if !priorKeys[key] {
			keys = append(keys, key)
		}
	}
	require.Len(t, keys, 1)
	ttl := f.redis.TTL(keys[0])
	require.Greater(t, ttl, time.Duration(0))
	// Replay retention follows the consumer clock and rounds TTL up to a millisecond.
	// Allow scheduling plus millisecond rounding, independent of the Redis wall clock.
	require.GreaterOrEqual(t, ttl, verified.ExpiresAt.Sub(after)-100*time.Millisecond)
	require.LessOrEqual(t, ttl, verified.ExpiresAt.Sub(before)+100*time.Millisecond)
	f.redis.FastForward(ttl + time.Millisecond)
	require.False(t, f.redis.Exists(keys[0]), "replay state must not outlive the signed expiry")
}

func TestRuntimeRefreshesCompleteJWKSWithoutIncomingTraffic(t *testing.T) {
	f := newRuntimeFixture(t, 2)
	f.config.RefreshAfter = 100 * time.Millisecond
	f.config.HardExpiry = 2 * time.Second
	runtime := startFixtureRuntime(t, f)
	// Swap published key material to distinguish replacement from a no-op fetch.
	f.jwks.set(http.StatusOK, runtimeJWKSDocument(t, map[string]*rsa.PrivateKey{"current": f.next, "next": f.key}))
	baseline := f.jwks.calls.Load()
	require.Eventually(t, func() bool { return f.jwks.calls.Load() >= baseline+2 }, time.Second, 10*time.Millisecond,
		"JWKS must refresh on schedule without a Verify call")
	// A failing endpoint now forces verification to use the refreshed last-good set.
	f.jwks.set(http.StatusServiceUnavailable, nil)
	method := rolev1.RoleService_ApplyOwnershipTransfer_FullMethodName
	token := issueRuntimeToken(t, f.next, "space", "current", "role", method, "rotated-request", runtimeHash)
	_, err := runtime.Verify(context.Background(), token, method, "rotated-request", runtimeHash)
	require.NoError(t, err, "complete background refresh must replace cached key material")
}

func TestRuntimeInvalidRefreshRetainsLastGoodOnlyUntilHardExpiry(t *testing.T) {
	for _, name := range []string{"incomplete", "malformed", "unavailable"} {
		t.Run(name, func(t *testing.T) {
			f := newRuntimeFixture(t, 2)
			f.config.RefreshAfter = 100 * time.Millisecond
			f.config.HardExpiry = 2 * time.Second
			runtime := startFixtureRuntime(t, f)
			started := time.Now()
			switch name {
			case "incomplete":
				f.jwks.set(http.StatusOK, runtimeJWKSDocument(t, map[string]*rsa.PrivateKey{"current": f.next}))
			case "malformed":
				f.jwks.set(http.StatusOK, []byte(`{"keys":`))
			case "unavailable":
				f.jwks.set(http.StatusServiceUnavailable, nil)
			}
			method := rolev1.RoleService_ApplyOwnershipTransfer_FullMethodName
			// Wait past soft expiry, leaving a wide interval before the hard deadline.
			timer := time.NewTimer(200 * time.Millisecond)
			<-timer.C
			token := issueRuntimeToken(t, f.key, "space", "current", "role", method, "last-good", runtimeHash)
			_, err := runtime.Verify(context.Background(), token, method, "last-good", runtimeHash)
			require.NoError(t, err, "invalid refresh must preserve the old complete set before hard expiry")
			require.GreaterOrEqual(t, f.jwks.calls.Load(), int64(2), "an actual refresh failure must precede last-good assertion")
			remaining := time.Until(started.Add(f.config.HardExpiry + 100*time.Millisecond))
			if remaining > 0 {
				timer = time.NewTimer(remaining)
				<-timer.C
			}
			token = issueRuntimeToken(t, f.key, "space", "current", "role", method, "after-hard-expiry", runtimeHash)
			_, err = runtime.Verify(context.Background(), token, method, "after-hard-expiry", runtimeHash)
			require.Error(t, err, "failed refresh must never extend last-good beyond hard expiry")
		})
	}
}

func TestRuntimeUnknownKIDStormHasIssuerWideCooldown(t *testing.T) {
	f := newRuntimeFixture(t, 2)
	runtime := startFixtureRuntime(t, f)
	method := rolev1.RoleService_ApplyOwnershipTransfer_FullMethodName
	baseline := f.jwks.calls.Load()
	for i := 0; i < 20; i++ {
		requestID := fmt.Sprintf("storm-%d", i)
		token := issueRuntimeToken(t, f.key, "space", fmt.Sprintf("unknown-%d", i), "role", method, requestID, runtimeHash)
		_, err := runtime.Verify(context.Background(), token, method, requestID, runtimeHash)
		require.Error(t, err)
	}
	require.LessOrEqual(t, f.jwks.calls.Load()-baseline, int64(1), "untrusted kid cardinality must not amplify JWKS traffic")
}

func TestRuntimeRejectsInvalidSignatureAndServiceUserAuthority(t *testing.T) {
	f := newRuntimeFixture(t, 2)
	runtime := startFixtureRuntime(t, f)
	method := rolev1.RoleService_ApplyOwnershipTransfer_FullMethodName
	for _, name := range []string{"wrong_signature", "account_id", "profile_id", "session_epoch"} {
		t.Run(name, func(t *testing.T) {
			token := issueRuntimeToken(t, f.key, "space", "current", "role", method, name, runtimeHash)
			if name == "wrong_signature" {
				// The kid names current while the signature is made by the distinct next key.
				token = issueRuntimeToken(t, f.next, "space", "current", "role", method, name, runtimeHash)
			} else {
				token = withSignedRuntimeClaim(t, token, f.key, name)
			}
			got, err := runtime.Verify(context.Background(), token, method, name, runtimeHash)
			require.Error(t, err)
			require.Empty(t, got.Subject, "rejected credentials must not expose service authority")
			require.Empty(t, got.AccountID)
			require.Empty(t, got.ProfileID)
			require.Zero(t, got.SessionEpoch)
		})
	}
}

func withSignedRuntimeClaim(t *testing.T, token string, key *rsa.PrivateKey, claim string) string {
	t.Helper()
	parts := strings.Split(token, ".")
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	require.NoError(t, err)
	var claims map[string]any
	require.NoError(t, json.Unmarshal(payload, &claims))
	if claim == "session_epoch" {
		claims[claim] = 1
	} else {
		claims[claim] = "injected-user"
	}
	payload, err = json.Marshal(claims)
	require.NoError(t, err)
	unsigned := parts[0] + "." + base64.RawURLEncoding.EncodeToString(payload)
	digest := sha256.Sum256([]byte(unsigned))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	require.NoError(t, err)
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(signature)
}

func TestRuntimeCloseStopsScheduledRefreshAndIsRepeatable(t *testing.T) {
	f := newRuntimeFixture(t, 2)
	f.config.RefreshAfter = 50 * time.Millisecond
	f.config.HardExpiry = time.Second
	runtime := startFixtureRuntime(t, f)
	baseline := f.jwks.calls.Load()
	require.Eventually(t, func() bool { return f.jwks.calls.Load() >= baseline+2 }, time.Second, 10*time.Millisecond)
	closed := make(chan error, 1)
	go func() { closed <- runtime.Close() }()
	select {
	case err := <-closed:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("runtime Close did not finish")
	}
	afterClose := f.jwks.calls.Load()
	require.Never(t, func() bool { return f.jwks.calls.Load() != afterClose }, 250*time.Millisecond, 10*time.Millisecond,
		"Close must stop scheduled network refreshes before returning")
	require.NoError(t, runtime.Close(), "repeated Close must be safe")
}

// Accepted correction: Redis absolute expiration uses Redis's wall clock. If
// Redis is ahead of the verifier, EXAT drops replay state while the JWT remains
// valid to the consumer. Relative TTL rounded up must preserve the full interval.
func TestRuntimeReplaySurvivesRedisClockAheadOfVerifier(t *testing.T) {
	f := newRuntimeFixture(t, 2)
	f.redis.SetTime(time.Now().Add(10 * time.Second))
	first := startFixtureRuntime(t, f)
	second := startFixtureRuntime(t, f)
	method := rolev1.RoleService_ApplyOwnershipTransfer_FullMethodName
	token := issueRuntimeToken(t, f.key, "space", "current", "role", method, "redis-clock-ahead", runtimeHash)
	verified, err := first.Verify(context.Background(), token, method, "redis-clock-ahead", runtimeHash)
	require.NoError(t, err)
	advance := time.Until(verified.ExpiresAt) - 5*time.Second
	require.Greater(t, advance, time.Duration(0))
	f.redis.FastForward(advance)
	// Only Redis expiration time advances; the actual JWT remains valid to both
	// consumers. Shared replay state must therefore still reject the second use.
	require.True(t, time.Now().Before(verified.ExpiresAt))
	_, err = second.Verify(context.Background(), token, method, "redis-clock-ahead", runtimeHash)
	require.Error(t, err, "Redis clock lead must not permit replay of an unexpired JWT")
}

func TestRuntimeRecordReplayRejectsExpiryOutsideBound(t *testing.T) {
	for _, tc := range []struct {
		name   string
		offset time.Duration
	}{
		{"expired", -time.Second},
		{"beyond_max_lifetime_and_skew", 36 * time.Second},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newRuntimeFixture(t, 2)
			runtime := startFixtureRuntime(t, f)
			before := f.redis.Keys()
			err := runtime.recordReplay(context.Background(), "space", tc.name, time.Now().Add(tc.offset))
			require.Error(t, err)
			require.ElementsMatch(t, before, f.redis.Keys(), "invalid retention bounds must not write replay state")
		})
	}
}

func TestRuntimeAllowsCanonicalLifetimeWithFutureIssuedAtSkew(t *testing.T) {
	f := newRuntimeFixture(t, 2)
	runtime := startFixtureRuntime(t, f)
	signer, err := principal.NewIssuer(principal.IssuerConfig{
		Issuer: "space", KeyID: "current", PrivateKey: f.key,
		Clock: func() time.Time { return time.Now().Add(5 * time.Second) },
	})
	require.NoError(t, err)
	method := rolev1.RoleService_ApplyOwnershipTransfer_FullMethodName
	token, err := signer.IssueService(principal.ServiceInput{Audience: "role", RPC: method, RequestID: "future-skew", RequestHash: runtimeHash})
	require.NoError(t, err)
	before := make(map[string]bool)
	for _, key := range f.redis.Keys() {
		before[key] = true
	}
	verified, err := runtime.Verify(context.Background(), token, method, "future-skew", runtimeHash)
	require.NoError(t, err, "30s JWT lifetime with allowed 5s future skew remains valid")
	var added []string
	for _, key := range f.redis.Keys() {
		if !before[key] {
			added = append(added, key)
		}
	}
	require.Len(t, added, 1)
	ttl := f.redis.TTL(added[0])
	require.Greater(t, ttl, 30*time.Second)
	require.LessOrEqual(t, ttl, 35*time.Second+time.Millisecond)
	require.GreaterOrEqual(t, ttl, time.Until(verified.ExpiresAt)-100*time.Millisecond)
	_, err = runtime.Verify(context.Background(), token, method, "future-skew", runtimeHash)
	require.Error(t, err, "allowed future skew must retain shared replay protection")
}
