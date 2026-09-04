package main

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRealtimeSessionEpochStrictConfigDefaultsToCompatibility(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value *string
	}{
		{name: "unset"},
		{name: "exact false", value: realtimeConfigString("false")},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			configureRealtimeSessionEpochEnv(t, tc.value, "", "", "", "")

			config, err := loadRealtimeConfigFromEnvChecked()
			require.NoError(t, err)
			require.False(t, config.sessionEpochStrict)
			require.Nil(t, config.sessionEpochFloor)
		})
	}
}

func TestRealtimeSessionEpochStrictConfigRejectsNonExactValues(t *testing.T) {
	for _, value := range []string{
		"", " true", "true ", " false", "false ",
		"TRUE", "True", "FALSE", "False", "1", "typo",
	} {
		value := value
		t.Run(value, func(t *testing.T) {
			configureRealtimeSessionEpochEnv(t, &value, "127.0.0.1:6379", "https://auth.example/jwks", "voice-auth", "voice-client")

			_, err := loadRealtimeConfigFromEnvChecked()
			require.Error(t, err)
		})
	}
}

func TestRealtimeSessionEpochStrictConfigFailsFastForMissingDependencies(t *testing.T) {
	strict := "true"
	for _, tc := range []struct {
		name      string
		redisAddr string
		jwksURL   string
		issuer    string
		audience  string
	}{
		{name: "missing Redis", jwksURL: "https://auth.example/jwks", issuer: "voice-auth", audience: "voice-client"},
		{name: "missing independent JWKS", redisAddr: "127.0.0.1:6379", issuer: "voice-auth", audience: "voice-client"},
		{name: "missing JWT issuer", redisAddr: "127.0.0.1:6379", jwksURL: "https://auth.example/jwks", audience: "voice-client"},
		{name: "missing JWT audience", redisAddr: "127.0.0.1:6379", jwksURL: "https://auth.example/jwks", issuer: "voice-auth"},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			configureRealtimeSessionEpochEnv(t, &strict, tc.redisAddr, tc.jwksURL, tc.issuer, tc.audience)

			_, err := loadRealtimeConfigFromEnvChecked()
			require.Error(t, err)
		})
	}
}

func TestRealtimeSessionEpochStrictConfigBuildsAuthOwnedFloorWithoutStartupPing(t *testing.T) {
	strict := "true"
	configureRealtimeSessionEpochEnv(t, &strict, "198.51.100.11:6379", "https://auth.example/jwks", "voice-auth", "voice-client")
	t.Setenv("REALTIME_REDIS_PASSWORD", "shared-password")

	config, err := loadRealtimeConfigFromEnvChecked()
	require.NoError(t, err)
	require.True(t, config.sessionEpochStrict)
	require.NotNil(t, config.tokenValidator, "strict mode must independently validate the upstream JWT")

	floor, ok := config.sessionEpochFloor.(*redisSessionEpochFloor)
	require.True(t, ok, "strict mode must construct the read-only Auth epoch floor")
	require.NotNil(t, floor)
	require.Equal(t, "198.51.100.11:6379", floor.addr)
	require.Equal(t, "shared-password", floor.password)
	require.Equal(t, sessionEpochFloorPrefix, floor.prefix)
	require.Equal(t, 2*time.Second, floor.timeout)
}

func configureRealtimeSessionEpochEnv(t *testing.T, strict *string, redisAddr, jwksURL, issuer, audience string) {
	t.Helper()
	t.Setenv("REALTIME_REDIS_ADDR", redisAddr)
	t.Setenv("REALTIME_REDIS_PASSWORD", "")
	t.Setenv("REALTIME_JWKS_URL", jwksURL)
	t.Setenv("GATEWAY_JWKS_URL", "")
	t.Setenv("REALTIME_JWT_ISSUER", issuer)
	t.Setenv("GATEWAY_JWT_ISSUER", "")
	t.Setenv("REALTIME_JWT_AUDIENCE", audience)
	t.Setenv("GATEWAY_JWT_AUDIENCE", "")
	if strict == nil {
		t.Setenv("REALTIME_SESSION_EPOCH_STRICT", "placeholder")
		require.NoError(t, os.Unsetenv("REALTIME_SESSION_EPOCH_STRICT"))
		return
	}
	t.Setenv("REALTIME_SESSION_EPOCH_STRICT", *strict)
}

func realtimeConfigString(value string) *string {
	return &value
}
