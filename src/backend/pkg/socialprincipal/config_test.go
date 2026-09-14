package socialprincipal

import (
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestConfigPartialAndDefaults(t *testing.T) {
	for _, target := range []string{"user", "space"} {
		t.Run(target, func(t *testing.T) {
			_, enabled, err := LoadFromEnv(target)
			require.NoError(t, err)
			require.False(t, enabled)
			prefix := prefixFor(target)
			t.Setenv(prefix+"TLS_CERT_FILE", "cert.pem")
			_, enabled, err = LoadFromEnv(target)
			require.Error(t, err)
			require.True(t, enabled)
			t.Setenv(prefix+"TLS_KEY_FILE", "key.pem")
			t.Setenv(prefix+"REPLAY_REDIS_ADDR", "redis:6379")
			t.Setenv("S2S_JWKS_URLS_JSON", `{"social":"https://social:8443/.well-known/jwks.json"}`)
			cfg, enabled, err := LoadFromEnv(target)
			require.NoError(t, err)
			require.True(t, enabled)
			require.Equal(t, ":9091", cfg.ListenAddr)
			require.Equal(t, 30*time.Second, cfg.RefreshAfter)
			require.Equal(t, 2*time.Minute, cfg.HardExpiry)
			require.Equal(t, 5*time.Second, cfg.UnknownKIDCooldown)
			t.Setenv("S2S_JWKS_HARD_EXPIRY", "")
			_, _, err = LoadFromEnv(target)
			require.Error(t, err)
		})
	}
}

func TestConfigRejectsUnsafeEndpoint(t *testing.T) {
	for _, endpoint := range []string{"http://social/jwks", "https://user:pass@social/jwks", "https://social/jwks#fragment"} {
		cfg := Config{Target: "user", JWKSURLs: map[string]string{"social": endpoint}, RefreshAfter: 30 * time.Second, HardExpiry: 2 * time.Minute, UnknownKIDCooldown: 5 * time.Second, ReplayAddr: "redis:6379", TLSCertFile: "cert", TLSKeyFile: "key", ListenAddr: ":9091"}
		require.Error(t, cfg.validate())
	}
}
