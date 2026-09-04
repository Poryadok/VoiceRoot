package main

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

func TestRealtimeSessionEpochStrictConfigDefaultsToCompatibility(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value *string
	}{
		{name: "unset"},
		{name: "exact false", value: realtimeConfigString("false")},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			configureRealtimeSessionEpochEnv(t, tc.value, "", "", "", "")

			config, err := loadRealtimeConfigFromEnvChecked()
			require.NoError(t, err)
			require.False(t, config.sessionEpochStrict)
			require.Nil(t, config.sessionEpochFloor)
		})
	}
}

func TestRealtimeSessionEpochStrictConfigRejectsNonExactValues(t *testing.T) {
	for _, value := range []string{
		"", " true", "true ", " false", "false ",
		"TRUE", "True", "FALSE", "False", "1", "typo",
	} {
		value := value
		t.Run(value, func(t *testing.T) {
			configureRealtimeSessionEpochEnv(t, &value, "127.0.0.1:6379", "https://auth.example/jwks", "voice-auth", "voice-client")

			_, err := loadRealtimeConfigFromEnvChecked()
			require.Error(t, err)
		})
	}
}

func TestRealtimeSessionEpochStrictConfigFailsFastForMissingDependencies(t *testing.T) {
	strict := "true"
	for _, tc := range []struct {
		name      string
		redisAddr string
		jwksURL   string
		issuer    string
		audience  string
	}{
		{name: "missing Redis", jwksURL: "https://auth.example/jwks", issuer: "voice-auth", audience: "voice-client"},
		{name: "missing independent JWKS", redisAddr: "127.0.0.1:6379", issuer: "voice-auth", audience: "voice-client"},
		{name: "missing JWT issuer", redisAddr: "127.0.0.1:6379", jwksURL: "https://auth.example/jwks", audience: "voice-client"},
		{name: "missing JWT audience", redisAddr: "127.0.0.1:6379", jwksURL: "https://auth.example/jwks", issuer: "voice-auth"},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			configureRealtimeSessionEpochEnv(t, &strict, tc.redisAddr, tc.jwksURL, tc.issuer, tc.audience)

			_, err := loadRealtimeConfigFromEnvChecked()
			require.Error(t, err)
		})
	}
}

func TestRealtimeSessionEpochStrictConfigBuildsAuthOwnedFloorWithoutStartupPing(t *testing.T) {
	strict := "true"
	configureRealtimeSessionEpochEnv(t, &strict, "198.51.100.11:6379", "https://auth.example/jwks", "voice-auth", "voice-client")
	t.Setenv("REALTIME_REDIS_PASSWORD", "shared-password")

	config, err := loadRealtimeConfigFromEnvChecked()
	require.NoError(t, err)
	require.True(t, config.sessionEpochStrict)
	require.NotNil(t, config.tokenValidator, "strict mode must independently validate the upstream JWT")

	floor, ok := config.sessionEpochFloor.(*redisSessionEpochFloor)
	require.True(t, ok, "strict mode must construct the read-only Auth epoch floor")
	require.NotNil(t, floor)
	require.Equal(t, "198.51.100.11:6379", floor.addr)
	require.Equal(t, "shared-password", floor.password)
	require.Equal(t, sessionEpochFloorPrefix, floor.prefix)
	require.Equal(t, 2*time.Second, floor.timeout)
}

func TestRealtimeBootstrapRejectsInvalidStrictConfigBeforeServerConstruction(t *testing.T) {
	for _, tc := range []struct {
		name      string
		strict    string
		redisAddr string
		jwksURL   string
		issuer    string
		audience  string
	}{
		{name: "malformed strict value", strict: "TRUE", redisAddr: "127.0.0.1:6379", jwksURL: "https://auth.example/jwks", issuer: "voice-auth", audience: "voice-client"},
		{name: "strict without Redis", strict: "true", jwksURL: "https://auth.example/jwks", issuer: "voice-auth", audience: "voice-client"},
		{name: "strict without JWKS", strict: "true", redisAddr: "127.0.0.1:6379", issuer: "voice-auth", audience: "voice-client"},
		{name: "strict without issuer", strict: "true", redisAddr: "127.0.0.1:6379", jwksURL: "https://auth.example/jwks", audience: "voice-client"},
		{name: "strict without audience", strict: "true", redisAddr: "127.0.0.1:6379", jwksURL: "https://auth.example/jwks", issuer: "voice-auth"},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			configureRealtimeSessionEpochEnv(t, &tc.strict, tc.redisAddr, tc.jwksURL, tc.issuer, tc.audience)

			serverConstructed := 0
			server, err := newRealtimeServerFromEnv(":8080", func(handler http.Handler) *http.Server {
				serverConstructed++
				return &http.Server{Addr: ":8080", Handler: handler}
			})
			require.Error(t, err)
			require.Nil(t, server)
			require.Zero(t, serverConstructed)
		})
	}
}

func TestRealtimeMainUsesCheckedSessionEpochBootstrap(t *testing.T) {
	mainSource, err := os.ReadFile("main.go")
	require.NoError(t, err)
	require.Contains(t, string(mainSource), "newRealtimeServerFromEnv(")
}

func TestRealtimeSessionEpochJWKSConfigPreservesCompatibilityAndRejectsMalformedPresentClaim(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	const issuer = "voice-auth"
	const audience = "voice-client"
	jwks := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if err := json.NewEncoder(w).Encode(map[string]any{
			"keys": []map[string]string{realtimeConfigRSAJWK("key-1", &key.PublicKey)},
		}); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}))
	t.Cleanup(jwks.Close)
	mr := miniredis.RunT(t)

	base := map[string]any{
		"sub": "account-1", "profile_id": "profile-1", "iss": issuer, "aud": audience,
		"exp": time.Now().Add(time.Hour).Unix(),
	}
	positiveEpoch := cloneRealtimeConfigJWTPayload(base)
	positiveEpoch["session_epoch"] = int64(7)
	zeroEpoch := cloneRealtimeConfigJWTPayload(base)
	zeroEpoch["session_epoch"] = int64(0)
	stringEpoch := cloneRealtimeConfigJWTPayload(base)
	stringEpoch["session_epoch"] = "7"
	fractionalEpoch := cloneRealtimeConfigJWTPayload(base)
	fractionalEpoch["session_epoch"] = 7.5

	falseValue := "false"
	for _, tc := range []struct {
		name      string
		strict    *string
		redisAddr string
		payload   map[string]any
		wantCode  string
		wantEpoch int64
	}{
		{name: "unset accepts legacy JWT", payload: base},
		{name: "exact false accepts legacy JWT", strict: &falseValue, payload: base},
		{name: "compatibility accepts positive epoch", payload: positiveEpoch, wantEpoch: 7},
		{name: "compatibility rejects zero epoch", payload: zeroEpoch, wantCode: "invalid_token"},
		{name: "compatibility rejects string epoch", payload: stringEpoch, wantCode: "invalid_token"},
		{name: "compatibility rejects fractional epoch", payload: fractionalEpoch, wantCode: "invalid_token"},
		{name: "strict rejects legacy JWT", strict: realtimeConfigString("true"), redisAddr: mr.Addr(), payload: base, wantCode: "invalid_token"},
		{name: "strict accepts positive epoch", strict: realtimeConfigString("true"), redisAddr: mr.Addr(), payload: positiveEpoch, wantEpoch: 7},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			configureRealtimeSessionEpochEnv(t, tc.strict, tc.redisAddr, jwks.URL, issuer, audience)

			config, err := loadRealtimeConfigFromEnvChecked()
			require.NoError(t, err)
			token := signRealtimeConfigJWT(t, "key-1", key, tc.payload)
			req := httptest.NewRequest(http.MethodGet, "/ws", nil)
			req.Header.Set("Authorization", "Bearer "+token)
			claims, code := config.tokenValidator.Validate(req)
			require.Equal(t, tc.wantCode, code)
			if code == "" {
				require.Equal(t, tc.wantEpoch, claims.SessionEpoch)
			}
		})
	}
}

func TestRealtimeStrictJWTValidationNeverAuthenticatesRawVoiceHeaders(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	const issuer = "voice-auth"
	const audience = "voice-client"
	jwks := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if err := json.NewEncoder(w).Encode(map[string]any{
			"keys": []map[string]string{realtimeConfigRSAJWK("key-1", &key.PublicKey)},
		}); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}))
	t.Cleanup(jwks.Close)
	mr := miniredis.RunT(t)
	strict := "true"
	configureRealtimeSessionEpochEnv(t, &strict, mr.Addr(), jwks.URL, issuer, audience)
	require.NoError(t, mr.Set("auth:session:min_epoch:account-1", "7"))
	config, err := loadRealtimeConfigFromEnvChecked()
	require.NoError(t, err)

	payload := map[string]any{
		"sub": "account-1", "profile_id": "profile-1", "iss": issuer, "aud": audience,
		"exp": time.Now().Add(time.Hour).Unix(), "session_epoch": int64(7),
	}
	token := signRealtimeConfigJWT(t, "key-1", key, payload)
	server := httptest.NewServer(newWSHandler(config.tokenValidator, nil, newWSHub(), nil, "epoch-auth-test", nil, nil))
	t.Cleanup(server.Close)

	dial := func(headers http.Header) (*websocket.Conn, *http.Response, error) {
		t.Helper()
		return websocket.DefaultDialer.Dial(realtimeConfigWSEndpoint(t, server), headers)
	}

	validButSpoofed := http.Header{}
	validButSpoofed.Set("Authorization", "Bearer "+token)
	validButSpoofed.Set("X-Voice-User-Id", "attacker-account")
	validButSpoofed.Set("X-Voice-Profile-Id", "profile-1")
	validButSpoofed.Set("X-Voice-Session-Epoch", "999")
	conn, response, err := dial(validButSpoofed)
	require.NoError(t, err, "verified Authorization must win over raw X-Voice identity headers; response=%v", response)
	t.Cleanup(func() { _ = conn.Close() })
	_, _, err = conn.ReadMessage()
	require.NoError(t, err, "verified token connection must receive hello")

	rawOnly := http.Header{}
	rawOnly.Set("X-Voice-User-Id", "account-1")
	rawOnly.Set("X-Voice-Profile-Id", "profile-1")
	rawOnly.Set("X-Voice-Session-Epoch", "7")
	_, response, err = dial(rawOnly)
	require.Error(t, err)
	require.NotNil(t, response)
	require.Equal(t, http.StatusUnauthorized, response.StatusCode)

	conflictingProfile := http.Header{}
	conflictingProfile.Set("Authorization", "Bearer "+token)
	conflictingProfile.Set("X-Voice-Profile-Id", "attacker-profile")
	_, response, err = dial(conflictingProfile)
	require.Error(t, err)
	require.NotNil(t, response)
	require.Equal(t, http.StatusUnauthorized, response.StatusCode)
}

func configureRealtimeSessionEpochEnv(t *testing.T, strict *string, redisAddr, jwksURL, issuer, audience string) {
	t.Helper()
	t.Setenv("REALTIME_REDIS_ADDR", redisAddr)
	t.Setenv("REALTIME_REDIS_PASSWORD", "")
	t.Setenv("REALTIME_JWKS_URL", jwksURL)
	t.Setenv("GATEWAY_JWKS_URL", "")
	t.Setenv("REALTIME_JWT_ISSUER", issuer)
	t.Setenv("GATEWAY_JWT_ISSUER", "")
	t.Setenv("REALTIME_JWT_AUDIENCE", audience)
	t.Setenv("GATEWAY_JWT_AUDIENCE", "")
	if strict == nil {
		t.Setenv("REALTIME_SESSION_EPOCH_STRICT", "placeholder")
		require.NoError(t, os.Unsetenv("REALTIME_SESSION_EPOCH_STRICT"))
		return
	}
	t.Setenv("REALTIME_SESSION_EPOCH_STRICT", *strict)
}

func realtimeConfigString(value string) *string {
	return &value
}

func cloneRealtimeConfigJWTPayload(payload map[string]any) map[string]any {
	clone := make(map[string]any, len(payload))
	for key, value := range payload {
		clone[key] = value
	}
	return clone
}

func realtimeConfigRSAJWK(kid string, key *rsa.PublicKey) map[string]string {
	return map[string]string{
		"kty": "RSA",
		"kid": kid,
		"alg": "RS256",
		"n":   base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
		"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
	}
}

func signRealtimeConfigJWT(t *testing.T, kid string, key *rsa.PrivateKey, payload map[string]any) string {
	t.Helper()
	header, err := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT", "kid": kid})
	require.NoError(t, err)
	body, err := json.Marshal(payload)
	require.NoError(t, err)
	signed := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(body)
	digest := sha256.Sum256([]byte(signed))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	require.NoError(t, err)
	return signed + "." + base64.RawURLEncoding.EncodeToString(signature)
}

func realtimeConfigWSEndpoint(t *testing.T, server *httptest.Server) string {
	t.Helper()
	endpoint, err := url.Parse(server.URL)
	require.NoError(t, err)
	endpoint.Scheme = "ws"
	endpoint.Path = "/ws"
	return endpoint.String()
}
