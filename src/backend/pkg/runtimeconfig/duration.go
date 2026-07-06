package runtimeconfig

import (
	"os"
	"strings"
	"time"
)

// DurationFromEnv parses key as a Go duration string. Empty or invalid values
// return fallback. Zero is allowed (e.g. HTTP read/write timeouts disabled).
func DurationFromEnv(key string, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return fallback
	}
	return d
}

func durationFromEnvPositive(key string, fallback time.Duration) time.Duration {
	d := DurationFromEnv(key, fallback)
	if d <= 0 {
		return fallback
	}
	return d
}

// HTTPServerTimeouts holds standard net/http.Server timeout fields from env.
type HTTPServerTimeouts struct {
	ReadHeader time.Duration
	Read       time.Duration
	Write      time.Duration
	Idle       time.Duration
}

// HTTPServerTimeoutsFromEnv reads HTTP_READ_HEADER_TIMEOUT, HTTP_READ_TIMEOUT,
// HTTP_WRITE_TIMEOUT, and HTTP_IDLE_TIMEOUT with service defaults.
func HTTPServerTimeoutsFromEnv() HTTPServerTimeouts {
	return HTTPServerTimeouts{
		ReadHeader: durationFromEnvPositive("HTTP_READ_HEADER_TIMEOUT", 5*time.Second),
		Read:       DurationFromEnv("HTTP_READ_TIMEOUT", 30*time.Second),
		Write:      DurationFromEnv("HTTP_WRITE_TIMEOUT", 60*time.Second),
		Idle:       durationFromEnvPositive("HTTP_IDLE_TIMEOUT", 120*time.Second),
	}
}

// ShutdownTimeoutFromEnv returns HTTP_SHUTDOWN_TIMEOUT or 10s default.
func ShutdownTimeoutFromEnv() time.Duration {
	return durationFromEnvPositive("HTTP_SHUTDOWN_TIMEOUT", 10*time.Second)
}

// PostgresConnectTimeoutFromEnv returns POSTGRES_CONNECT_TIMEOUT or 15s default.
func PostgresConnectTimeoutFromEnv() time.Duration {
	return durationFromEnvPositive("POSTGRES_CONNECT_TIMEOUT", 15*time.Second)
}
