package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadConfigRejectsMissingAuthorityAndDatabase(t *testing.T) {
	_, err := loadConfig(func(string) string { return "" })
	require.Error(t, err)

	values := map[string]string{
		"DATABASE_URL":                  "postgres://game@localhost/game_integration_db",
		"GAME_INTEGRATION_JWKS_URL":     "https://auth.example/jwks",
		"GAME_INTEGRATION_JWT_ISSUER":   "https://auth.example",
		"GAME_INTEGRATION_JWT_AUDIENCE": "voice",
	}
	_, err = loadConfig(func(name string) string { return values[name] })
	require.ErrorContains(t, err, "GAME_INTEGRATION_REDIS_ADDR")

	values["GAME_INTEGRATION_REDIS_ADDR"] = "localhost:6379"
	config, err := loadConfig(func(name string) string { return values[name] })
	require.NoError(t, err)
	require.Equal(t, ":8080", config.ListenAddr)
	require.Equal(t, values["DATABASE_URL"], config.DatabaseURL)
}
