package main

import (
	"context"
	"github.com/stretchr/testify/require"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestReportTargetRuntimeFixture(t *testing.T) {
	fixture := os.Getenv("REPORT_TARGET_RUNTIME_FIXTURE")
	if fixture == "" {
		t.Skip("generated isolated Linux PKI required")
	}
	const prefix = "MODERATION_REPORT_TARGETS_"
	t.Setenv("MODERATION_PRINCIPAL_SIGNING_KEYS_DIR", filepath.Join(fixture, "moderation"))
	t.Setenv("MODERATION_PRINCIPAL_ACTIVE_KID", "current")
	issuer, keys, err := loadModerationIssuerFromEnv()
	require.NoError(t, err)
	require.Len(t, keys, 2)
	values := map[string]string{"TLS_CA_FILE": filepath.Join(fixture, "ca.crt"), "CLIENT_CERT_FILE": filepath.Join(fixture, "moderation-client.crt"), "CLIENT_KEY_FILE": filepath.Join(fixture, "moderation-client.key")}
	for _, owner := range []string{"USER", "MESSAGING", "SPACE", "STORY"} {
		values[owner+"_GRPC_ADDR"] = "127.0.0.1:1"
		values[owner+"_TLS_SERVER_NAME"] = "fixture-unreachable"
	}
	for name, value := range values {
		t.Setenv(prefix+name, value)
	}
	client, closeRuntime, err := newReportTargetRuntime(context.Background(), slog.Default(), issuer)
	require.NoError(t, err)
	defer closeRuntime()
	require.Len(t, client.Connections, 4)
	require.Same(t, issuer, client.Issuer, "no parallel report signer")
	_, _, err = newReportTargetRuntime(context.Background(), slog.Default(), nil)
	require.Error(t, err)
	closeRuntime()
	t.Setenv(prefix+"STORY_TLS_SERVER_NAME", "")
	_, _, err = newReportTargetRuntime(context.Background(), slog.Default(), issuer)
	require.Error(t, err, "partial owner matrix must fail startup")
}
