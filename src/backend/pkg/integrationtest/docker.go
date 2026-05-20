// Package integrationtest centralizes Docker/testcontainers setup for backend integration tests.
package integrationtest

import (
	"os"
	"runtime"
)

func init() {
	ConfigureDockerTesting()
}

// ConfigureDockerTesting adjusts testcontainers for the host OS.
// Import this package with a blank import in integration test packages, or call from TestMain.
func ConfigureDockerTesting() {
	// Ryuk sidecar can fail on Docker Desktop for Windows ("no port to wait for").
	if runtime.GOOS == "windows" && os.Getenv("TESTCONTAINERS_RYUK_DISABLED") == "" {
		_ = os.Setenv("TESTCONTAINERS_RYUK_DISABLED", "true")
	}
}
