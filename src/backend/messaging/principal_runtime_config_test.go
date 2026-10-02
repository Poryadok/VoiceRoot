package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSharedReplayRedisDoesNotEnableUnconfiguredIssuerRuntimes(t *testing.T) {
	t.Setenv("MESSAGING_PRINCIPAL_REPLAY_REDIS_URL", "redis://localhost:6379/0")
	t.Setenv("MODERATION_PRINCIPAL_JWKS_URL", "")
	t.Setenv("MESSAGING_PRINCIPAL_TLS_CERT_FILE", "")
	t.Setenv("MESSAGING_PRINCIPAL_TLS_KEY_FILE", "")
	t.Setenv("MODERATION_PRINCIPAL_JWKS_CA_FILE", "")
	t.Setenv("GATEWAY_PRINCIPAL_JWKS_URL", "")
	t.Setenv("MESSAGING_GATEWAY_PRINCIPAL_TLS_CERT_FILE", "")
	t.Setenv("MESSAGING_GATEWAY_PRINCIPAL_TLS_KEY_FILE", "")
	t.Setenv("GATEWAY_PRINCIPAL_JWKS_CA_FILE", "")
	t.Setenv("BOT_PRINCIPAL_JWKS_URL", "")
	t.Setenv("MESSAGING_BOT_PRINCIPAL_TLS_CERT_FILE", "")
	t.Setenv("MESSAGING_BOT_PRINCIPAL_TLS_KEY_FILE", "")
	t.Setenv("BOT_PRINCIPAL_JWKS_CA_FILE", "")
	t.Setenv("GAME_INTEGRATION_PRINCIPAL_JWKS_URL", "")
	t.Setenv("MESSAGING_GAME_INTEGRATION_JWKS_TLS_CERT_FILE", "")
	t.Setenv("MESSAGING_GAME_INTEGRATION_JWKS_TLS_KEY_FILE", "")
	t.Setenv("GAME_INTEGRATION_PRINCIPAL_JWKS_CA_FILE", "")

	for _, load := range []struct {
		name string
		fn   func(context.Context) (any, error)
	}{
		{"moderation", func(ctx context.Context) (any, error) { return loadModerationPrincipalRuntime(ctx) }},
		{"gateway", func(ctx context.Context) (any, error) { return loadGatewayPrincipalRuntime(ctx) }},
		{"bot", func(ctx context.Context) (any, error) { return loadBotPrincipalRuntime(ctx) }},
	} {
		t.Run(load.name, func(t *testing.T) {
			runtime, err := load.fn(context.Background())
			require.NoError(t, err)
			require.Nil(t, runtime)
		})
	}
}
