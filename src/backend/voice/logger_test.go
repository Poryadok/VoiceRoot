package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"

	"voice/backend/voice/internal/principalgrpc"
)

// A separate process observes stdout/stderr and the real bootstrap without
// installing a test-only default handler that would mask production wiring.
func TestServiceLoggerDependencyDiagnostics(t *testing.T) {
	if os.Getenv("VOICE_LOGGER_TEST_CHILD") == "1" {
		logger := newServiceLogger()
		logger.Info("logger control")
		ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
			"x-request-id", " request-123 ", "authorization", "Bearer secret-canary"))
		for _, stage := range []string{"jwks_request", "replay_write", "auth_floor_call"} {
			principalgrpc.DependencyFailure(ctx, stage, fmt.Errorf("secret-canary: %w", context.DeadlineExceeded), time.Now())
		}
		principalgrpc.DependencyHTTPFailure(context.Background(), 503, time.Now())
		os.Exit(0)
	}
	exe, err := os.Executable()
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, exe, "-test.run=^TestServiceLoggerDependencyDiagnostics$")
	cmd.Env = append(os.Environ(), "VOICE_LOGGER_TEST_CHILD=1", "LOG_LEVEL=info")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	require.NoError(t, cmd.Run())
	require.Empty(t, stderr.String(), "diagnostics must use JSON stdout")
	require.NotContains(t, stdout.String(), "secret-canary")
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	require.Len(t, lines, 5)
	stages := []string{"jwks_request", "replay_write", "auth_floor_call", "jwks_status"}
	for i, line := range lines {
		var record map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &record))
		require.Equal(t, "voice", record["service"])
		if i == 0 {
			continue
		}
		require.Equal(t, "voice_trust_dependency_failure", record["event"])
		require.Equal(t, stages[i-1], record["stage"])
		require.Contains(t, record, "duration_ms")
		if i == 4 {
			require.Equal(t, "", record["request_id"])
			require.Equal(t, float64(503), record["http_status"])
		} else {
			require.Equal(t, "request-123", record["request_id"])
			require.Equal(t, "timeout", record["cause"])
		}
	}
}
