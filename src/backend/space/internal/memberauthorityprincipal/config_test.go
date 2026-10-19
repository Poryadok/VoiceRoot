package memberauthorityprincipal

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func clearMemberAuthorityEnv(t *testing.T) {
	t.Helper()
	for _, name := range append(memberauthorityEnvNames, "S2S_JWKS_URLS_JSON", "S2S_JWKS_CA_FILE", "S2S_JWKS_CLIENT_CERT_FILE", "S2S_JWKS_CLIENT_KEY_FILE", "S2S_JWKS_REFRESH_AFTER", "S2S_JWKS_HARD_EXPIRY", "S2S_UNKNOWN_KID_COOLDOWN") {
		t.Setenv(name, "")
		require.NoError(t, os.Unsetenv(name))
	}
}

func validMemberAuthorityEnv(t *testing.T) {
	t.Helper()
	clearMemberAuthorityEnv(t)
	t.Setenv("SPACE_MEMBER_AUTHORITY_TLS_CERT_FILE", "server.pem")
	t.Setenv("SPACE_MEMBER_AUTHORITY_TLS_KEY_FILE", "server-key.pem")
	t.Setenv("SPACE_MEMBER_AUTHORITY_CLIENT_CA_FILE", "clients.pem")
	t.Setenv("SPACE_MEMBER_AUTHORITY_REPLAY_REDIS_ADDR", "localhost:6379")
	t.Setenv("S2S_JWKS_URLS_JSON", `{"role":"https://role.internal/.well-known/jwks.json","chat":"https://chat.internal/.well-known/jwks.json","social":"https://social.internal/keys"}`)
}

func TestMemberAuthorityConfigDefaultsAndClosedIssuers(t *testing.T) {
	validMemberAuthorityEnv(t)
	cfg, enabled, err := LoadFromEnv()
	require.NoError(t, err)
	require.True(t, enabled)
	require.Equal(t, ":9096", cfg.ListenAddr)
	require.Equal(t, 30*time.Second, cfg.RefreshAfter)
	require.Equal(t, 2*time.Minute, cfg.HardExpiry)
	require.Equal(t, 5*time.Second, cfg.UnknownKIDCooldown)
	require.Len(t, cfg.JWKSURLs, 1, "other global issuer endpoints do not grant memberauthority authority")
	require.Contains(t, cfg.JWKSURLs, "role")
	require.NotContains(t, cfg.JWKSURLs, "chat")
}

func TestMemberAuthorityConfigDisabledOnlyWhenOwnedSettingsAbsent(t *testing.T) {
	clearMemberAuthorityEnv(t)
	t.Setenv("S2S_JWKS_URLS_JSON", `{"social":"https://social.internal/keys"}`)
	_, enabled, err := LoadFromEnv()
	require.NoError(t, err)
	require.False(t, enabled)
	t.Setenv("SPACE_MEMBER_AUTHORITY_GRPC_LISTEN", "")
	_, enabled, err = LoadFromEnv()
	require.True(t, enabled)
	require.Error(t, err, "present-empty cannot silently disable a partially configured listener")
}

func TestMemberAuthorityConfigRejectsPartialAndUnsafeBindings(t *testing.T) {
	for _, tc := range []struct{ name, value string }{
		{"SPACE_MEMBER_AUTHORITY_TLS_CERT_FILE", ""}, {"SPACE_MEMBER_AUTHORITY_TLS_KEY_FILE", ""},
		{"SPACE_MEMBER_AUTHORITY_CLIENT_CA_FILE", ""}, {"SPACE_MEMBER_AUTHORITY_REPLAY_REDIS_ADDR", ""},
		{"SPACE_MEMBER_AUTHORITY_GRPC_LISTEN", "not-a-host-port"},
		{"S2S_JWKS_URLS_JSON", `{"chat":"https://chat.internal/keys"}`},
		{"S2S_JWKS_URLS_JSON", `{"role":"http://role.internal/keys","chat":"https://chat.internal/keys"}`},
		{"S2S_JWKS_URLS_JSON", `{"role":"https://user:password@role.internal/keys","chat":"https://chat.internal/keys"}`},
		{"S2S_JWKS_URLS_JSON", `{"role":"https://role.internal/keys#fragment","chat":"https://chat.internal/keys"}`},
		{"S2S_JWKS_REFRESH_AFTER", ""}, {"S2S_JWKS_HARD_EXPIRY", "-1s"}, {"S2S_UNKNOWN_KID_COOLDOWN", "invalid"},
		{"S2S_JWKS_CLIENT_CERT_FILE", "orphan.pem"},
	} {
		t.Run(tc.name+tc.value, func(t *testing.T) {
			validMemberAuthorityEnv(t)
			t.Setenv(tc.name, tc.value)
			_, enabled, err := LoadFromEnv()
			require.True(t, enabled)
			require.Error(t, err)
		})
	}
	validMemberAuthorityEnv(t)
	t.Setenv("S2S_JWKS_REFRESH_AFTER", "3m")
	_, _, err := LoadFromEnv()
	require.Error(t, err, "hard expiry must not precede refresh")
}
