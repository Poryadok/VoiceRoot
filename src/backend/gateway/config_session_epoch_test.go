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
	"os"
	"strings"
	"testing"
	"time"
)

func TestGatewaySessionEpochStrictConfigDefaultsToCompatibility(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value *string
	}{
		{name: "unset"},
		{name: "exact false", value: configString("false")},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			configureSessionEpochEnv(t, tc.value, "")

			config, err := loadGatewayConfigFromEnvChecked()
			if err != nil {
				t.Fatalf("load config: %v", err)
			}
			if config.sessionEpochStrict {
				t.Fatal("session epoch strict unexpectedly enabled")
			}
			if config.sessionEpochFloor != nil {
				t.Fatalf("session epoch floor = %T, want nil in compatibility mode", config.sessionEpochFloor)
			}
		})
	}
}

func TestGatewaySessionEpochStrictConfigRejectsNonExactValues(t *testing.T) {
	for _, value := range []string{
		"", " true", "true ", " false", "false ",
		"TRUE", "True", "FALSE", "False", "1", "typo",
	} {
		value := value
		t.Run(value, func(t *testing.T) {
			configureSessionEpochEnv(t, &value, "127.0.0.1:6379")

			if _, err := loadGatewayConfigFromEnvChecked(); err == nil {
				t.Fatalf("strict value %q unexpectedly accepted", value)
			}
		})
	}
}

func TestGatewaySessionEpochStrictConfigRequiresRedisAndBuildsCanonicalAuthFloor(t *testing.T) {
	strict := "true"
	t.Run("requires Redis before serving", func(t *testing.T) {
		configureSessionEpochEnv(t, &strict, "")

		if _, err := loadGatewayConfigFromEnvChecked(); err == nil {
			t.Fatal("strict config without Redis unexpectedly succeeded")
		}
	})

	t.Run("uses Auth-owned Redis floor without prefix override or startup ping", func(t *testing.T) {
		configureSessionEpochEnv(t, &strict, "198.51.100.11:6379")
		t.Setenv("GATEWAY_REDIS_PASSWORD", "shared-password")

		config, err := loadGatewayConfigFromEnvChecked()
		if err != nil {
			t.Fatalf("load strict config: %v", err)
		}
		if !config.sessionEpochStrict {
			t.Fatal("session epoch strict not enabled")
		}
		floor, ok := config.sessionEpochFloor.(*redisSessionEpochFloor)
		if !ok || floor == nil {
			t.Fatalf("session epoch floor = %T, want *redisSessionEpochFloor", config.sessionEpochFloor)
		}
		// The epoch floor is Auth-owned and has no Gateway prefix override.
		if floor.addr != "198.51.100.11:6379" || floor.password != "shared-password" || floor.prefix != sessionEpochFloorPrefix || floor.timeout != 2*time.Second {
			t.Fatalf("floor = %#v, want shared Redis addr/password, Auth prefix, and 2s timeout", floor)
		}
	})
}

func TestGatewayBootstrapRejectsInvalidStrictConfigBeforeServerConstruction(t *testing.T) {
	for _, tc := range []struct {
		name   string
		strict string
		redis  string
	}{
		{name: "malformed strict value", strict: "TRUE", redis: "198.51.100.13:6379"},
		{name: "strict without Redis", strict: "true", redis: ""},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			configureSessionEpochEnv(t, &tc.strict, tc.redis)

			serverConstructed := 0
			server, err := newGatewayServerFromEnv(":8080", func(handler http.Handler) *http.Server {
				serverConstructed++
				return &http.Server{Addr: ":8080", Handler: handler}
			})
			if err == nil {
				t.Fatal("invalid strict config unexpectedly bootstrapped a server")
			}
			if server != nil {
				t.Fatalf("server = %#v, want nil on startup config error", server)
			}
			if serverConstructed != 0 {
				t.Fatalf("server constructor calls = %d, want 0", serverConstructed)
			}
		})
	}
}

func TestGatewayMainUsesCheckedConfigBootstrap(t *testing.T) {
	mainSource, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	if !strings.Contains(string(mainSource), "newGatewayServerFromEnv(") {
		t.Fatal("main must use newGatewayServerFromEnv so startup config errors prevent serving")
	}
}

func TestGatewaySessionEpochJWKSConfigUsesStrictnessOption(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	const issuer = "voice-auth"
	const audience = "voice-client"
	jwks := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"keys": []map[string]string{configRSAJWK("key-1", &key.PublicKey)},
		})
	}))
	t.Cleanup(jwks.Close)

	base := map[string]any{
		"sub": "account-1", "iss": issuer, "aud": audience,
		"exp": time.Now().Add(time.Hour).Unix(),
	}
	withEpoch := cloneConfigJWTPayload(base)
	withEpoch["session_epoch"] = int64(7)
	malformedEpoch := cloneConfigJWTPayload(base)
	malformedEpoch["session_epoch"] = "7"

	for _, tc := range []struct {
		name      string
		strict    *string
		redisAddr string
		payload   map[string]any
		wantCode  string
		wantEpoch int64
	}{
		{name: "compatibility permits legacy JWT", payload: base, wantEpoch: 0},
		{name: "compatibility permits positive epoch", payload: withEpoch, wantEpoch: 7},
		{name: "compatibility rejects malformed present epoch", payload: malformedEpoch, wantCode: "invalid_token"},
		{name: "strict rejects legacy JWT", strict: configString("true"), redisAddr: "198.51.100.12:6379", payload: base, wantCode: "invalid_token"},
		{name: "strict permits positive epoch", strict: configString("true"), redisAddr: "198.51.100.12:6379", payload: withEpoch, wantEpoch: 7},
		{name: "strict rejects malformed epoch", strict: configString("true"), redisAddr: "198.51.100.12:6379", payload: malformedEpoch, wantCode: "invalid_token"},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			configureSessionEpochEnv(t, tc.strict, tc.redisAddr)
			t.Setenv("GATEWAY_JWKS_URL", jwks.URL)
			t.Setenv("GATEWAY_JWT_ISSUER", issuer)
			t.Setenv("GATEWAY_JWT_AUDIENCE", audience)

			config, err := loadGatewayConfigFromEnvChecked()
			if err != nil {
				t.Fatalf("load config: %v", err)
			}
			token := signConfigJWT(t, "key-1", key, tc.payload)
			req := httptest.NewRequest(http.MethodGet, "/api/v1/users/me", nil)
			req.Header.Set("Authorization", "Bearer "+token)
			claims, code := config.tokenValidator.Validate(req)
			if code != tc.wantCode {
				t.Fatalf("JWT validation code = %q, want %q", code, tc.wantCode)
			}
			if code == "" && claims.SessionEpoch != tc.wantEpoch {
				t.Fatalf("session epoch = %d, want %d", claims.SessionEpoch, tc.wantEpoch)
			}
		})
	}
}

func configureSessionEpochEnv(t *testing.T, strict *string, redisAddr string) {
	t.Helper()
	t.Setenv("GATEWAY_AUTH_MODE", "")
	t.Setenv("GATEWAY_STATIC_TOKENS_JSON", "")
	t.Setenv("GATEWAY_JWKS_URL", "")
	t.Setenv("GATEWAY_JWT_ISSUER", "")
	t.Setenv("GATEWAY_JWT_AUDIENCE", "")
	t.Setenv("GATEWAY_REDIS_ADDR", redisAddr)
	t.Setenv("GATEWAY_REDIS_PASSWORD", "")
	if strict == nil {
		t.Setenv("GATEWAY_SESSION_EPOCH_STRICT", "placeholder")
		if err := os.Unsetenv("GATEWAY_SESSION_EPOCH_STRICT"); err != nil {
			t.Fatalf("unset strict env: %v", err)
		}
		return
	}
	t.Setenv("GATEWAY_SESSION_EPOCH_STRICT", *strict)
}

func configString(value string) *string {
	return &value
}

func cloneConfigJWTPayload(payload map[string]any) map[string]any {
	clone := make(map[string]any, len(payload))
	for key, value := range payload {
		clone[key] = value
	}
	return clone
}

func configRSAJWK(kid string, key *rsa.PublicKey) map[string]string {
	return map[string]string{
		"kty": "RSA",
		"kid": kid,
		"alg": "RS256",
		"n":   base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
		"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
	}
}

func signConfigJWT(t *testing.T, kid string, key *rsa.PrivateKey, payload map[string]any) string {
	t.Helper()
	header, err := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT", "kid": kid})
	if err != nil {
		t.Fatalf("marshal JWT header: %v", err)
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal JWT payload: %v", err)
	}
	signed := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(body)
	digest := sha256.Sum256([]byte(signed))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatalf("sign JWT: %v", err)
	}
	return signed + "." + base64.RawURLEncoding.EncodeToString(signature)
}
