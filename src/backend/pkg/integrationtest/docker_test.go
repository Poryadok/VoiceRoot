package integrationtest

import (
	"os"
	"runtime"
	"testing"
)

func TestConfigureDockerTesting_disablesRyukOnWindowsAndCI(t *testing.T) {
	key := "TESTCONTAINERS_RYUK_DISABLED"
	prev, had := os.LookupEnv(key)
	ciPrev, hadCI := os.LookupEnv("CI")
	gaPrev, hadGA := os.LookupEnv("GITHUB_ACTIONS")
	t.Cleanup(func() {
		if had {
			_ = os.Setenv(key, prev)
		} else {
			_ = os.Unsetenv(key)
		}
		if hadCI {
			_ = os.Setenv("CI", ciPrev)
		} else {
			_ = os.Unsetenv("CI")
		}
		if hadGA {
			_ = os.Setenv("GITHUB_ACTIONS", gaPrev)
		} else {
			_ = os.Unsetenv("GITHUB_ACTIONS")
		}
	})

	t.Run("local non-CI host", func(t *testing.T) {
		_ = os.Unsetenv(key)
		_ = os.Unsetenv("CI")
		_ = os.Unsetenv("GITHUB_ACTIONS")

		ConfigureDockerTesting()

		if runtime.GOOS == "windows" {
			if os.Getenv(key) != "true" {
				t.Fatalf("expected %s=true on windows, got %q", key, os.Getenv(key))
			}
			return
		}
		if _, err := os.Stat("/.dockerenv"); err == nil {
			if os.Getenv(key) != "true" {
				t.Fatalf("expected %s=true inside container, got %q", key, os.Getenv(key))
			}
			return
		}
		if os.Getenv(key) != "" {
			t.Fatalf("expected %s unset on local %s, got %q", key, runtime.GOOS, os.Getenv(key))
		}
	})

	t.Run("CI env", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("covered by local non-CI host case on windows")
		}
		_ = os.Unsetenv(key)
		_ = os.Unsetenv("GITHUB_ACTIONS")
		_ = os.Setenv("CI", "true")

		ConfigureDockerTesting()

		if os.Getenv(key) != "true" {
			t.Fatalf("expected %s=true when CI is set, got %q", key, os.Getenv(key))
		}
	})

	t.Run("GITHUB_ACTIONS env", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("covered by local non-CI host case on windows")
		}
		_ = os.Unsetenv(key)
		_ = os.Unsetenv("CI")
		_ = os.Setenv("GITHUB_ACTIONS", "true")

		ConfigureDockerTesting()

		if os.Getenv(key) != "true" {
			t.Fatalf("expected %s=true when GITHUB_ACTIONS is set, got %q", key, os.Getenv(key))
		}
	})
}

func TestConfigureTestcontainersHostOverride(t *testing.T) {
	t.Run("windows defaults to loopback", func(t *testing.T) {
		t.Setenv("TESTCONTAINERS_HOST_OVERRIDE", "")

		configureTestcontainersHost("windows")

		if got := os.Getenv("TESTCONTAINERS_HOST_OVERRIDE"); got != "127.0.0.1" {
			t.Fatalf("expected loopback host override, got %q", got)
		}
	})

	t.Run("explicit override is preserved", func(t *testing.T) {
		want := "host.docker.internal"
		t.Setenv("TESTCONTAINERS_HOST_OVERRIDE", want)

		configureTestcontainersHost("windows")

		if got := os.Getenv("TESTCONTAINERS_HOST_OVERRIDE"); got != want {
			t.Fatalf("expected explicit host override %q, got %q", want, got)
		}
	})

	t.Run("non-windows does not add an override", func(t *testing.T) {
		t.Setenv("TESTCONTAINERS_HOST_OVERRIDE", "")

		configureTestcontainersHost("linux")

		if got := os.Getenv("TESTCONTAINERS_HOST_OVERRIDE"); got != "" {
			t.Fatalf("expected no host override on linux, got %q", got)
		}
	})
}
