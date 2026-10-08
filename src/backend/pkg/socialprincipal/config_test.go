package socialprincipal

import (
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestConfigPartialAndDefaults(t *testing.T) {
	for _, target := range []string{"user", "space", "file"} {
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
			t.Setenv("S2S_JWKS_URLS_JSON", `{"`+expectedIssuer(target)+`":"https://social:8443/.well-known/jwks.json"}`)
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

func TestLoadFromEnvWithPrefix_IsolatesUserFileListener(t *testing.T) {
	t.Setenv("S2S_JWKS_URLS_JSON", `{"social":"https://social:8443/.well-known/jwks.json"}`)
	_, enabled, err := LoadFromEnvWithPrefix("file", "USER_FILE_PRINCIPAL_", ":9092")
	require.NoError(t, err)
	require.False(t, enabled, "shared S2S settings must not activate the File listener")

	t.Setenv("USER_FILE_PRINCIPAL_TLS_CERT_FILE", "file-cert.pem")
	_, enabled, err = LoadFromEnvWithPrefix("file", "USER_FILE_PRINCIPAL_", ":9092")
	require.Error(t, err, "target-owned partial configuration must fail closed")
	require.True(t, enabled)

	t.Setenv("USER_FILE_PRINCIPAL_TLS_KEY_FILE", "file-key.pem")
	t.Setenv("USER_FILE_PRINCIPAL_REPLAY_REDIS_ADDR", "redis:6379")
	t.Setenv("S2S_JWKS_URLS_JSON", `{"social":"https://social:8443/.well-known/jwks.json","file":"https://file:8443/.well-known/jwks.json"}`)
	cfg, enabled, err := LoadFromEnvWithPrefix("file", "USER_FILE_PRINCIPAL_", ":9092")
	require.NoError(t, err)
	require.True(t, enabled)
	require.Equal(t, ":9092", cfg.ListenAddr)
	require.Equal(t, map[string]string{"file": "https://file:8443/.well-known/jwks.json"}, cfg.JWKSURLs)
}

func TestLoadFromEnvWithAudience_ConfiguresNotificationOnlyCapability(t *testing.T) {
	const prefix = "USER_NOTIFICATION_PRINCIPAL_"
	t.Setenv("S2S_JWKS_URLS_JSON", `{"notification":"https://notification:8443/.well-known/jwks.json"}`)
	_, enabled, err := LoadFromEnvWithAudience("user", "notification", prefix, ":9095")
	require.NoError(t, err)
	require.False(t, enabled, "shared JWKS configuration must not enable the dedicated listener")

	t.Setenv(prefix+"TLS_CERT_FILE", "cert.pem")
	_, enabled, err = LoadFromEnvWithAudience("user", "notification", prefix, ":9095")
	require.Error(t, err, "partial listener config must fail closed")
	require.True(t, enabled)

	t.Setenv(prefix+"TLS_KEY_FILE", "key.pem")
	t.Setenv(prefix+"REPLAY_REDIS_ADDR", "redis:6379")
	cfg, enabled, err := LoadFromEnvWithAudience("user", "notification", prefix, ":9095")
	require.NoError(t, err)
	require.True(t, enabled)
	require.Equal(t, "user", cfg.Target)
	require.Equal(t, "notification", cfg.Capability)
	require.Equal(t, ":9095", cfg.ListenAddr)
	require.Equal(t, map[string]string{"notification": "https://notification:8443/.well-known/jwks.json"}, cfg.JWKSURLs)
}

func TestLoadFromEnvWithAudience_ConfiguresMessagingOnlyCapability(t *testing.T) {
	const prefix = "USER_MESSAGING_PRINCIPAL_"
	t.Setenv("S2S_JWKS_URLS_JSON", `{"messaging":"https://messaging:8443/.well-known/jwks.json","notification":"https://notification:8443/.well-known/jwks.json"}`)
	_, enabled, err := LoadFromEnvWithAudience("user", "messaging", prefix, ":9096")
	require.NoError(t, err)
	require.False(t, enabled, "shared JWKS configuration must not enable the dedicated listener")

	t.Setenv(prefix+"TLS_CERT_FILE", "cert.pem")
	_, enabled, err = LoadFromEnvWithAudience("user", "messaging", prefix, ":9096")
	require.Error(t, err, "partial listener config must fail closed")
	require.True(t, enabled)

	t.Setenv(prefix+"TLS_KEY_FILE", "key.pem")
	t.Setenv(prefix+"REPLAY_REDIS_ADDR", "redis:6379")
	_, enabled, err = LoadFromEnvWithAudience("user", "messaging", prefix, ":9096")
	require.Error(t, err, "the Messaging listener must not enable without a client CA")
	require.True(t, enabled)
	invalidCAFile := filepath.Join(t.TempDir(), "invalid-client-ca.pem")
	require.NoError(t, os.WriteFile(invalidCAFile, []byte("not a certificate bundle"), 0600))
	t.Setenv(prefix+"CLIENT_CA_FILE", invalidCAFile)
	_, enabled, err = LoadFromEnvWithAudience("user", "messaging", prefix, ":9096")
	require.Error(t, err, "an unreadable CA bundle must fail closed")
	require.True(t, enabled)
	_, clientCAPEM, _ := ephemeralTLS(t)
	clientCAFile := filepath.Join(t.TempDir(), "client-ca.pem")
	require.NoError(t, os.WriteFile(clientCAFile, clientCAPEM, 0600))
	t.Setenv(prefix+"CLIENT_CA_FILE", clientCAFile)
	_, enabled, err = LoadFromEnvWithAudience("user", "messaging", prefix, ":9096")
	require.Error(t, err, "the Messaging capability must require a separate outbound JWKS client identity")
	require.True(t, enabled)

	_, jwksClientCertPEM, jwksClientKeyPEM := ephemeralTLS(t)
	clientCertFile := filepath.Join(t.TempDir(), "jwks-client.crt")
	clientKeyFile := filepath.Join(t.TempDir(), "jwks-client.key")
	require.NoError(t, os.WriteFile(clientCertFile, jwksClientCertPEM, 0600))
	require.NoError(t, os.WriteFile(clientKeyFile, jwksClientKeyPEM, 0600))
	t.Setenv("USER_PRINCIPAL_JWKS_CLIENT_CERT_FILE", clientCertFile)
	t.Setenv("USER_PRINCIPAL_JWKS_CLIENT_KEY_FILE", clientKeyFile)
	cfg, enabled, err := LoadFromEnvWithAudience("user", "messaging", prefix, ":9096")
	require.NoError(t, err)
	require.True(t, enabled)
	require.Equal(t, "user", cfg.Target)
	require.Equal(t, "messaging", cfg.Capability)
	require.Equal(t, ":9096", cfg.ListenAddr)
	require.Equal(t, clientCAFile, cfg.ClientCAFile)
	require.NotNil(t, cfg.ClientCAs)
	require.Equal(t, clientCertFile, cfg.JWKSClientCertFile)
	require.Equal(t, clientKeyFile, cfg.JWKSClientKeyFile)
	require.Equal(t, map[string]string{"messaging": "https://messaging:8443/.well-known/jwks.json"}, cfg.JWKSURLs)
}

func TestConfigRejectsUnsafeEndpoint(t *testing.T) {
	for _, endpoint := range []string{"http://social/jwks", "https://user:pass@social/jwks", "https://social/jwks#fragment"} {
		cfg := Config{Target: "user", JWKSURLs: map[string]string{"social": endpoint}, RefreshAfter: 30 * time.Second, HardExpiry: 2 * time.Minute, UnknownKIDCooldown: 5 * time.Second, ReplayAddr: "redis:6379", TLSCertFile: "cert", TLSKeyFile: "key", ListenAddr: ":9091"}
		require.Error(t, cfg.validate())
	}
}
