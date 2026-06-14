package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestComposePhase12SubscriptionWiring_yaml documents subscription gRPC upstream in compose.
func TestComposePhase12SubscriptionWiring_yaml(t *testing.T) {
	t.Parallel()

	root := repoRootFromTest(t)
	raw, err := os.ReadFile(filepath.Join(root, "docker-compose.yml"))
	require.NoError(t, err)
	yml := string(raw)

	require.Contains(t, yml, "\n  subscription:\n")
	require.Contains(t, yml, "SUBSCRIPTION_GRPC_LISTEN: :9090")
	require.Contains(t, yml, "subscription_db")
	require.Contains(t, yml, `"subscription":"subscription:9090"`)
}

// TestTranscodeSubscription_GetMe_non404WhenGRPCConfigured documents /api/v1/subscription/me REST transcode.
func TestTranscodeSubscription_GetMe_non404WhenGRPCConfigured(t *testing.T) {
	t.Parallel()

	rec := &recordingSubscriptionBackend{}
	h := newSubscriptionContractGateway(t, rec)

	resp := performRequest(h, "GET", "/api/v1/subscription/me", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.NotEqual(t, 404, resp.Code, "body=%s", resp.Body.String())
	require.NotNil(t, rec.lastGetSubscription)
}
