package chatcreation

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestCreateConfigRequiresEveryTrustDependency(t *testing.T) {
	names := []string{"GRPC_LISTEN", "TLS_CERT_FILE", "TLS_KEY_FILE", "CLIENT_CA_FILE", "PRINCIPAL_JWKS_URL", "PRINCIPAL_JWKS_CA_FILE", "PRINCIPAL_REDIS_ADDR", "PRINCIPAL_REDIS_PASSWORD", "PRINCIPAL_JWKS_CLIENT_CERT_FILE", "PRINCIPAL_JWKS_CLIENT_KEY_FILE", "BOT_PRINCIPAL_JWKS_URL"}
	for _, name := range names {
		t.Setenv("CHAT_CREATE_"+name, "")
	}
	_, enabled, err := CreateConfigFromEnv()
	require.NoError(t, err)
	require.False(t, enabled)
	for _, missing := range names {
		if missing == "PRINCIPAL_REDIS_PASSWORD" {
			continue
		}
		t.Run(missing, func(t *testing.T) {
			for _, name := range names {
				t.Setenv("CHAT_CREATE_"+name, "configured")
			}
			t.Setenv("CHAT_CREATE_PRINCIPAL_JWKS_URL", "https://gateway.example/jwks")
			t.Setenv("CHAT_CREATE_BOT_PRINCIPAL_JWKS_URL", "https://bot.example/jwks")
			t.Setenv("CHAT_CREATE_"+missing, "")
			_, enabled, err := CreateConfigFromEnv()
			require.True(t, enabled)
			require.Error(t, err)
		})
	}
}
