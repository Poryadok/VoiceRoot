package spaceprincipalruntime

import (
	"os"
	"testing"
)

var lifecycleEnvNames = []string{
	"NOTIFICATION_SPACE_LIFECYCLE_GRPC_LISTEN",
	"NOTIFICATION_SPACE_LIFECYCLE_TLS_CERT_FILE",
	"NOTIFICATION_SPACE_LIFECYCLE_TLS_KEY_FILE",
	"NOTIFICATION_SPACE_LIFECYCLE_CLIENT_CA_FILE",
	"NOTIFICATION_SPACE_LIFECYCLE_REPLAY_REDIS_ADDR",
	"NOTIFICATION_SPACE_LIFECYCLE_REPLAY_REDIS_PASSWORD",
	"S2S_JWKS_URLS_JSON",
	"S2S_JWKS_REFRESH_AFTER",
	"S2S_JWKS_HARD_EXPIRY",
	"S2S_UNKNOWN_KID_COOLDOWN",
	"S2S_JWKS_CA_FILE",
}

func isolateLifecycleEnv(t *testing.T) {
	t.Helper()
	for _, name := range lifecycleEnvNames {
		value, existed := os.LookupEnv(name)
		if err := os.Unsetenv(name); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if existed {
				_ = os.Setenv(name, value)
			} else {
				_ = os.Unsetenv(name)
			}
		})
	}
}

func TestLoadFromEnvDisablesWhenConfigurationIsAbsent(t *testing.T) {
	isolateLifecycleEnv(t)

	_, enabled, err := LoadFromEnv()
	if err != nil || enabled {
		t.Fatalf("LoadFromEnv() = enabled %v, err %v; want disabled without error", enabled, err)
	}
}

func TestLoadFromEnvRejectsPartialConfiguration(t *testing.T) {
	isolateLifecycleEnv(t)
	t.Setenv("NOTIFICATION_SPACE_LIFECYCLE_GRPC_LISTEN", ":9445")

	_, enabled, err := LoadFromEnv()
	if err == nil || !enabled {
		t.Fatalf("LoadFromEnv() = enabled %v, err %v; want enabled with configuration error", enabled, err)
	}
}

func TestLoadFromEnvAcceptsCompleteConfiguration(t *testing.T) {
	isolateLifecycleEnv(t)
	for name, value := range map[string]string{
		"NOTIFICATION_SPACE_LIFECYCLE_GRPC_LISTEN":       ":9445",
		"NOTIFICATION_SPACE_LIFECYCLE_TLS_CERT_FILE":     "/tls/notification.crt",
		"NOTIFICATION_SPACE_LIFECYCLE_TLS_KEY_FILE":      "/tls/notification.key",
		"NOTIFICATION_SPACE_LIFECYCLE_CLIENT_CA_FILE":    "/ca/space-lifecycle.crt",
		"NOTIFICATION_SPACE_LIFECYCLE_REPLAY_REDIS_ADDR": "redis:6379",
		"S2S_JWKS_URLS_JSON":                             `{"space":"https://phase0-jwks:8443/space/jwks.json"}`,
	} {
		t.Setenv(name, value)
	}

	cfg, enabled, err := LoadFromEnv()
	if err != nil || !enabled {
		t.Fatalf("LoadFromEnv() = enabled %v, err %v; want enabled without error", enabled, err)
	}
	if cfg.ListenAddr != ":9445" || cfg.JWKSURL != "https://phase0-jwks:8443/space/jwks.json" || cfg.RefreshAfter <= 0 {
		t.Fatalf("LoadFromEnv() returned unexpected config: %+v", cfg)
	}
}

func TestConfigRejectsNonHTTPSJWKS(t *testing.T) {
	cfg := Config{
		ListenAddr: ":9445", ReplayAddr: "redis:6379", TLSCertFile: "server.crt", TLSKeyFile: "server.key", ClientCAFile: "client-ca.crt",
		JWKSURL: "http://phase0-jwks/space/jwks.json", RefreshAfter: 30, HardExpiry: 120, UnknownKIDCooldown: 5,
	}
	if err := cfg.validate(); err == nil {
		t.Fatal("Config.validate() accepted an HTTP JWKS endpoint")
	}
}
