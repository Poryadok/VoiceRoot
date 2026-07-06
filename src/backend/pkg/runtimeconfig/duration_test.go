package runtimeconfig_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"voice/backend/pkg/runtimeconfig"
)

func TestDurationFromEnv_emptyUsesFallback(t *testing.T) {
	t.Setenv("TEST_DURATION", "")
	require.Equal(t, 5*time.Second, runtimeconfig.DurationFromEnv("TEST_DURATION", 5*time.Second))
}

func TestDurationFromEnv_invalidUsesFallback(t *testing.T) {
	t.Setenv("TEST_DURATION", "not-a-duration")
	require.Equal(t, 5*time.Second, runtimeconfig.DurationFromEnv("TEST_DURATION", 5*time.Second))
}

func TestDurationFromEnv_valid(t *testing.T) {
	t.Setenv("TEST_DURATION", "12s")
	require.Equal(t, 12*time.Second, runtimeconfig.DurationFromEnv("TEST_DURATION", 5*time.Second))
}

func TestDurationFromEnv_allowsZero(t *testing.T) {
	t.Setenv("TEST_DURATION", "0")
	require.Equal(t, time.Duration(0), runtimeconfig.DurationFromEnv("TEST_DURATION", 30*time.Second))
}

func TestHTTPServerTimeoutsFromEnv_defaults(t *testing.T) {
	t.Setenv("HTTP_READ_HEADER_TIMEOUT", "")
	t.Setenv("HTTP_READ_TIMEOUT", "")
	t.Setenv("HTTP_WRITE_TIMEOUT", "")
	t.Setenv("HTTP_IDLE_TIMEOUT", "")

	got := runtimeconfig.HTTPServerTimeoutsFromEnv()
	require.Equal(t, 5*time.Second, got.ReadHeader)
	require.Equal(t, 30*time.Second, got.Read)
	require.Equal(t, 60*time.Second, got.Write)
	require.Equal(t, 120*time.Second, got.Idle)
}

func TestHTTPServerTimeoutsFromEnv_custom(t *testing.T) {
	t.Setenv("HTTP_READ_HEADER_TIMEOUT", "2s")
	t.Setenv("HTTP_READ_TIMEOUT", "0")
	t.Setenv("HTTP_WRITE_TIMEOUT", "0")
	t.Setenv("HTTP_IDLE_TIMEOUT", "90s")

	got := runtimeconfig.HTTPServerTimeoutsFromEnv()
	require.Equal(t, 2*time.Second, got.ReadHeader)
	require.Equal(t, time.Duration(0), got.Read)
	require.Equal(t, time.Duration(0), got.Write)
	require.Equal(t, 90*time.Second, got.Idle)
}

func TestShutdownTimeoutFromEnv_default(t *testing.T) {
	t.Setenv("HTTP_SHUTDOWN_TIMEOUT", "")
	require.Equal(t, 10*time.Second, runtimeconfig.ShutdownTimeoutFromEnv())
}

func TestShutdownTimeoutFromEnv_custom(t *testing.T) {
	t.Setenv("HTTP_SHUTDOWN_TIMEOUT", "25s")
	require.Equal(t, 25*time.Second, runtimeconfig.ShutdownTimeoutFromEnv())
}

func TestPostgresConnectTimeoutFromEnv_default(t *testing.T) {
	t.Setenv("POSTGRES_CONNECT_TIMEOUT", "")
	require.Equal(t, 15*time.Second, runtimeconfig.PostgresConnectTimeoutFromEnv())
}

func TestPostgresConnectTimeoutFromEnv_custom(t *testing.T) {
	t.Setenv("POSTGRES_CONNECT_TIMEOUT", "30s")
	require.Equal(t, 30*time.Second, runtimeconfig.PostgresConnectTimeoutFromEnv())
}
