package grpcsvc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBotPinCleanupConfigurationHasNoOrdinaryFallback(t *testing.T) {
	legacy := map[string]string{"BOT_PRINCIPAL_JWKS_URL": "https://old.invalid/.well-known/principal-jwks.json", "MESSAGING_PRINCIPAL_REPLAY_REDIS_URL": "redis://old.invalid:6379"}
	config := BotPinCleanupConfigFromEnv(func(name string) string { return legacy[name] })
	server, err := StartBotPinCleanupServer(context.Background(), config, nil)
	require.NoError(t, err)
	require.Nil(t, server)
	suffixes := []string{"GRPC_LISTEN", "TLS_CERT_FILE", "TLS_KEY_FILE", "CLIENT_CA_FILE", "JWKS_URL", "JWKS_CA_FILE", "JWKS_CLIENT_CERT_FILE", "JWKS_CLIENT_KEY_FILE", "REPLAY_REDIS_URL"}
	for _, suffix := range suffixes {
		t.Run(suffix, func(t *testing.T) {
			partial := BotPinCleanupConfigFromEnv(func(name string) string {
				if name == "MESSAGING_BOT_PIN_CLEANUP_"+suffix {
					return "partial-only"
				}
				return ""
			})
			server, err := StartBotPinCleanupServer(context.Background(), partial, nil)
			require.Error(t, err)
			require.Nil(t, server, "partial configuration must never open a listener")
		})
	}
}
