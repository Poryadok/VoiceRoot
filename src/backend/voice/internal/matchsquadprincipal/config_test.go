package matchsquadprincipal

import (
	"strings"
	"testing"
)

func envLookup(values map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	}
}

func completeEnv() map[string]string {
	return map[string]string{
		envPrefix + "GRPC_LISTEN":       "127.0.0.1:9092",
		envPrefix + "TLS_CERT_FILE":     "server.crt",
		envPrefix + "TLS_KEY_FILE":      "server.key",
		envPrefix + "CLIENT_CA_FILE":    "client-ca.pem",
		envPrefix + "JWKS_URL":          "https://matchmaking.internal/principal/jwks.json",
		envPrefix + "JWKS_CA_FILE":      "issuer-ca.pem",
		envPrefix + "REPLAY_REDIS_ADDR": "redis.internal:6379",
	}
}

func TestLoadFromEnvDisablesOnlyWhenAllOwnedVariablesAreAbsent(t *testing.T) {
	cfg, enabled, err := LoadFromEnv(envLookup(map[string]string{}))
	if err != nil || enabled || cfg != (Config{}) {
		t.Fatalf("empty config = %#v, enabled=%v, err=%v", cfg, enabled, err)
	}
	partial := map[string]string{envPrefix + "JWKS_CA_FILE": ""}
	if _, enabled, err := LoadFromEnv(envLookup(partial)); !enabled || err == nil {
		t.Fatalf("present partial config must fail closed: enabled=%v err=%v", enabled, err)
	}
}

func TestLoadFromEnvAcceptsCompleteExactListenerConfig(t *testing.T) {
	cfg, enabled, err := LoadFromEnv(envLookup(completeEnv()))
	if err != nil || !enabled {
		t.Fatalf("complete config rejected: enabled=%v err=%v", enabled, err)
	}
	if cfg.ListenerAddr != "127.0.0.1:9092" || cfg.JWKSURL != "https://matchmaking.internal/principal/jwks.json" || cfg.ReplayRedisAddr != "redis.internal:6379" {
		t.Fatalf("config values were not normalized: %#v", cfg)
	}
}

func TestLoadFromEnvRejectsPartialOrUnsafeJWKSConfiguration(t *testing.T) {
	partial := completeEnv()
	delete(partial, envPrefix+"CLIENT_CA_FILE")
	if _, _, err := LoadFromEnv(envLookup(partial)); err == nil {
		t.Fatal("partial listener config was accepted")
	}
	for _, jwks := range []string{
		"http://matchmaking.internal/jwks.json",
		"https://user:pass@matchmaking.internal/jwks.json",
		"https://matchmaking.internal/jwks.json?token=secret",
		"https://matchmaking.internal/jwks.json#fragment",
	} {
		values := completeEnv()
		values[envPrefix+"JWKS_URL"] = jwks
		_, _, err := LoadFromEnv(envLookup(values))
		if err == nil || strings.Contains(err.Error(), "secret") {
			t.Fatalf("unsafe JWKS URL was not rejected safely: %q err=%v", jwks, err)
		}
	}
}
