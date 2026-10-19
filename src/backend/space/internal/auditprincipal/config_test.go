package auditprincipal

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func clearAuditEnv(t *testing.T) {
	t.Helper()
	for _, name := range append(auditEnvNames, "S2S_JWKS_URLS_JSON", "S2S_JWKS_CA_FILE", "S2S_JWKS_CLIENT_CERT_FILE", "S2S_JWKS_CLIENT_KEY_FILE", "S2S_JWKS_REFRESH_AFTER", "S2S_JWKS_HARD_EXPIRY", "S2S_UNKNOWN_KID_COOLDOWN") {
		t.Setenv(name, "")
		require.NoError(t, os.Unsetenv(name))
	}
}

func validAuditEnv(t *testing.T) {
	t.Helper()
	clearAuditEnv(t)
	t.Setenv("SPACE_AUDIT_PRINCIPAL_TLS_CERT_FILE", "server.pem")
	t.Setenv("SPACE_AUDIT_PRINCIPAL_TLS_KEY_FILE", "server-key.pem")
	t.Setenv("SPACE_AUDIT_PRINCIPAL_CLIENT_CA_FILE", "clients.pem")
	t.Setenv("SPACE_AUDIT_PRINCIPAL_REPLAY_REDIS_ADDR", "localhost:6379")
	t.Setenv("S2S_JWKS_URLS_JSON", `{"role":"https://role.internal/.well-known/jwks.json","chat":"https://chat.internal/.well-known/jwks.json","social":"https://social.internal/keys"}`)
}

func TestAuditConfigDefaultsAndClosedIssuers(t *testing.T) {
	validAuditEnv(t)
	cfg, enabled, err := LoadFromEnv()
	require.NoError(t, err)
	require.True(t, enabled)
	require.Equal(t, ":9098", cfg.ListenAddr)
	require.Equal(t, 30*time.Second, cfg.RefreshAfter)
	require.Equal(t, 2*time.Minute, cfg.HardExpiry)
	require.Equal(t, 5*time.Second, cfg.UnknownKIDCooldown)
	require.Len(t, cfg.JWKSURLs, 2, "other global issuer endpoints do not grant audit authority")
	require.Contains(t, cfg.JWKSURLs, "role")
	require.Contains(t, cfg.JWKSURLs, "chat")
}

func TestAuditConfigDisabledOnlyWhenOwnedSettingsAbsent(t *testing.T) {
	clearAuditEnv(t)
	t.Setenv("S2S_JWKS_URLS_JSON", `{"social":"https://social.internal/keys"}`)
	_, enabled, err := LoadFromEnv()
	require.NoError(t, err)
	require.False(t, enabled)
	t.Setenv("SPACE_AUDIT_PRINCIPAL_GRPC_LISTEN", "")
	_, enabled, err = LoadFromEnv()
	require.True(t, enabled)
	require.Error(t, err, "present-empty cannot silently disable a partially configured listener")
}

func TestAuditConfigRejectsPartialAndUnsafeBindings(t *testing.T) {
	for _, tc := range []struct{ name, value string }{
		{"SPACE_AUDIT_PRINCIPAL_TLS_CERT_FILE", ""}, {"SPACE_AUDIT_PRINCIPAL_TLS_KEY_FILE", ""},
		{"SPACE_AUDIT_PRINCIPAL_CLIENT_CA_FILE", ""}, {"SPACE_AUDIT_PRINCIPAL_REPLAY_REDIS_ADDR", ""},
		{"SPACE_AUDIT_PRINCIPAL_GRPC_LISTEN", "not-a-host-port"},
		{"S2S_JWKS_URLS_JSON", `{"role":"https://role.internal/keys"}`},
		{"S2S_JWKS_URLS_JSON", `{"role":"http://role.internal/keys","chat":"https://chat.internal/keys"}`},
		{"S2S_JWKS_URLS_JSON", `{"role":"https://user:password@role.internal/keys","chat":"https://chat.internal/keys"}`},
		{"S2S_JWKS_URLS_JSON", `{"role":"https://role.internal/keys#fragment","chat":"https://chat.internal/keys"}`},
		{"S2S_JWKS_REFRESH_AFTER", ""}, {"S2S_JWKS_HARD_EXPIRY", "-1s"}, {"S2S_UNKNOWN_KID_COOLDOWN", "invalid"},
		{"S2S_JWKS_CLIENT_CERT_FILE", "orphan.pem"},
	} {
		t.Run(tc.name+tc.value, func(t *testing.T) {
			validAuditEnv(t)
			t.Setenv(tc.name, tc.value)
			_, enabled, err := LoadFromEnv()
			require.True(t, enabled)
			require.Error(t, err)
		})
	}
	validAuditEnv(t)
	t.Setenv("S2S_JWKS_REFRESH_AFTER", "3m")
	_, _, err := LoadFromEnv()
	require.Error(t, err, "hard expiry must not precede refresh")
}
