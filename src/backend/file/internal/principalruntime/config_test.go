package principalruntime

import (
	"github.com/stretchr/testify/require"
	"os"
	"testing"
	"time"
)

func clearPrincipalEnv(t *testing.T) {
	t.Helper()
	for _, name := range []string{"FILE_PRINCIPAL_GRPC_LISTEN", "FILE_PRINCIPAL_TLS_CERT_FILE", "FILE_PRINCIPAL_TLS_KEY_FILE", "FILE_PRINCIPAL_REPLAY_REDIS_ADDR", "FILE_PRINCIPAL_REPLAY_REDIS_PASSWORD", "S2S_JWKS_URLS_JSON", "S2S_JWKS_REFRESH_AFTER", "S2S_JWKS_HARD_EXPIRY", "S2S_UNKNOWN_KID_COOLDOWN", "S2S_JWKS_CA_FILE"} {
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
func setValidPrincipalEnv(t *testing.T) {
	t.Setenv("FILE_PRINCIPAL_GRPC_LISTEN", ":9091")
	t.Setenv("FILE_PRINCIPAL_TLS_CERT_FILE", "cert.pem")
	t.Setenv("FILE_PRINCIPAL_TLS_KEY_FILE", "key.pem")
	t.Setenv("FILE_PRINCIPAL_REPLAY_REDIS_ADDR", "redis:6379")
	t.Setenv("S2S_JWKS_URLS_JSON", `{"story":"https://story.internal/.well-known/jwks.json"}`)
}
func TestLoadFromEnvDisabledOnlyWhenEverySettingIsAbsent(t *testing.T) {
	clearPrincipalEnv(t)
	_, enabled, err := LoadFromEnv()
	require.NoError(t, err)
	require.False(t, enabled)
}
func TestLoadFromEnvRequiresCompleteTLSReplayAndStoryTrust(t *testing.T) {
	for _, missing := range []string{"FILE_PRINCIPAL_GRPC_LISTEN", "FILE_PRINCIPAL_TLS_CERT_FILE", "FILE_PRINCIPAL_TLS_KEY_FILE", "FILE_PRINCIPAL_REPLAY_REDIS_ADDR", "S2S_JWKS_URLS_JSON"} {
		t.Run(missing, func(t *testing.T) {
			clearPrincipalEnv(t)
			setValidPrincipalEnv(t)
			t.Setenv(missing, "")
			_, enabled, err := LoadFromEnv()
			require.True(t, enabled)
			require.Error(t, err)
		})
	}
}
func TestLoadFromEnvUsesCanonicalCacheDefaultsAndRejectsInvalidValues(t *testing.T) {
	clearPrincipalEnv(t)
	setValidPrincipalEnv(t)
	cfg, enabled, err := LoadFromEnv()
	require.NoError(t, err)
	require.True(t, enabled)
	require.Equal(t, 30*time.Second, cfg.RefreshAfter)
	require.Equal(t, 2*time.Minute, cfg.HardExpiry)
	require.Equal(t, 5*time.Second, cfg.UnknownKIDCooldown)
	for _, name := range []string{"S2S_JWKS_REFRESH_AFTER", "S2S_JWKS_HARD_EXPIRY", "S2S_UNKNOWN_KID_COOLDOWN"} {
		for _, value := range []string{"", "no-duration", "0s", "-1s"} {
			t.Run(name+"="+value, func(t *testing.T) { t.Setenv(name, value); _, _, err := LoadFromEnv(); require.Error(t, err) })
		}
	}
}
func TestConfigRequiresStoryHTTPSAndValidatesEveryConfiguredIssuer(t *testing.T) {
	cfg := Config{JWKSURLs: map[string]string{"story": "https://story.internal/.well-known/jwks.json"}, RefreshAfter: 30 * time.Second, HardExpiry: 2 * time.Minute, UnknownKIDCooldown: 5 * time.Second, ReplayAddr: "redis:6379", TLSCertFile: "cert.pem", TLSKeyFile: "key.pem", ListenAddr: ":9091"}
	require.NoError(t, cfg.validate())
	cfg.JWKSURLs["gateway"] = "https://gateway.internal/jwks"
	require.NoError(t, cfg.validate(), "additional trusted caller can be verified then denied by method policy")
	for _, endpoint := range []string{"http://story.internal/jwks", "https://user:password@story.internal/jwks", "https://story.internal/jwks#fragment", ""} {
		cfg.JWKSURLs["story"] = endpoint
		require.Error(t, cfg.validate())
	}
	delete(cfg.JWKSURLs, "story")
	require.Error(t, cfg.validate(), "Story trust is required")
}
