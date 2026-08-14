package config

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRequirePersistence(t *testing.T) {
	t.Setenv("ANALYTICS_REQUIRE_PERSISTENCE", "true")
	require.True(t, RequirePersistence())
	t.Setenv("ANALYTICS_REQUIRE_PERSISTENCE", "0")
	require.False(t, RequirePersistence())
}

func TestResolveHashKeyDevDefault(t *testing.T) {
	t.Setenv("ANALYTICS_REQUIRE_PERSISTENCE", "")
	t.Setenv("ANALYTICS_ID_HASH_KEY", "")
	got := ResolveHashKey(slog.Default())
	require.Equal(t, DevHashKeyDefault, got)
}

func TestResolveHashKeyExplicit(t *testing.T) {
	t.Setenv("ANALYTICS_REQUIRE_PERSISTENCE", "")
	t.Setenv("ANALYTICS_ID_HASH_KEY", "prod-key")
	got := ResolveHashKey(slog.Default())
	require.Equal(t, "prod-key", got)
}
