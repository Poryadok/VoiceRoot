package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// TestComposeWiring_yaml asserts docker-compose.yml wires core gRPC upstreams,
// Realtime, and Gateway /ws proxy (dev-compose-realtime-gateway).
func TestComposeWiring_yaml(t *testing.T) {
	t.Parallel()

	root := repoRootFromTest(t)
	yml := readComposeYAML(t, root)

	require.Contains(t, yml, "GATEWAY_GRPC_UPSTREAMS_JSON:")
	require.Contains(t, yml, `"users":"user:9090"`)
	require.Contains(t, yml, `"friends":"social:9090"`)
	require.Contains(t, yml, `"chats":"chat:9090"`)
	require.Contains(t, yml, `"messages":"messaging:9090"`)
	require.Contains(t, yml, `"spaces":"space:9090"`)

	require.Contains(t, yml, "\n  space:\n")
	require.Contains(t, yml, "SPACE_GRPC_LISTEN: :9090")
	require.Contains(t, yml, "space_db")
	space := parseComposeYAML(t, root).Services["space"]
	for name, value := range map[string]string{"USER_GRPC_ADDR": "user:9090", "CHAT_GRPC_ADDR": "chat:9090", "SOCIAL_GRPC_ADDR": "social:9090", "NATS_URL": "nats://nats:4222"} {
		require.Equal(t, value, space.Environment[name])
	}
	for _, dependency := range []string{"postgres", "redis", "nats", "role", "user", "chat", "social"} {
		require.Equal(t, "service_healthy", space.DependsOn[dependency].Condition)
	}
	for _, dependency := range []string{"compose-db-init", "social-principal-init"} {
		require.Equal(t, "service_completed_successfully", space.DependsOn[dependency].Condition)
	}

	require.Contains(t, yml, "GATEWAY_REALTIME_UPSTREAM_URL:")
	require.Contains(t, yml, "http://realtime:8080")
	require.Contains(t, yml, "GATEWAY_SESSION_EPOCH_STRICT: \"true\"")

	require.Contains(t, yml, "\n  realtime:\n")
	require.Contains(t, yml, "REALTIME_REDIS_ADDR:")
	require.Contains(t, yml, "REALTIME_JWKS_URL:")
	require.Contains(t, yml, "REALTIME_SESSION_EPOCH_STRICT: \"true\"")
	require.Contains(t, yml, "REALTIME_CHAT_GRPC_ADDR: chat:9090")
	require.Contains(t, yml, "REALTIME_USER_GRPC_ADDR: user:9090")
	require.Contains(t, yml, "REALTIME_SOCIAL_GRPC_ADDR: social:9090")
	require.Contains(t, yml, "NATS_URL: nats://nats:4222")

	require.Contains(t, yml, "\n  chat:\n")
	require.Contains(t, yml, "MESSAGING_GRPC_ADDR: messaging:9090")

	require.Contains(t, yml, "\n  messaging:\n")
	require.Contains(t, yml, "FILE_GRPC_ADDR: file:9090")

	require.Contains(t, yml, "\n  notification:\n")
	require.Contains(t, yml, "NOTIFICATION_GRPC_LISTEN: :9090")
	require.Contains(t, yml, "notification_db")
	require.Contains(t, yml, `"notifications":"notification:9090"`)

	require.Contains(t, yml, "\n  matchmaking:\n")
	require.Contains(t, yml, "MATCHMAKING_GRPC_LISTEN: :9090")
	require.Contains(t, yml, "CHAT_GRPC_ADDR: chat:9090")
	require.Contains(t, yml, "VOICE_GRPC_ADDR: voice:9090")
	require.Contains(t, yml, "matchmaking_db")
	require.Contains(t, yml, `"matchmaking":"matchmaking:9090"`)
}

// TestComposeMatchmakingRatingPrivacyWiring_yaml keeps local Compose aligned
// with the S2S clients that enforce show_mm_rating audiences. Without the User
// client Matchmaking intentionally keeps its documented degraded passthrough,
// so every dependency must be explicit in the app stack.
func TestComposeMatchmakingRatingPrivacyWiring_yaml(t *testing.T) {
	t.Parallel()

	root := repoRootFromTest(t)
	compose := parseComposeYAML(t, root)
	matchmaking, ok := compose.Services["matchmaking"]
	require.True(t, ok, "matchmaking service must be present in Compose")

	// The server configures these clients independently. All three are needed to
	// enforce every show_mm_rating audience instead of retaining the documented
	// standalone passthrough when USER_GRPC_ADDR is absent.
	require.Equal(t, "user:9090", matchmaking.Environment["USER_GRPC_ADDR"])
	require.Equal(t, "social:9090", matchmaking.Environment["SOCIAL_GRPC_ADDR"])
	require.Equal(t, "space:9090", matchmaking.Environment["SPACE_GRPC_ADDR"])

	for _, dependency := range []string{"user", "social", "space"} {
		require.Equal(t, "service_healthy", matchmaking.DependsOn[dependency].Condition,
			"matchmaking must wait for %s before privacy checks are enabled", dependency)
	}
}

type composeYAML struct {
	Services map[string]composeService `yaml:"services"`
}

type composeService struct {
	Environment map[string]string           `yaml:"environment"`
	DependsOn   map[string]composeDependsOn `yaml:"depends_on"`
}

type composeDependsOn struct {
	Condition string `yaml:"condition"`
}

func repoRootFromTest(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	// src/backend/gateway → repo root
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}

func readComposeYAML(t *testing.T, root string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, "docker-compose.yml"))
	require.NoError(t, err)
	return strings.ReplaceAll(string(raw), "\r\n", "\n")
}

func parseComposeYAML(t *testing.T, root string) composeYAML {
	t.Helper()

	var compose composeYAML
	require.NoError(t, yaml.Unmarshal([]byte(readComposeYAML(t, root)), &compose))
	return compose
}
