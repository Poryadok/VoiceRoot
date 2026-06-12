package runtimeconfig

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLoadSearchTiming_Defaults(t *testing.T) {
	t.Setenv("MATCHMAKING_SEARCH_NUDGE_AFTER", "")
	t.Setenv("MATCHMAKING_SEARCH_TIMEOUT", "")
	timing := LoadSearchTiming()
	require.Equal(t, 15*time.Minute, timing.NudgeAfter)
	require.Equal(t, 30*time.Minute, timing.Timeout)
}

func TestLoadSearchTiming_EnvOverride(t *testing.T) {
	t.Setenv("MATCHMAKING_SEARCH_NUDGE_AFTER", "10s")
	t.Setenv("MATCHMAKING_SEARCH_TIMEOUT", "20s")
	timing := LoadSearchTiming()
	require.Equal(t, 10*time.Second, timing.NudgeAfter)
	require.Equal(t, 20*time.Second, timing.Timeout)
}
