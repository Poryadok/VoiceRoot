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
	for _, value := range []string{"1", "true", "yes", " TRUE "} {
		t.Setenv("ANALYTICS_REQUIRE_PERSISTENCE", value)
		require.True(t, RequirePersistence(), value)
	}
	for _, value := range []string{"", "0", "false", "no", " FALSE "} {
		t.Setenv("ANALYTICS_REQUIRE_PERSISTENCE", value)
		require.False(t, RequirePersistence(), value)
	}
}

func TestPersistenceConfigurationFatal(t *testing.T) {
	for _, tc := range []struct{ name, mode, dsn, want string }{
		{"missing DSN", "true", "", "CLICKHOUSE_DSN required"},
		{"blank DSN", "yes", " \t ", "CLICKHOUSE_DSN required"},
		{"numeric required mode", "1", "", "CLICKHOUSE_DSN required"},
		{"misspelled mode", "ture", "", "ANALYTICS_REQUIRE_PERSISTENCE must be"},
		{"invalid mode with DSN", "enabled", "clickhouse://localhost:9000/voice", "ANALYTICS_REQUIRE_PERSISTENCE must be"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command(os.Args[0], "-test.run=^TestPersistenceConfigurationFatalHelper$")
			cmd.Env = append(envWithout("ANALYTICS_REQUIRE_PERSISTENCE", "CLICKHOUSE_DSN", "ANALYTICS_DSN_FATAL_HELPER"),
				"ANALYTICS_DSN_FATAL_HELPER=1", "ANALYTICS_REQUIRE_PERSISTENCE="+tc.mode, "CLICKHOUSE_DSN="+tc.dsn)
			output, err := cmd.CombinedOutput()
			require.Error(t, err)
			require.Contains(t, string(output), tc.want)
		})
	}
}

func TestPersistenceConfigurationFatalHelper(t *testing.T) {
	if os.Getenv("ANALYTICS_DSN_FATAL_HELPER") != "1" {
		t.Skip("fatal-path subprocess helper")
	}
	RequirePersistence()
	ResolveClickHouseDSN(nil)
	os.Exit(0)
}

func TestResolveClickHouseDSN(t *testing.T) {
	t.Setenv("ANALYTICS_REQUIRE_PERSISTENCE", "true")
	t.Setenv("CLICKHOUSE_DSN", "  clickhouse://localhost:9000/voice  ")
	require.Equal(t, "clickhouse://localhost:9000/voice", ResolveClickHouseDSN(nil))
	t.Setenv("ANALYTICS_REQUIRE_PERSISTENCE", "false")
	t.Setenv("CLICKHOUSE_DSN", "")
	require.Empty(t, ResolveClickHouseDSN(nil))
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
