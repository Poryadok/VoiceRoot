package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestComposePhase1Wiring_yaml asserts docker-compose.yml wires Phase-1 gRPC upstreams,
// Realtime, and Gateway /ws proxy (dev-compose-realtime-gateway).
func TestComposePhase1Wiring_yaml(t *testing.T) {
	t.Parallel()

	root := repoRootFromTest(t)
	raw, err := os.ReadFile(filepath.Join(root, "docker-compose.yml"))
	require.NoError(t, err)
	yml := string(raw)

	require.Contains(t, yml, "GATEWAY_GRPC_UPSTREAMS_JSON:")
	require.Contains(t, yml, `"users":"user:9090"`)
	require.Contains(t, yml, `"friends":"social:9090"`)
	require.Contains(t, yml, `"chats":"chat:9090"`)
	require.Contains(t, yml, `"messages":"messaging:9090"`)

	require.Contains(t, yml, "GATEWAY_REALTIME_UPSTREAM_URL:")
	require.Contains(t, yml, "http://realtime:8080")

	require.Contains(t, yml, "\n  realtime:\n")
	require.Contains(t, yml, "REALTIME_REDIS_ADDR:")
	require.Contains(t, yml, "REALTIME_JWKS_URL:")
	require.Contains(t, yml, "REALTIME_CHAT_GRPC_ADDR: chat:9090")
	require.Contains(t, yml, "NATS_URL: nats://nats:4222")
}

func repoRootFromTest(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	// src/backend/gateway → repo root
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}
