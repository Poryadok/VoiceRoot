package partycoverage

import (
	"context"
	"testing"
	"time"

	"github.com/docker/docker/client"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
)

func TestDockerFenceObservationRejectsLiveRestoredAndWrongDaemon(t *testing.T) {
	if testing.Short() {
		t.Skip("owned real Docker component observation")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	api, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	require.NoError(t, err)
	defer api.Close()
	daemon, err := api.Info(ctx)
	require.NoError(t, err)
	environment, cohort := uuid.New(), uuid.New()
	makeWriter := func() testcontainers.Container {
		c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{ContainerRequest: testcontainers.ContainerRequest{Image: "alpine:3.22", Cmd: []string{"sleep", "60"}, Labels: map[string]string{environmentLabel: environment.String(), cohortLabel: cohort.String(), resourceLabel: "writer"}}, Started: true})
		require.NoError(t, err)
		cleanupContainer(t, c)
		return c
	}
	writer := makeWriter()
	actual, err := api.ContainerInspect(ctx, writer.GetContainerID())
	require.NoError(t, err)
	scope := dockerScope{EnvironmentID: environment, CohortID: cohort, Targets: []dockerTarget{{ID: actual.ID, ImageID: actual.Image, Resource: "writer"}}}
	_, _, err = observeDockerFence(ctx, api, daemon.ID, scope)
	require.ErrorIs(t, err, ErrUnavailable, "observation never removes a live target")
	_, err = api.ContainerInspect(ctx, actual.ID)
	require.NoError(t, err)
	_, err = retireDockerCohort(ctx, api, scope)
	require.NoError(t, err)
	proof, hash, err := observeDockerFence(ctx, api, daemon.ID, scope)
	require.NoError(t, err)
	again, againHash, err := observeDockerFence(ctx, api, daemon.ID, scope)
	require.NoError(t, err)
	require.Equal(t, proof, again)
	require.Equal(t, hash, againHash)
	_, _, err = observeDockerFence(ctx, api, daemon.ID+"-other", scope)
	require.ErrorIs(t, err, ErrUnavailable)
	empty := scope
	empty.Targets = nil
	_, _, err = observeDockerFence(ctx, api, daemon.ID, empty)
	require.ErrorIs(t, err, ErrUnavailable)
	makeWriter() // A recreated labelled writer invalidates the old observation.
	_, _, err = observeDockerFence(ctx, api, daemon.ID, scope)
	require.ErrorIs(t, err, ErrUnavailable)
}
