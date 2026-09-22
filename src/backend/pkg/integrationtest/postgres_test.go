package integrationtest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

const expectedPostgresStartupTimeout = time.Minute

func TestStartPostgresContainerUsesIndependentBoundedStartupContext(t *testing.T) {
	originalRun := postgresRun
	t.Cleanup(func() { postgresRun = originalRun })

	caller, cancel := context.WithCancel(context.Background())
	cancel()
	container := &postgres.PostgresContainer{}
	postgresRun = func(ctx context.Context, _ string, _ ...testcontainers.ContainerCustomizer) (*postgres.PostgresContainer, error) {
		require.NoError(t, ctx.Err())
		deadline, ok := ctx.Deadline()
		require.True(t, ok)
		remaining := time.Until(deadline)
		require.GreaterOrEqual(t, remaining, expectedPostgresStartupTimeout-5*time.Second)
		require.LessOrEqual(t, remaining, expectedPostgresStartupTimeout+5*time.Second)
		return container, nil
	}

	got, err := startPostgresContainer(caller, "testdb")

	require.NoError(t, err)
	require.Same(t, container, got)
}

func TestStartPostgresContainerRetriesOnceAfterPartialStartupFailure(t *testing.T) {
	originalRun := postgresRun
	originalTerminate := postgresTerminate
	t.Cleanup(func() {
		postgresRun = originalRun
		postgresTerminate = originalTerminate
	})

	partial := &postgres.PostgresContainer{}
	recovered := &postgres.PostgresContainer{}
	firstErr := errors.New("temporary postgres startup failure")
	events := make([]string, 0, 3)
	runs := 0
	postgresRun = func(context.Context, string, ...testcontainers.ContainerCustomizer) (*postgres.PostgresContainer, error) {
		runs++
		if runs == 1 {
			events = append(events, "run-1")
			return partial, firstErr
		}
		require.Equal(t, []string{"run-1", "cleanup-partial"}, events)
		events = append(events, "run-2")
		return recovered, nil
	}
	postgresTerminate = func(_ context.Context, got *postgres.PostgresContainer) error {
		require.Same(t, partial, got)
		events = append(events, "cleanup-partial")
		return nil
	}

	got, err := startPostgresContainer(context.Background(), "testdb")

	require.NoError(t, err)
	require.Same(t, recovered, got)
	require.Equal(t, []string{"run-1", "cleanup-partial", "run-2"}, events)
}

func TestStartPostgresContainerStopsAfterRetryFailureAndCleansBothPartialContainers(t *testing.T) {
	originalRun := postgresRun
	originalTerminate := postgresTerminate
	t.Cleanup(func() {
		postgresRun = originalRun
		postgresTerminate = originalTerminate
	})

	first := &postgres.PostgresContainer{}
	second := &postgres.PostgresContainer{}
	firstErr := errors.New("first startup failure")
	retryErr := errors.New("retry startup failure")
	events := make([]string, 0, 4)
	runs := 0
	postgresRun = func(context.Context, string, ...testcontainers.ContainerCustomizer) (*postgres.PostgresContainer, error) {
		runs++
		switch runs {
		case 1:
			events = append(events, "run-1")
			return first, firstErr
		case 2:
			require.Equal(t, []string{"run-1", "cleanup-first"}, events)
			events = append(events, "run-2")
			return second, retryErr
		default:
			t.Fatal("postgres startup must not retry more than once")
			return nil, nil
		}
	}
	postgresTerminate = func(_ context.Context, got *postgres.PostgresContainer) error {
		switch got {
		case first:
			events = append(events, "cleanup-first")
		case second:
			events = append(events, "cleanup-second")
		default:
			t.Fatal("unexpected partial container cleanup")
		}
		return nil
	}

	got, err := startPostgresContainer(context.Background(), "testdb")

	require.ErrorIs(t, err, retryErr)
	require.Same(t, second, got)
	require.Equal(t, []string{"run-1", "cleanup-first", "run-2", "cleanup-second"}, events)
}

func TestStartPostgresContainerCleansPartiallyCreatedContainer(t *testing.T) {
	originalRun := postgresRun
	originalTerminate := postgresTerminate
	t.Cleanup(func() {
		postgresRun = originalRun
		postgresTerminate = originalTerminate
	})

	partial := &postgres.PostgresContainer{}
	wantErr := errors.New("wait failed")
	var terminated *postgres.PostgresContainer
	postgresRun = func(context.Context, string, ...testcontainers.ContainerCustomizer) (*postgres.PostgresContainer, error) {
		return partial, wantErr
	}
	postgresTerminate = func(_ context.Context, got *postgres.PostgresContainer) error {
		terminated = got
		return nil
	}

	got, err := startPostgresContainer(context.Background(), "testdb")

	require.ErrorIs(t, err, wantErr)
	require.Same(t, partial, got)
	require.Same(t, partial, terminated)
}

func TestStartPostgresContainerDoesNotTerminateSuccessfulContainer(t *testing.T) {
	originalRun := postgresRun
	originalTerminate := postgresTerminate
	t.Cleanup(func() {
		postgresRun = originalRun
		postgresTerminate = originalTerminate
	})

	container := &postgres.PostgresContainer{}
	postgresRun = func(context.Context, string, ...testcontainers.ContainerCustomizer) (*postgres.PostgresContainer, error) {
		return container, nil
	}
	terminated := false
	postgresTerminate = func(context.Context, *postgres.PostgresContainer) error {
		terminated = true
		return nil
	}

	got, err := startPostgresContainer(context.Background(), "testdb")

	require.NoError(t, err)
	require.Same(t, container, got)
	require.False(t, terminated)
}
