package integrationtest

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

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
