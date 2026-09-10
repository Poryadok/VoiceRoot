package principalruntime

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
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/require"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/principal"
)

const runtimeHash = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

// Real HTTPS, RSA signatures, and the Redis wire protocol exercise the runtime's
// dependency wiring without replacing verifier or replay behavior with mocks.
type runtimeFixture struct {
	config Config
	redis  *miniredis.Miniredis
	key    *rsa.PrivateKey
	next   *rsa.PrivateKey
	jwks   *runtimeJWKS
}

func newRuntimeFixture(t *testing.T, keyCount int) runtimeFixture {
	t.Helper()
	current, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	next, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	jwksKey := func(kid string, key *rsa.PrivateKey) map[string]string {
		return map[string]string{"kid": kid, "kty": "RSA", "use": "sig", "alg": "RS256",
			"n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
			"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes())}
	}
	keys := []map[string]string{jwksKey("current", current), jwksKey("next", next)}
	body, err := json.Marshal(map[string]any{"keys": keys[:keyCount]})
	require.NoError(t, err)
	jwks := &runtimeJWKS{}
	jwks.set(http.StatusOK, body)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		response := jwks.response.Load()
		jwks.calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(response.status)
		_, _ = w.Write(response.body)
	}))
	t.Cleanup(server.Close)
	dir := t.TempDir()
	certFile := filepath.Join(dir, "tls.crt")
	keyFile := filepath.Join(dir, "tls.key")
	require.NoError(t, os.WriteFile(certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.TLS.Certificates[0].Certificate[0]}), 0600))
	keyDER, err := x509.MarshalPKCS8PrivateKey(server.TLS.Certificates[0].PrivateKey)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}), 0600))
	redis := miniredis.RunT(t)
	return runtimeFixture{config: Config{
		JWKSURLs: map[string]string{"space": server.URL}, RefreshAfter: 30 * time.Second, HardExpiry: 2 * time.Minute, UnknownKIDCooldown: 5 * time.Second,
		ReplayAddr: redis.Addr(), JWKSCAFile: certFile, TLSCertFile: certFile, TLSKeyFile: keyFile, ListenAddr: "127.0.0.1:0",
	}, redis: redis, key: current, next: next, jwks: jwks}
}

func issueRuntimeToken(t *testing.T, key *rsa.PrivateKey, issuer, kid, audience, method, requestID, hash string) string {
	t.Helper()
	signer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: issuer, KeyID: kid, PrivateKey: key})
	require.NoError(t, err)
	token, err := signer.IssueService(principal.ServiceInput{Audience: audience, RPC: method, RequestID: requestID, RequestHash: hash})
	require.NoError(t, err)
	return token
}

func TestRuntimeVerifiesBothRotationKeysAndRejectsReplay(t *testing.T) {
	f := newRuntimeFixture(t, 2)
	runtime, err := New(context.Background(), f.config)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, runtime.Close()) })
	for _, tc := range []struct {
		kid, method string
		key         *rsa.PrivateKey
	}{
		{"current", rolev1.RoleService_PrepareOwnershipTransfer_FullMethodName, f.key},
		{"next", rolev1.RoleService_AbortOwnershipTransfer_FullMethodName, f.next},
	} {
		t.Run(tc.kid, func(t *testing.T) {
			token := issueRuntimeToken(t, tc.key, "space", tc.kid, "role", tc.method, "request-"+tc.kid, runtimeHash)
			got, err := runtime.Verify(context.Background(), token, tc.method, "request-"+tc.kid, runtimeHash)
			require.NoError(t, err)
			require.Equal(t, "service:space", got.Subject)
			require.Empty(t, got.AccountID)
			require.Empty(t, got.ProfileID)
			require.Zero(t, got.SessionEpoch)
			require.Equal(t, "space", got.Issuer)
			require.Equal(t, "role", got.Audience)
			require.Equal(t, tc.method, got.RPC)
			_, err = runtime.Verify(context.Background(), token, tc.method, "request-"+tc.kid, runtimeHash)
			require.Error(t, err, "a JWT must not be accepted twice")
		})
	}
}

func TestRuntimeOwnershipV2ActivationRequiresCompleteDrainAndRetiredSpaceFence(t *testing.T) {
	f := newRuntimeFixture(t, 2)
	runtime, err := New(context.Background(), f.config)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, runtime.Close()) })
	complete := OwnershipV2Activation{
		V1Drained: true, RetiredSpaceFence: true,
		SupportedMethods: []string{
			rolev1.RoleService_PrepareOwnershipTransfer_FullMethodName,
			rolev1.RoleService_FinalizeOwnershipTransfer_FullMethodName,
			rolev1.RoleService_AbortOwnershipTransfer_FullMethodName,
		},
	}
	for _, tc := range []struct {
		name   string
		mutate func(*OwnershipV2Activation)
	}{
		{"v1_not_drained", func(g *OwnershipV2Activation) { g.V1Drained = false }},
		{"retired_space_fence_missing", func(g *OwnershipV2Activation) { g.RetiredSpaceFence = false }},
		{"method_missing", func(g *OwnershipV2Activation) { g.SupportedMethods = g.SupportedMethods[:2] }},
		{"legacy_method_present", func(g *OwnershipV2Activation) {
			g.SupportedMethods = append(g.SupportedMethods, rolev1.RoleService_ApplyOwnershipTransfer_FullMethodName)
		}},
		{"unrelated_method_present", func(g *OwnershipV2Activation) {
			g.SupportedMethods = append(g.SupportedMethods, rolev1.RoleService_ListRoles_FullMethodName)
		}},
		{"duplicate_method", func(g *OwnershipV2Activation) {
			g.SupportedMethods = append(g.SupportedMethods, rolev1.RoleService_PrepareOwnershipTransfer_FullMethodName)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gate := complete
			gate.SupportedMethods = append([]string(nil), complete.SupportedMethods...)
			tc.mutate(&gate)
			require.Error(t, runtime.ActivateOwnershipV2Capabilities(gate))
			require.False(t, runtime.ownershipV2CapabilitiesActive.Load())
		})
	}
	require.NoError(t, runtime.ActivateOwnershipV2Capabilities(complete))
	require.True(t, runtime.ownershipV2CapabilitiesActive.Load())
}

func TestRuntimeRejectsWrongIdentityBindingAndUnknownKey(t *testing.T) {
	f := newRuntimeFixture(t, 2)
	runtime, err := New(context.Background(), f.config)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, runtime.Close()) })
	method := rolev1.RoleService_PrepareOwnershipTransfer_FullMethodName
	for _, tc := range []struct{ name, issuer, kid, audience, rpc, requestID, hash string }{
		{"issuer", "gateway", "current", "role", method, "request", runtimeHash},
		{"audience", "space", "current", "chat", method, "request", runtimeHash},
		{"rpc", "space", "current", "role", rolev1.RoleService_AbortOwnershipTransfer_FullMethodName, "request", runtimeHash},
		{"request_id", "space", "current", "role", method, "different", runtimeHash},
		{"hash", "space", "current", "role", method, "request", "sha256:" + strings.Repeat("b", 64)},
		{"unknown_kid", "space", "unpublished", "role", method, "request", runtimeHash},
	} {
		t.Run(tc.name, func(t *testing.T) {
			token := issueRuntimeToken(t, f.key, tc.issuer, tc.kid, tc.audience, tc.rpc, tc.requestID, tc.hash)
			_, err := runtime.Verify(context.Background(), token, method, "request", runtimeHash)
			require.Error(t, err)
		})
	}
}

func TestRuntimeFailsClosedWhenReplayRedisStops(t *testing.T) {
	f := newRuntimeFixture(t, 2)
	runtime, err := New(context.Background(), f.config)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, runtime.Close()) })
	f.redis.Close()
	method := rolev1.RoleService_PrepareOwnershipTransfer_FullMethodName
	token := issueRuntimeToken(t, f.key, "space", "current", "role", method, "request", runtimeHash)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err = runtime.Verify(ctx, token, method, "request", runtimeHash)
	require.Error(t, err)
}

// Accepted plan correction: Space waits for Role health in Compose. Requiring
// Space JWKS availability during Role startup creates a boot dependency cycle.
// Phase-0 requires trusted complete JWKS before credential acceptance, while
// malformed local configuration and unavailable replay Redis remain startup errors.
func TestRuntimeRejectsUnsafeConfigAndFailsClosedUntilJWKSIsTrusted(t *testing.T) {
	for _, name := range []string{"one_key", "plaintext", "untrusted_tls", "redis_unavailable"} {
		t.Run(name, func(t *testing.T) {
			keyCount := 2
			if name == "one_key" {
				keyCount = 1
			}
			f := newRuntimeFixture(t, keyCount)
			switch name {
			case "plaintext":
				plainServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					keys := make([]map[string]string, 0, 2)
					for kid, key := range map[string]*rsa.PrivateKey{"current": f.key, "next": f.next} {
						keys = append(keys, map[string]string{"kid": kid, "kty": "RSA", "use": "sig", "alg": "RS256", "n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()), "e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes())})
					}
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode(map[string]any{"keys": keys})
				}))
				t.Cleanup(plainServer.Close)
				f.config.JWKSURLs["space"] = plainServer.URL
			case "untrusted_tls":
				f.config.JWKSCAFile = ""
			case "redis_unavailable":
				f.redis.Close()
			}
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			runtime, err := New(ctx, f.config)
			if runtime != nil {
				t.Cleanup(func() { _ = runtime.Close() })
			}
			if name == "plaintext" || name == "redis_unavailable" {
				require.Error(t, err)
				return
			}
			require.NoError(t, err, "remote JWKS readiness must not block Role startup")
			method := rolev1.RoleService_PrepareOwnershipTransfer_FullMethodName
			token := issueRuntimeToken(t, f.key, "space", "current", "role", method, "untrusted", runtimeHash)
			got, err := runtime.Verify(ctx, token, method, "untrusted", runtimeHash)
			require.Error(t, err)
			require.Empty(t, got.Subject)
		})
	}
}

func TestRuntimeStartupRejectsPartialConfig(t *testing.T) {
	f := newRuntimeFixture(t, 2)
	cases := []struct {
		name       string
		invalidate func(*Config)
	}{
		{"missing_space", func(c *Config) { c.JWKSURLs = map[string]string{"gateway": "https://gateway.internal/jwks"} }},
		{"missing_replay", func(c *Config) { c.ReplayAddr = "" }},
		{"missing_tls_cert", func(c *Config) { c.TLSCertFile = "" }},
		{"missing_tls_key", func(c *Config) { c.TLSKeyFile = "" }},
		{"zero_refresh", func(c *Config) { c.RefreshAfter = 0 }},
		{"zero_hard_expiry", func(c *Config) { c.HardExpiry = 0 }},
		{"zero_cooldown", func(c *Config) { c.UnknownKIDCooldown = 0 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := f.config
			tc.invalidate(&cfg)
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			runtime, err := New(ctx, cfg)
			if runtime != nil {
				t.Cleanup(func() { _ = runtime.Close() })
			}
			require.Error(t, err)
		})
	}
}

// Each response is published atomically, so refresh goroutines see one complete
// immutable body/status pair and race checks can include fixture updates.
type runtimeJWKSResponse struct {
	status int
	body   []byte
}
type runtimeJWKS struct {
	response atomic.Pointer[runtimeJWKSResponse]
	calls    atomic.Int64
}

func (j *runtimeJWKS) set(status int, body []byte) {
	j.response.Store(&runtimeJWKSResponse{status: status, body: append([]byte(nil), body...)})
}
func runtimeJWKSDocument(t *testing.T, keys map[string]*rsa.PrivateKey) []byte {
	t.Helper()
	entries := make([]map[string]string, 0, len(keys))
	for kid, key := range keys {
		entries = append(entries, map[string]string{"kid": kid, "kty": "RSA", "use": "sig", "alg": "RS256", "n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()), "e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes())})
	}
	body, err := json.Marshal(map[string]any{"keys": entries})
	require.NoError(t, err)
	return body
}

func TestRuntimeStartsBeforeIssuerAndAcceptsOnlyAfterTrustedJWKSRecovery(t *testing.T) {
	f := newRuntimeFixture(t, 2)
	f.config.UnknownKIDCooldown = 50 * time.Millisecond
	f.jwks.set(http.StatusServiceUnavailable, nil)
	runtime, err := New(context.Background(), f.config)
	require.NoError(t, err, "Space issuer startup must not form a cycle with Role health")
	t.Cleanup(func() { require.NoError(t, runtime.Close()) })
	method := rolev1.RoleService_PrepareOwnershipTransfer_FullMethodName
	token := issueRuntimeToken(t, f.key, "space", "current", "role", method, "issuer-not-ready", runtimeHash)
	got, err := runtime.Verify(context.Background(), token, method, "issuer-not-ready", runtimeHash)
	require.Error(t, err)
	require.Empty(t, got.Subject)
	require.Empty(t, f.redis.Keys(), "unverified credentials cannot be accepted or recorded as verified replay")
	f.jwks.set(http.StatusOK, runtimeJWKSDocument(t, map[string]*rsa.PrivateKey{"current": f.key, "next": f.next}))
	// Failed requests never acquire authority; once the issuer starts, the same
	// still-live credential can succeed after the bounded unknown-kid cooldown.
	require.Eventually(t, func() bool {
		got, err = runtime.Verify(context.Background(), token, method, "issuer-not-ready", runtimeHash)
		return err == nil
	}, 2*time.Second, 20*time.Millisecond)
	require.Equal(t, "service:space", got.Subject)
}
