package principalruntime

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	botv1 "voice.app/voice/bot/v1"
	"voice/backend/pkg/principal"
)

func TestConfigFromEnvDisablesWhenUnsetAndRejectsPartialRuntime(t *testing.T) {
	values := map[string]string{}
	get := func(key string) string { return values[key] }

	if _, enabled, err := ConfigFromEnv(get); err != nil || enabled {
		t.Fatalf("unset config: enabled=%v err=%v", enabled, err)
	}
	values["BOT_GAME_EVENT_GRPC_LISTEN"] = ":9443"
	if _, enabled, err := ConfigFromEnv(get); !enabled || err == nil {
		t.Fatalf("partial config: enabled=%v err=%v", enabled, err)
	}
}

func TestConfigFromEnvRequiresInternalHTTPSJWKS(t *testing.T) {
	values := completeConfigEnv()
	values["GAME_INTEGRATION_PRINCIPAL_JWKS_URL"] = "http://gameintegration:8080/internal/v1/principal/jwks.json"
	_, enabled, err := ConfigFromEnv(func(key string) string { return values[key] })
	if !enabled || err == nil {
		t.Fatalf("invalid URL must fail closed: enabled=%v err=%v", enabled, err)
	}
}

func TestConfigFromEnvAcceptsCompleteRuntime(t *testing.T) {
	values := completeConfigEnv()
	cfg, enabled, err := ConfigFromEnv(func(key string) string { return values[key] })
	if err != nil || !enabled {
		t.Fatalf("complete config: enabled=%v err=%v", enabled, err)
	}
	if cfg.BotGameEventListen != ":9443" || cfg.GISJWKSURL == "" || cfg.BotKeyID != "bot-key-1" {
		t.Fatalf("unexpected parsed config: %+v", cfg)
	}
}

func TestConfigFromEnvAcceptsCompleteSpaceLifecycleSettings(t *testing.T) {
	values := completeConfigEnv()
	values["BOT_SPACE_LIFECYCLE_GRPC_LISTEN"] = ":9444"
	values["BOT_SPACE_LIFECYCLE_TLS_CERT_FILE"] = "bot-space-lifecycle.crt"
	values["BOT_SPACE_LIFECYCLE_TLS_KEY_FILE"] = "bot-space-lifecycle.key"
	values["BOT_SPACE_LIFECYCLE_CLIENT_CA_FILE"] = "space-lifecycle-client-ca.crt"
	values["SPACE_PRINCIPAL_JWKS_URL"] = "https://phase0-jwks:8443/space/jwks.json"
	values["SPACE_PRINCIPAL_JWKS_CA_FILE"] = "phase0-ca.crt"
	cfg, enabled, err := ConfigFromEnv(func(key string) string { return values[key] })
	if err != nil || !enabled {
		t.Fatalf("complete config: enabled=%v err=%v", enabled, err)
	}
	if cfg.BotSpaceLifecycleListen != ":9444" || cfg.SpaceJWKSURL == "" {
		t.Fatalf("Space lifecycle config not parsed: %+v", cfg)
	}
}

func TestConfigFromEnvRejectsPartialSpaceLifecycleSettings(t *testing.T) {
	values := completeConfigEnv()
	values["BOT_SPACE_LIFECYCLE_GRPC_LISTEN"] = ":9444"
	_, enabled, err := ConfigFromEnv(func(key string) string { return values[key] })
	if !enabled || err == nil {
		t.Fatalf("partial Space lifecycle config must fail closed: enabled=%v err=%v", enabled, err)
	}
}

func TestConfigFromEnvRequiresHTTPSForSpaceJWKS(t *testing.T) {
	values := completeConfigEnv()
	values["BOT_SPACE_LIFECYCLE_GRPC_LISTEN"] = ":9444"
	values["BOT_SPACE_LIFECYCLE_TLS_CERT_FILE"] = "bot-space-lifecycle.crt"
	values["BOT_SPACE_LIFECYCLE_TLS_KEY_FILE"] = "bot-space-lifecycle.key"
	values["BOT_SPACE_LIFECYCLE_CLIENT_CA_FILE"] = "space-lifecycle-client-ca.crt"
	values["SPACE_PRINCIPAL_JWKS_URL"] = "http://phase0-jwks:8443/space/jwks.json"
	values["SPACE_PRINCIPAL_JWKS_CA_FILE"] = "phase0-ca.crt"
	_, enabled, err := ConfigFromEnv(func(key string) string { return values[key] })
	if !enabled || err == nil {
		t.Fatalf("non-HTTPS Space JWKS must fail closed: enabled=%v err=%v", enabled, err)
	}
}

func TestVerifySpaceBindsExactRequestAndRejectsReplay(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "space", KeyID: "current", PrivateKey: key, Clock: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	principal.JWKSHandler("current", &key.PublicKey).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/space/jwks.json", nil))
	resolver, err := principal.NewJWKSResolverWithConfig(principal.JWKSResolverConfig{Fetch: func(context.Context, string) ([]byte, error) { return response.Body.Bytes(), nil }, RefreshAfter: time.Minute, HardExpiry: 2 * time.Minute, UnknownKIDCooldown: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	runtime := &Runtime{spaceResolver: resolver, replayGuard: func(_ context.Context, issuer, id string, expiry time.Time) error {
		if issuer != "space" || !expiry.After(now) {
			t.Fatalf("unexpected replay binding issuer=%q expiry=%v", issuer, expiry)
		}
		if seen[id] {
			return errors.New("replay")
		}
		seen[id] = true
		return nil
	}}
	request := &botv1.PurgeSpaceRequest{}
	hash, err := principal.RequestHash(request)
	if err != nil {
		t.Fatal(err)
	}
	method := botv1.BotService_PurgeSpace_FullMethodName
	issue := func(audience, rpc, requestID, requestHash string) string {
		token, issueErr := issuer.IssueService(principal.ServiceInput{Audience: audience, RPC: rpc, RequestID: requestID, RequestHash: requestHash})
		if issueErr != nil {
			t.Fatal(issueErr)
		}
		return token
	}
	token := issue("bot", method, "request-1", hash)
	verified, err := runtime.VerifySpace(context.Background(), token, method, "request-1", hash)
	if err != nil || verified.Issuer != "space" || verified.Audience != "bot" {
		t.Fatalf("valid Space principal rejected: principal=%+v err=%v", verified, err)
	}
	if _, err := runtime.VerifySpace(context.Background(), token, method, "request-1", hash); err == nil {
		t.Fatal("replayed Space principal was accepted")
	}
	for _, tc := range []struct{ name, token, method, requestID, hash string }{
		{"audience", issue("voice", method, "request-1", hash), method, "request-1", hash},
		{"rpc", issue("bot", "/voice.bot.v1.BotService/Other", "request-1", hash), method, "request-1", hash},
		{"request id", issue("bot", method, "other", hash), method, "request-1", hash},
		{"request body", issue("bot", method, "request-1", "sha256:"+strings.Repeat("0", 64)), method, "request-1", hash},
		{"unknown method", token, "/voice.bot.v1.BotService/Other", "request-1", hash},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := runtime.VerifySpace(context.Background(), tc.token, tc.method, tc.requestID, tc.hash); err == nil {
				t.Fatal("invalid Space principal was accepted")
			}
		})
	}
}

func completeConfigEnv() map[string]string {
	return map[string]string{
		"GAME_INTEGRATION_PRINCIPAL_JWKS_URL":      "https://gameintegration/internal/v1/principal/jwks.json",
		"GAME_INTEGRATION_PRINCIPAL_TLS_CERT_FILE": "gis-client.crt",
		"GAME_INTEGRATION_PRINCIPAL_TLS_KEY_FILE":  "gis-client.key",
		"GAME_INTEGRATION_PRINCIPAL_TLS_CA_FILE":   "gis-ca.crt",
		"BOT_PRINCIPAL_REPLAY_REDIS_URL":           "redis://localhost:6379/0",
		"BOT_PRINCIPAL_PRIVATE_KEY_FILE":           "bot-signing.key",
		"BOT_PRINCIPAL_KEY_ID":                     "bot-key-1",
		"BOT_PRINCIPAL_JWKS_LISTEN":                ":9444",
		"BOT_PRINCIPAL_TLS_CERT_FILE":              "bot-jwks.crt",
		"BOT_PRINCIPAL_TLS_KEY_FILE":               "bot-jwks.key",
		"BOT_PRINCIPAL_TLS_CLIENT_CA_FILE":         "gis-client-ca.crt",
		"BOT_GAME_EVENT_GRPC_LISTEN":               ":9443",
		"BOT_GAME_EVENT_TLS_CERT_FILE":             "bot-game-event.crt",
		"BOT_GAME_EVENT_TLS_KEY_FILE":              "bot-game-event.key",
		"BOT_GAME_EVENT_CLIENT_CA_FILE":            "gis-client-ca.crt",
	}
}
