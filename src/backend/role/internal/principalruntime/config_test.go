package principalruntime

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

var runtimeEnvNames = []string{
	"S2S_JWKS_URLS_JSON", "S2S_JWKS_REFRESH_AFTER", "S2S_JWKS_HARD_EXPIRY",
	"S2S_UNKNOWN_KID_COOLDOWN", "S2S_JWKS_CA_FILE",
	"ROLE_PRINCIPAL_REPLAY_REDIS_ADDR", "ROLE_PRINCIPAL_REPLAY_REDIS_PASSWORD",
	"ROLE_PRINCIPAL_TLS_CERT_FILE", "ROLE_PRINCIPAL_TLS_KEY_FILE", "ROLE_PRINCIPAL_GRPC_LISTEN",
}

func clearRuntimeEnv(t *testing.T) {
	t.Helper()
	for _, name := range runtimeEnvNames {
		// Setenv registers restoration before Unsetenv makes absence observable.
		t.Setenv(name, "")
		require.NoError(t, os.Unsetenv(name))
	}
}

func validRuntimeEnv(t *testing.T) {
	t.Helper()
	clearRuntimeEnv(t)
	t.Setenv("S2S_JWKS_URLS_JSON", `{"space":"https://space.internal/jwks"}`)
	t.Setenv("ROLE_PRINCIPAL_REPLAY_REDIS_ADDR", "redis.internal:6379")
	t.Setenv("ROLE_PRINCIPAL_TLS_CERT_FILE", "role.crt")
	t.Setenv("ROLE_PRINCIPAL_TLS_KEY_FILE", "role.key")
}

func TestLoadFromEnvDisabledOnlyWhenNarrowConfigurationAbsent(t *testing.T) {
	clearRuntimeEnv(t)
	_, enabled, err := LoadFromEnv()
	require.NoError(t, err)
	require.False(t, enabled)
}

func TestLoadFromEnvUsesPhaseZeroDefaults(t *testing.T) {
	validRuntimeEnv(t)
	cfg, enabled, err := LoadFromEnv()
	require.NoError(t, err)
	require.True(t, enabled)
	require.Equal(t, map[string]string{"space": "https://space.internal/jwks"}, cfg.JWKSURLs)
	require.Equal(t, 30*time.Second, cfg.RefreshAfter)
	require.Equal(t, 2*time.Minute, cfg.HardExpiry)
	require.Equal(t, 5*time.Second, cfg.UnknownKIDCooldown)
	require.Equal(t, "redis.internal:6379", cfg.ReplayAddr)
	require.Equal(t, "role.crt", cfg.TLSCertFile)
	require.Equal(t, "role.key", cfg.TLSKeyFile)
	require.Equal(t, ":9091", cfg.ListenAddr)
}

func TestLoadFromEnvPreservesExplicitConfiguration(t *testing.T) {
	validRuntimeEnv(t)
	t.Setenv("S2S_JWKS_REFRESH_AFTER", "20s")
	t.Setenv("S2S_JWKS_HARD_EXPIRY", "1m")
	t.Setenv("S2S_UNKNOWN_KID_COOLDOWN", "3s")
	t.Setenv("S2S_JWKS_CA_FILE", "internal-ca.pem")
	t.Setenv("ROLE_PRINCIPAL_REPLAY_REDIS_PASSWORD", "test-password")
	t.Setenv("ROLE_PRINCIPAL_GRPC_LISTEN", "127.0.0.1:19091")
	cfg, enabled, err := LoadFromEnv()
	require.NoError(t, err)
	require.True(t, enabled)
	require.Equal(t, 20*time.Second, cfg.RefreshAfter)
	require.Equal(t, time.Minute, cfg.HardExpiry)
	require.Equal(t, 3*time.Second, cfg.UnknownKIDCooldown)
	require.Equal(t, "internal-ca.pem", cfg.JWKSCAFile)
	require.Equal(t, "test-password", cfg.ReplayPassword)
	require.Equal(t, "127.0.0.1:19091", cfg.ListenAddr)
}

func TestLoadFromEnvPartialConfigurationFailsClosed(t *testing.T) {
	for _, name := range runtimeEnvNames {
		t.Run(name, func(t *testing.T) {
			clearRuntimeEnv(t)
			t.Setenv(name, "")
			_, _, err := LoadFromEnv()
			require.Error(t, err, "present but empty configuration must not disable security")
		})
	}
	for _, name := range []string{"S2S_JWKS_URLS_JSON", "ROLE_PRINCIPAL_REPLAY_REDIS_ADDR", "ROLE_PRINCIPAL_TLS_CERT_FILE", "ROLE_PRINCIPAL_TLS_KEY_FILE"} {
		t.Run("missing_"+name, func(t *testing.T) {
			validRuntimeEnv(t)
			require.NoError(t, os.Unsetenv(name))
			_, _, err := LoadFromEnv()
			require.Error(t, err)
		})
	}
}

func TestLoadFromEnvRejectsUntrustedJWKSConfiguration(t *testing.T) {
	for name, value := range map[string]string{
		"malformed": "{", "null": "null", "empty": "{}",
		"missing_space": `{"gateway":"https://gateway.internal/jwks"}`,
		"plaintext":     `{"space":"http://space.internal/jwks"}`,
		"missing_host":  `{"space":"https:///jwks"}`,
		"empty_url":     `{"space":""}`,
		"empty_issuer":  `{"space":"https://space.internal/jwks","":"https://other.internal/jwks"}`,
	} {
		t.Run(name, func(t *testing.T) {
			validRuntimeEnv(t)
			t.Setenv("S2S_JWKS_URLS_JSON", value)
			_, _, err := LoadFromEnv()
			require.Error(t, err)
		})
	}
}

func TestLoadFromEnvRejectsInvalidCacheDurations(t *testing.T) {
	for _, name := range []string{"S2S_JWKS_REFRESH_AFTER", "S2S_JWKS_HARD_EXPIRY", "S2S_UNKNOWN_KID_COOLDOWN"} {
		for _, value := range []string{"", " ", "not-a-duration", "0s", "-1s"} {
			t.Run(name+"/"+value, func(t *testing.T) {
				validRuntimeEnv(t)
				t.Setenv(name, value)
				_, _, err := LoadFromEnv()
				require.Error(t, err)
			})
		}
	}
	t.Run("hard_expiry_before_refresh", func(t *testing.T) {
		validRuntimeEnv(t)
		t.Setenv("S2S_JWKS_HARD_EXPIRY", "29s")
		_, _, err := LoadFromEnv()
		require.Error(t, err)
	})
}
