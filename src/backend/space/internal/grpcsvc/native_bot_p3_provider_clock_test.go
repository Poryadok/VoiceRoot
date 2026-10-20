package grpcsvc

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"path/filepath"
	"testing"
	"time"
	"voice/backend/pkg/integrationtest"
	"voice/backend/space/internal/store"
)

func setNativeP3ProviderDBTime(t *testing.T, admin *store.SpaceStore, at time.Time) {
	t.Helper()
	at = at.UTC()
	require.Zero(t, at.Nanosecond(), "boundary fixture chooses its first P3 clock on an exactly representable whole second")
	value := at.Format("2006-01-02 15:04:05")
	// Atomic rename never exposes a truncated clock file to another PG process.
	_, err := admin.Pool.Exec(context.Background(), "COPY (SELECT '"+value+"') TO PROGRAM 'cat > /tmp/native-terminal-clock.next && mv /tmp/native-terminal-clock.next /tmp/native-terminal-clock'")
	require.NoError(t, err)
	var actual time.Time
	require.NoError(t, admin.Pool.QueryRow(context.Background(), `SELECT clock_timestamp()`).Scan(&actual))
	require.Equal(t, at, actual.UTC(), "test clock must actually hold the exact DB microsecond boundary")
}

func nativeP3ProviderClockImage(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	integrationtest.ConfigureDockerTesting()
	tag := uuid.NewString()
	image := "voice-native-terminal-clock:" + tag
	process, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			FromDockerfile: testcontainers.FromDockerfile{Context: filepath.Join(repoRoot(t), "src/backend/space/internal/store/testdata/terminal_clock"), Dockerfile: "Dockerfile", Repo: "voice-native-terminal-clock", Tag: tag},
			Cmd:            []string{"true"}, WaitingFor: wait.ForExit(),
		}, Started: true,
	})
	if process != nil {
		t.Cleanup(func() {
			cleanup, stop := context.WithTimeout(context.Background(), 30*time.Second)
			defer stop()
			require.NoError(t, process.Terminate(cleanup))
		})
	}
	require.NoError(t, err)
	return image
}
