package matchsquadmemberprincipal

import (
	"strings"
	"testing"
)

func TestLoadFromEnvIsAbsentOrStrict(t *testing.T) {
	lookup := func(string) (string, bool) { return "", false }
	if _, enabled, err := LoadFromEnv(lookup); err != nil || enabled {
		t.Fatalf("empty config = enabled %v, err %v", enabled, err)
	}
	values := map[string]string{"TLS_CERT_FILE": "cert", "TLS_KEY_FILE": "key", "CLIENT_CA_FILE": "client-ca", "JWKS_CA_FILE": "jwks-ca", "JWKS_URL": "https://gateway.example/jwks", "REDIS_ADDR": "redis:6379"}
	lookup = func(key string) (string, bool) {
		value, ok := values[strings.TrimPrefix(key, envPrefix)]
		return value, ok
	}
	if _, enabled, err := LoadFromEnv(lookup); err == nil || !enabled {
		t.Fatalf("partial config = enabled %v, err %v; want enabled error", enabled, err)
	}
	values["GRPC_LISTEN"] = ":9093"
	if cfg, enabled, err := LoadFromEnv(lookup); err != nil || !enabled || cfg.ListenerAddr != ":9093" {
		t.Fatalf("complete config = %#v, enabled %v, err %v", cfg, enabled, err)
	}
	values["JWKS_URL"] = "https://gateway.example/jwks?token=secret"
	if _, _, err := LoadFromEnv(lookup); err == nil {
		t.Fatal("JWKS query credentials were accepted")
	}
}
