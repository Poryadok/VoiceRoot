package runtimeconfig

import (
	"os"
	"strings"
	"time"
)

const (
	defaultNudgeAfter = 15 * time.Minute
	defaultTimeout    = 30 * time.Minute
)

// SearchTiming holds runtime search timeout configuration.
type SearchTiming struct {
	NudgeAfter time.Duration
	Timeout    time.Duration
}

// LoadSearchTiming reads MATCHMAKING_SEARCH_NUDGE_AFTER and MATCHMAKING_SEARCH_TIMEOUT.
func LoadSearchTiming() SearchTiming {
	return SearchTiming{
		NudgeAfter: parseDurationEnv("MATCHMAKING_SEARCH_NUDGE_AFTER", defaultNudgeAfter),
		Timeout:    parseDurationEnv("MATCHMAKING_SEARCH_TIMEOUT", defaultTimeout),
	}
}

func parseDurationEnv(key string, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return fallback
	}
	return d
}
