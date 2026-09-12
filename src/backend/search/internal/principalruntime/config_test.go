package principalruntime

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func clearPrincipalEnv(t *testing.T) {
	t.Helper()
	for _, name := range []string{"SEARCH_PRINCIPAL_GRPC_LISTEN", "SEARCH_PRINCIPAL_TLS_CERT_FILE", "SEARCH_PRINCIPAL_TLS_KEY_FILE", "SEARCH_PRINCIPAL_REPLAY_REDIS_ADDR", "SEARCH_PRINCIPAL_REPLAY_REDIS_PASSWORD", "S2S_JWKS_URLS_JSON", "S2S_JWKS_REFRESH_AFTER", "S2S_JWKS_HARD_EXPIRY", "S2S_UNKNOWN_KID_COOLDOWN", "S2S_JWKS_CA_FILE"} {
		value, set := os.LookupEnv(name)
		require.NoError(t, os.Unsetenv(name))
		t.Cleanup(func() {
			if set {
				_ = os.Setenv(name, value)
			} else {
				_ = os.Unsetenv(name)
			}
		})
	}
}

func TestLoadFromEnvDisabledOnlyWhenEveryPrincipalSettingIsAbsent(t *testing.T) {
	clearPrincipalEnv(t)
	_, enabled, err := LoadFromEnv()
	require.NoError(t, err)
	require.False(t, enabled)
}

func TestLoadFromEnvRejectsPartialAndMalformedConfiguration(t *testing.T) {
	clearPrincipalEnv(t)
	t.Setenv("SEARCH_PRINCIPAL_GRPC_LISTEN", ":9091")
	_, enabled, err := LoadFromEnv()
	require.True(t, enabled)
	require.Error(t, err)
}

func TestConfigAcceptsOnlyExactSpaceHTTPSTrustAndCanonicalDurations(t *testing.T) {
	cfg := Config{JWKSURLs: map[string]string{"space": "https://space.internal/.well-known/jwks.json"}, RefreshAfter: 30 * time.Second, HardExpiry: 2 * time.Minute, UnknownKIDCooldown: 5 * time.Second, ReplayAddr: "redis:6379", TLSCertFile: "cert.pem", TLSKeyFile: "key.pem", ListenAddr: ":9091"}
	require.NoError(t, cfg.validate())
	cfg.JWKSURLs["chat"] = "https://chat.internal/jwks"
	require.Error(t, cfg.validate(), "unexpected issuers must not widen Search lifecycle authority")
	delete(cfg.JWKSURLs, "chat")
	cfg.JWKSURLs["space"] = "http://space.internal/jwks"
	require.Error(t, cfg.validate(), "plaintext JWKS is forbidden")
}
