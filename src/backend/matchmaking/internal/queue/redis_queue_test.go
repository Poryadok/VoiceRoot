package queue

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func newTestQueue(t *testing.T) (*RedisQueue, func()) {
	t.Helper()
	s := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: s.Addr()})
	q := &RedisQueue{Client: client, Prefix: "test"}
	return q, func() {
		_ = client.Close()
		s.Close()
	}
}

func TestRedisQueue_FIFOEnqueueAndDequeue(t *testing.T) {
	t.Parallel()
	q, cleanup := newTestQueue(t)
	t.Cleanup(cleanup)
	ctx := context.Background()
	gameID := uuid.New()
	mode := "5v5 Ranked"
	region := "eu"
	s1 := uuid.New()
	s2 := uuid.New()
	now := time.Now().UTC()

	require.NoError(t, q.Enqueue(ctx, gameID, mode, region, s1, now))
	require.NoError(t, q.Enqueue(ctx, gameID, mode, region, s2, now.Add(time.Second)))

	depth, err := q.QueueDepth(ctx, gameID, mode, region)
	require.NoError(t, err)
	require.Equal(t, int64(2), depth)

	require.NoError(t, q.Dequeue(ctx, gameID, mode, region, s1))
	depth, err = q.QueueDepth(ctx, gameID, mode, region)
	require.NoError(t, err)
	require.Equal(t, int64(1), depth)
}

func TestRedisQueue_UnavailableWhenNil(t *testing.T) {
	t.Parallel()
	var q RedisQueue
	ctx := context.Background()
	err := q.AcquireLock(ctx, uuid.New(), uuid.New())
	require.ErrorIs(t, err, ErrQueueUnavailable)
}

func TestRedisQueue_DequeueMissingReturnsError(t *testing.T) {
	t.Parallel()
	q, cleanup := newTestQueue(t)
	t.Cleanup(cleanup)
	ctx := context.Background()
	err := q.Dequeue(ctx, uuid.New(), "mode", "eu", uuid.New())
	require.ErrorIs(t, err, ErrNotEnqueued)
}

func TestRedisQueue_ReleaseLockIgnoresOtherSession(t *testing.T) {
	t.Parallel()
	q, cleanup := newTestQueue(t)
	t.Cleanup(cleanup)
	ctx := context.Background()
	profileID := uuid.New()
	s1 := uuid.New()
	s2 := uuid.New()
	require.NoError(t, q.AcquireLock(ctx, profileID, s1))
	require.NoError(t, q.ReleaseLock(ctx, profileID, s2))
	err := q.AcquireLock(ctx, profileID, s2)
	require.ErrorIs(t, err, ErrLockHeld)
}

func TestRedisQueue_ReleaseLockWhenMissing(t *testing.T) {
	t.Parallel()
	q, cleanup := newTestQueue(t)
	t.Cleanup(cleanup)
	ctx := context.Background()
	require.NoError(t, q.ReleaseLock(ctx, uuid.New(), uuid.New()))
}

func TestRedisQueue_Ping(t *testing.T) {
	t.Parallel()
	q, cleanup := newTestQueue(t)
	t.Cleanup(cleanup)
	require.NoError(t, q.Ping(context.Background()))
}

func TestRedisQueue_LockPreventsDuplicate(t *testing.T) {
	t.Parallel()
	q, cleanup := newTestQueue(t)
	t.Cleanup(cleanup)
	ctx := context.Background()
	profileID := uuid.New()
	s1 := uuid.New()
	s2 := uuid.New()

	require.NoError(t, q.AcquireLock(ctx, profileID, s1))
	err := q.AcquireLock(ctx, profileID, s2)
	require.ErrorIs(t, err, ErrLockHeld)

	require.NoError(t, q.ReleaseLock(ctx, profileID, s1))
	require.NoError(t, q.AcquireLock(ctx, profileID, s2))
}
