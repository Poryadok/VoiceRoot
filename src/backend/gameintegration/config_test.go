package main

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/google/uuid"
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
	require.Empty(t, config.OperatorAccounts)
	operatorID := uuid.New()
	values["GAME_INTEGRATION_OPERATOR_ACCOUNT_IDS"] = operatorID.String()
	config, err = loadConfig(func(name string) string { return values[name] })
	require.NoError(t, err)
	require.Contains(t, config.OperatorAccounts, operatorID)
	values["GAME_INTEGRATION_OPERATOR_ACCOUNT_IDS"] = "not-a-uuid"
	_, err = loadConfig(func(name string) string { return values[name] })
	require.ErrorContains(t, err, "GAME_INTEGRATION_OPERATOR_ACCOUNT_IDS")
	values["GAME_INTEGRATION_OPERATOR_ACCOUNT_IDS"] = ""
	values["GAME_INTEGRATION_CREDENTIAL_KEY_B64"] = base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
	config, err = loadConfig(func(name string) string { return values[name] })
	require.NoError(t, err)
	require.Len(t, config.CredentialKey, 32)
	values["GAME_INTEGRATION_CREDENTIAL_KEY_B64"] = "short"
	_, err = loadConfig(func(name string) string { return values[name] })
	require.ErrorContains(t, err, "GAME_INTEGRATION_CREDENTIAL_KEY_B64")
	values["GAME_INTEGRATION_CREDENTIAL_KEY_B64"] = ""
	values["GAME_INTEGRATION_AUTH_WORKLOAD_KEY_B64"] = base64.StdEncoding.EncodeToString([]byte("abcdef0123456789abcdef0123456789"))
	config, err = loadConfig(func(name string) string { return values[name] })
	require.NoError(t, err)
	require.Len(t, config.AuthWorkloadKey, 32)
	values["GAME_INTEGRATION_AUTH_WORKLOAD_KEY_B64"] = "short"
	_, err = loadConfig(func(name string) string { return values[name] })
	require.ErrorContains(t, err, "GAME_INTEGRATION_AUTH_WORKLOAD_KEY_B64")
}

func TestLoadConfigRequiresBotProofURLAndDedicatedKeyTogether(t *testing.T) {
	values := map[string]string{
		"DATABASE_URL":                  "postgres://game@localhost/game_integration_db",
		"GAME_INTEGRATION_REDIS_ADDR":   "localhost:6379",
		"GAME_INTEGRATION_JWKS_URL":     "https://auth.example/jwks",
		"GAME_INTEGRATION_JWT_ISSUER":   "https://auth.example",
		"GAME_INTEGRATION_JWT_AUDIENCE": "voice",
	}
	getenv := func(name string) string { return values[name] }
	_, err := loadConfig(getenv)
	require.NoError(t, err, "the endpoint remains fail closed while optional service wiring is absent")

	values["BOT_INTERNAL_URL"] = "http://bot:8080"
	_, err = loadConfig(getenv)
	require.ErrorContains(t, err, "must be configured together")
	values["BOT_INTERNAL_URL"] = ""
	values["GAME_INTEGRATION_BOT_WORKLOAD_KEY_B64"] = base64.StdEncoding.EncodeToString([]byte(strings.Repeat("k", 32)))
	_, err = loadConfig(getenv)
	require.ErrorContains(t, err, "must be configured together")

	values["BOT_INTERNAL_URL"] = "http://bot:8080"
	config, err := loadConfig(getenv)
	require.NoError(t, err)
	require.Equal(t, "http://bot:8080", config.BotAuthorityURL)
	require.Len(t, config.BotWorkloadKey, 32)

	values["GAME_INTEGRATION_BOT_WORKLOAD_KEY_B64"] = "not-base64"
	_, err = loadConfig(getenv)
	require.ErrorContains(t, err, "GAME_INTEGRATION_BOT_WORKLOAD_KEY_B64")
}
