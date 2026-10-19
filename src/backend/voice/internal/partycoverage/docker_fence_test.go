package partycoverage

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/docker/docker/client"
	"github.com/docker/docker/errdefs"
	"github.com/google/uuid"
	"github.com/livekit/protocol/auth"
	protocol "github.com/livekit/protocol/livekit"
	lksdk "github.com/livekit/server-sdk-go/v2"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestDockerCohortRetirementEndsLiveMediaAndRejectsOldGrant(t *testing.T) {
	if testing.Short() {
		t.Skip("real owned Docker/LiveKit cohort proof in non-short suite")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	api, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	require.NoError(t, err)
	defer api.Close()
	env, cohort := uuid.New(), uuid.New()
	old, url := mediaFixture(t, ctx, env, cohort, "oldkey", "old-isolated-secret-at-least-thirty-two")
	writer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{ContainerRequest: testcontainers.ContainerRequest{Image: "alpine:3.22", Cmd: []string{"sleep", "600"}, Labels: map[string]string{environmentLabel: env.String(), cohortLabel: cohort.String(), resourceLabel: "writer"}}, Started: true})
	require.NoError(t, err)
	cleanupContainer(t, writer)
	oldInfo, err := api.ContainerInspect(ctx, old.GetContainerID())
	require.NoError(t, err)
	writerInfo, err := api.ContainerInspect(ctx, writer.GetContainerID())
	require.NoError(t, err)
	token, err := auth.NewAccessToken("oldkey", "old-isolated-secret-at-least-thirty-two").SetIdentity("precut-user").AddGrant(&auth.VideoGrant{RoomJoin: true, Room: "precut-room"}).SetValidFor(time.Hour).ToJWT()
	require.NoError(t, err)
	disconnected := make(chan struct{}, 1)
	mediaLost := func() {
		select {
		case disconnected <- struct{}{}:
		default:
		}
	}
	room, err := lksdk.ConnectToRoomWithToken(url, token, &lksdk.RoomCallback{OnDisconnected: mediaLost, OnReconnecting: mediaLost}, lksdk.WithConnectTimeout(10*time.Second), lksdk.WithAutoSubscribe(false))
	require.NoError(t, err)
	defer room.Disconnect()
	roomAPI := lksdk.NewRoomServiceClient(url, "oldkey", "old-isolated-secret-at-least-thirty-two")
	require.Eventually(t, func() bool {
		participants, err := roomAPI.ListParticipants(ctx, &protocol.ListParticipantsRequest{Room: "precut-room"})
		return err == nil && len(participants.Participants) == 1 && participants.Participants[0].State == protocol.ParticipantInfo_ACTIVE
	}, 10*time.Second, 50*time.Millisecond)
	scope := dockerScope{EnvironmentID: env, CohortID: cohort, Targets: []dockerTarget{{ID: old.GetContainerID(), ImageID: oldInfo.Image, Resource: "media"}}}
	_, err = retireDockerCohort(ctx, api, scope)
	require.Error(t, err, "omitting an old execution producer must not close or certify a partial cohort")
	_, err = roomAPI.ListRooms(ctx, &protocol.ListRoomsRequest{})
	require.NoError(t, err)
	scope.Targets = append(scope.Targets, dockerTarget{ID: writer.GetContainerID(), ImageID: writerInfo.Image, Resource: "writer"})
	proof, err := retireDockerCohort(ctx, api, scope)
	require.NoError(t, err)
	require.NotEmpty(t, proof)
	select {
	case <-disconnected:
	case <-time.After(15 * time.Second):
		t.Fatal("client did not observe old media connection loss after realm retirement")
	}
	require.NotEqual(t, lksdk.ConnectionStateConnected, room.ConnectionState())
	_, err = api.ContainerInspect(ctx, old.GetContainerID())
	require.True(t, errdefs.IsNotFound(err))
	_, err = api.ContainerInspect(ctx, writer.GetContainerID())
	require.True(t, errdefs.IsNotFound(err))
	replay, err := retireDockerCohort(ctx, api, scope)
	require.NoError(t, err)
	require.Equal(t, proof, replay)
	_, newURL := mediaFixture(t, ctx, env, uuid.New(), "newkey", "new-isolated-secret-at-least-thirty-two")
	rejoined, err := lksdk.ConnectToRoomWithToken(newURL, token, nil, lksdk.WithConnectTimeout(2*time.Second))
	if rejoined != nil {
		rejoined.Disconnect()
	}
	require.Error(t, err, "old bearer must not gain admission to replacement realm")
}

func mediaFixture(t *testing.T, ctx context.Context, env, cohort uuid.UUID, key, secret string) (testcontainers.Container, string) {
	t.Helper()
	networks := []string{}
	if n := os.Getenv("PARTY_COVERAGE_TEST_NETWORK"); n != "" {
		networks = append(networks, n)
	}
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{ContainerRequest: testcontainers.ContainerRequest{Image: "livekit/livekit-server:latest", Cmd: []string{"--dev", "--bind", "0.0.0.0", "--keys", key + ": " + secret}, Networks: networks, Labels: map[string]string{environmentLabel: env.String(), cohortLabel: cohort.String(), resourceLabel: "media"}, WaitingFor: wait.ForLog("starting LiveKit server")}, Started: true})
	require.NoError(t, err)
	cleanupContainer(t, c)
	ip, err := c.ContainerIP(ctx)
	require.NoError(t, err)
	return c, "http://" + ip + ":7880"
}
func cleanupContainer(t *testing.T, c testcontainers.Container) {
	t.Helper()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		api, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
		if err != nil {
			t.Errorf("owned cleanup Docker client: %v", err)
			return
		}
		defer api.Close()
		_, err = api.ContainerInspect(ctx, c.GetContainerID())
		if errdefs.IsNotFound(err) {
			return
		}
		if err != nil {
			t.Errorf("owned cleanup readback: %v", err)
			return
		}
		if err := c.Terminate(ctx); err != nil && !errdefs.IsNotFound(err) {
			t.Errorf("owned container cleanup: %v", err)
		}
	})
}
