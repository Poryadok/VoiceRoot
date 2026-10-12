package spaceprincipalruntime

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

var lifecycleConfigNames = []string{
	"REPLAY_REDIS_ADDR", "REPLAY_REDIS_PASSWORD", "TLS_CERT_FILE",
	"TLS_KEY_FILE", "CLIENT_CA_FILE", "GRPC_LISTEN",
}

func isolateLifecycleConfig(t *testing.T) {
	t.Helper()
	for _, suffix := range lifecycleConfigNames {
		name := "VOICE_SPACE_LIFECYCLE_PRINCIPAL_" + suffix
		t.Setenv(name, "")
		require.NoError(t, os.Unsetenv(name))
	}
	t.Setenv("S2S_JWKS_URLS_JSON", `{"space":"https://space.test/jwks","gateway":"https://gateway.test/jwks"}`)
	t.Setenv("S2S_JWKS_CA_FILE", "/fixture/ca.crt")
	t.Setenv("S2S_JWKS_REFRESH_AFTER", "30s")
	t.Setenv("S2S_JWKS_HARD_EXPIRY", "2m")
	t.Setenv("S2S_UNKNOWN_KID_COOLDOWN", "5s")
}

func TestLoadFromEnvSharedJWKSDoesNotEnableLifecycle(t *testing.T) {
	isolateLifecycleConfig(t)
	_, enabled, err := LoadFromEnv()
	require.NoError(t, err)
	require.False(t, enabled)
}

func TestLoadFromEnvPartialLifecycleSettingsFailClosed(t *testing.T) {
	for _, suffix := range lifecycleConfigNames {
		t.Run(suffix, func(t *testing.T) {
			isolateLifecycleConfig(t)
			t.Setenv("VOICE_SPACE_LIFECYCLE_PRINCIPAL_"+suffix, "")
			_, enabled, err := LoadFromEnv()
			require.True(t, enabled)
			require.Error(t, err)
		})
	}
}

func TestLoadFromEnvCompleteLifecycleUsesSharedJWKS(t *testing.T) {
	isolateLifecycleConfig(t)
	for suffix, value := range map[string]string{
		"REPLAY_REDIS_ADDR": "redis.test:6379", "TLS_CERT_FILE": "/fixture/server.crt",
		"TLS_KEY_FILE": "/fixture/server.key", "CLIENT_CA_FILE": "/fixture/client-ca.crt",
		"GRPC_LISTEN": ":9092",
	} {
		t.Setenv("VOICE_SPACE_LIFECYCLE_PRINCIPAL_"+suffix, value)
	}
	cfg, enabled, err := LoadFromEnv()
	require.NoError(t, err)
	require.True(t, enabled)
	require.Equal(t, ":9092", cfg.ListenAddr)
	require.Equal(t, "https://space.test/jwks", cfg.JWKSURLs["space"])
	require.Equal(t, "https://gateway.test/jwks", cfg.JWKSURLs["gateway"])
	t.Setenv("S2S_JWKS_URLS_JSON", "")
	_, enabled, err = LoadFromEnv()
	require.True(t, enabled)
	require.Error(t, err)
}
