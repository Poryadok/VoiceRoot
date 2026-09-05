package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestComposeWiringUserNATS_yaml documents the User publisher's Compose wiring
// for the shared user.events JetStream stream.
func TestComposeWiringUserNATS_yaml(t *testing.T) {
	t.Parallel()

	root := repoRootFromComposeTest(t)
	compose := readComposeYAMLForUserTest(t, root)
	userService := serviceBlock(compose, "user")

	t.Run("publisher environment", func(t *testing.T) {
		require.Contains(t, userService, "NATS_URL: nats://nats:4222")
	})
	t.Run("NATS health dependency", func(t *testing.T) {
		require.Contains(t, userService, "    depends_on:\n")
		require.Contains(t, userService, "      nats:\n        condition: service_healthy\n")
	})
}

func repoRootFromComposeTest(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	// src/backend/user -> repository root.
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}

func readComposeYAMLForUserTest(t *testing.T, root string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, "docker-compose.yml"))
	require.NoError(t, err)
	return strings.ReplaceAll(string(raw), "\r\n", "\n")
}

func serviceBlock(compose, service string) string {
	var block strings.Builder
	found := false
	for _, line := range strings.Split(compose, "\n") {
		if line == "  "+service+":" {
			found = true
			continue
		}
		if found && strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "    ") {
			break
		}
		if found {
			block.WriteString(line)
			block.WriteByte('\n')
		}
	}
	return block.String()
}
