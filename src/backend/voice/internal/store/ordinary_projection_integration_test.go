package store

import (
	"context"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"net"
	"sync/atomic"
	"testing"
	"time"
	callsv1 "voice.app/voice/calls/v1"
	"voice/backend/pkg/integrationtest"
)

func TestOrdinaryProjectionRealRedisTwoInstancesRetryWatchAndPreserveNewOwner(t *testing.T) {
	if testing.Short() {
		t.Skip("requires isolated real Redis 7")
	}
	integrationtest.ConfigureDockerTesting()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{Name: "voice-be106-redis-" + uuid.NewString(), Image: "redis:7.4-bookworm",
			ExposedPorts: []string{"6379/tcp"}, WaitingFor: wait.ForLog("Ready to accept connections").WithStartupTimeout(30 * time.Second)}, Started: true})
	require.NoError(t, err)
	t.Cleanup(func() {
		cleanup, done := context.WithTimeout(context.Background(), 15*time.Second)
		defer done()
		require.NoError(t, container.Terminate(cleanup))
	})
	host, err := container.Host(ctx)
	require.NoError(t, err)
	port, err := container.MappedPort(ctx, "6379/tcp")
	require.NoError(t, err)
	client := redis.NewClient(&redis.Options{Addr: net.JoinHostPort(host, port.Port()), MaxRetries: 0})
	t.Cleanup(func() { _ = client.Close() })
	second := redis.NewClient(&redis.Options{Addr: net.JoinHostPort(host, port.Port()), MaxRetries: 0})
	t.Cleanup(func() { _ = second.Close() })
	info, err := client.Info(ctx, "server").Result()
	require.NoError(t, err)
	require.Contains(t, info, "redis_version:7.")
	first, other := NewRedisCallStore(client, "be106:"), NewRedisCallStore(second, "be106:")
	profile, chat := uuid.NewString(), uuid.NewString()
	old := ordinaryProjectionCall(uuid.NewString(), profile, chat, 1)
	_, err = first.ApplyOrdinaryProjection(ctx, old, nil)
	require.NoError(t, err)
	terminal := old
	terminal.OrdinaryVersion = 2
	terminal.Status = callsv1.CallStatus_CALL_STATUS_ENDED
	terminal.States = map[string]ParticipantState{}
	next := ordinaryProjectionCall(uuid.NewString(), profile, chat, 1)
	var interleaved atomic.Bool
	_, err = first.ApplyOrdinaryProjection(ctx, next, func(ctx context.Context, owner string) error {
		require.Equal(t, old.RoomID, owner)
		if interleaved.CompareAndSwap(false, true) {
			// Execute through a different TCP client after WATCH and owner read,
			// before EXEC. This deterministically invalidates the first attempt.
			_, err := other.ApplyOrdinaryProjection(ctx, terminal, nil)
			return err
		}
		return nil
	})
	require.NoError(t, err)
	require.True(t, interleaved.Load())
	_, err = other.ApplyOrdinaryProjection(ctx, terminal, nil)
	require.NoError(t, err)
	require.Equal(t, next.RoomID, client.Get(ctx, first.activeKey(profile)).Val())
	require.Equal(t, next.RoomID, client.Get(ctx, first.activeChatKey(chat)).Val())
	got, err := other.GetCall(ctx, next.RoomID)
	require.NoError(t, err)
	require.Equal(t, next.OrdinaryOperationID, got.OrdinaryOperationID)
}
