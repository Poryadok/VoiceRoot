package config

import (
	"log/slog"
	"os"
	"os/exec"
	"strings"
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
	t.Setenv("ANALYTICS_REQUIRE_PERSISTENCE", "true")
	t.Setenv("ANALYTICS_ID_HASH_KEY", "configured-test-hash-key")
	got := ResolveHashKey(slog.Default())
	require.Equal(t, "configured-test-hash-key", got)
}

func TestResolveHashKeyRequiresConfiguredKeyInPersistenceMode(t *testing.T) {
	for _, tc := range []struct {
		name      string
		key       string
		fatalText string
	}{
		{
			name:      "missing key",
			key:       "",
			fatalText: "ANALYTICS_ID_HASH_KEY required when ANALYTICS_REQUIRE_PERSISTENCE=true",
		},
		{
			name:      "development default",
			key:       DevHashKeyDefault,
			fatalText: "ANALYTICS_ID_HASH_KEY must not use dev default when ANALYTICS_REQUIRE_PERSISTENCE=true",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command(os.Args[0], "-test.run=^TestResolveHashKeyFatalHelper$")
			cmd.Env = envWithout("ANALYTICS_REQUIRE_PERSISTENCE", "ANALYTICS_ID_HASH_KEY", "ANALYTICS_CONFIG_FATAL_HELPER")
			cmd.Env = append(cmd.Env,
				"ANALYTICS_CONFIG_FATAL_HELPER=1",
				"ANALYTICS_REQUIRE_PERSISTENCE=true",
				"ANALYTICS_ID_HASH_KEY="+tc.key,
			)

			output, err := cmd.CombinedOutput()
			require.Error(t, err, "ResolveHashKey should terminate for %s", tc.name)
			require.Contains(t, string(output), tc.fatalText)
		})
	}
}

func TestResolveHashKeyFatalHelper(t *testing.T) {
	if os.Getenv("ANALYTICS_CONFIG_FATAL_HELPER") != "1" {
		t.Skip("helper for fatal-path subprocess test")
	}
	ResolveHashKey(nil)
	os.Exit(0)
}

func envWithout(names ...string) []string {
	wanted := make(map[string]struct{}, len(names))
	for _, name := range names {
		wanted[name] = struct{}{}
	}

	env := make([]string, 0, len(os.Environ()))
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if _, exists := wanted[name]; !exists {
			env = append(env, entry)
		}
	}
	return env
}
