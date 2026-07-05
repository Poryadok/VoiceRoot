// Package integrationtest centralizes Docker/testcontainers setup for backend integration tests.
package integrationtest

import (
	"os"
	"runtime"
)

func init() {
	ConfigureDockerTesting()
}

// ConfigureDockerTesting adjusts testcontainers for the host OS and CI runners.
// Import this package with a blank import in integration test packages, or call from TestMain.
func ConfigureDockerTesting() {
	if os.Getenv("TESTCONTAINERS_RYUK_DISABLED") != "" {
		return
	}
	// Ryuk sidecar can fail on Docker Desktop for Windows ("no port to wait for")
	// and in CI when pulling testcontainers/ryuk from Docker Hub fails (rate limits, network).
	if runtime.GOOS == "windows" || os.Getenv("CI") != "" || os.Getenv("GITHUB_ACTIONS") == "true" {
		_ = os.Setenv("TESTCONTAINERS_RYUK_DISABLED", "true")
	}
}
