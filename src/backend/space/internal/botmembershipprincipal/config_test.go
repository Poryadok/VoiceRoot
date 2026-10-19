package botmembershipprincipal

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func clearBotMembershipEnv(t *testing.T) {
	t.Helper()
	for _, name := range append(botmembershipEnvNames, "S2S_JWKS_URLS_JSON", "S2S_JWKS_CA_FILE", "S2S_JWKS_CLIENT_CERT_FILE", "S2S_JWKS_CLIENT_KEY_FILE", "S2S_JWKS_REFRESH_AFTER", "S2S_JWKS_HARD_EXPIRY", "S2S_UNKNOWN_KID_COOLDOWN") {
		t.Setenv(name, "")
		require.NoError(t, os.Unsetenv(name))
	}
}

func validBotMembershipEnv(t *testing.T) {
	t.Helper()
	clearBotMembershipEnv(t)
	t.Setenv("SPACE_BOT_MEMBERSHIP_TLS_CERT_FILE", "server.pem")
	t.Setenv("SPACE_BOT_MEMBERSHIP_TLS_KEY_FILE", "server-key.pem")
	t.Setenv("SPACE_BOT_MEMBERSHIP_CLIENT_CA_FILE", "clients.pem")
	t.Setenv("SPACE_BOT_MEMBERSHIP_REPLAY_REDIS_ADDR", "localhost:6379")
	t.Setenv("S2S_JWKS_URLS_JSON", `{"bot":"https://bot.internal/.well-known/jwks.json","chat":"https://chat.internal/.well-known/jwks.json","social":"https://social.internal/keys"}`)
}

func TestBotMembershipConfigDefaultsAndClosedIssuers(t *testing.T) {
	validBotMembershipEnv(t)
	cfg, enabled, err := LoadFromEnv()
	require.NoError(t, err)
	require.True(t, enabled)
	require.Equal(t, ":9098", cfg.ListenAddr)
	require.Equal(t, 30*time.Second, cfg.RefreshAfter)
	require.Equal(t, 2*time.Minute, cfg.HardExpiry)
	require.Equal(t, 5*time.Second, cfg.UnknownKIDCooldown)
	require.Len(t, cfg.JWKSURLs, 1, "other global issuer endpoints do not grant botmembership authority")
	require.Contains(t, cfg.JWKSURLs, "bot")
	require.NotContains(t, cfg.JWKSURLs, "chat")
}

func TestBotMembershipConfigDisabledOnlyWhenOwnedSettingsAbsent(t *testing.T) {
	clearBotMembershipEnv(t)
	t.Setenv("S2S_JWKS_URLS_JSON", `{"social":"https://social.internal/keys"}`)
	_, enabled, err := LoadFromEnv()
	require.NoError(t, err)
	require.False(t, enabled)
	t.Setenv("SPACE_BOT_MEMBERSHIP_GRPC_LISTEN", "")
	_, enabled, err = LoadFromEnv()
	require.True(t, enabled)
	require.Error(t, err, "present-empty cannot silently disable a partially configured listener")
}

func TestBotMembershipConfigRejectsPartialAndUnsafeBindings(t *testing.T) {
	for _, tc := range []struct{ name, value string }{
		{"SPACE_BOT_MEMBERSHIP_TLS_CERT_FILE", ""}, {"SPACE_BOT_MEMBERSHIP_TLS_KEY_FILE", ""},
		{"SPACE_BOT_MEMBERSHIP_CLIENT_CA_FILE", ""}, {"SPACE_BOT_MEMBERSHIP_REPLAY_REDIS_ADDR", ""},
		{"SPACE_BOT_MEMBERSHIP_GRPC_LISTEN", "not-a-host-port"},
		{"S2S_JWKS_URLS_JSON", `{"chat":"https://chat.internal/keys"}`},
		{"S2S_JWKS_URLS_JSON", `{"bot":"http://bot.internal/keys","chat":"https://chat.internal/keys"}`},
		{"S2S_JWKS_URLS_JSON", `{"bot":"https://user:password@bot.internal/keys","chat":"https://chat.internal/keys"}`},
		{"S2S_JWKS_URLS_JSON", `{"bot":"https://bot.internal/keys#fragment","chat":"https://chat.internal/keys"}`},
		{"S2S_JWKS_REFRESH_AFTER", ""}, {"S2S_JWKS_HARD_EXPIRY", "-1s"}, {"S2S_UNKNOWN_KID_COOLDOWN", "invalid"},
		{"S2S_JWKS_CLIENT_CERT_FILE", "orphan.pem"},
	} {
		t.Run(tc.name+tc.value, func(t *testing.T) {
			validBotMembershipEnv(t)
			t.Setenv(tc.name, tc.value)
			_, enabled, err := LoadFromEnv()
			require.True(t, enabled)
			require.Error(t, err)
		})
	}
	validBotMembershipEnv(t)
	t.Setenv("S2S_JWKS_REFRESH_AFTER", "3m")
	_, _, err := LoadFromEnv()
	require.Error(t, err, "hard expiry must not precede refresh")
}
