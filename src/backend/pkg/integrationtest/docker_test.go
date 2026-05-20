package integrationtest

import (
	"os"
	"runtime"
	"testing"
)

func TestConfigureDockerTesting_windowsDisablesRyuk(t *testing.T) {
	key := "TESTCONTAINERS_RYUK_DISABLED"
	prev, had := os.LookupEnv(key)
	t.Cleanup(func() {
		if had {
			_ = os.Setenv(key, prev)
		} else {
			_ = os.Unsetenv(key)
		}
	})
	_ = os.Unsetenv(key)

	ConfigureDockerTesting()

	if runtime.GOOS == "windows" {
		if os.Getenv(key) != "true" {
			t.Fatalf("expected %s=true on windows, got %q", key, os.Getenv(key))
		}
	} else if os.Getenv(key) != "" {
		t.Fatalf("expected %s unset on %s, got %q", key, runtime.GOOS, os.Getenv(key))
	}
}
