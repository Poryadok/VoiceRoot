package queue

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"voice/backend/matchmaking/internal/terminalrouting"
)

func TestRedisQueue_TerminalGenerationDoesNotOutliveRoutingDeadline(t *testing.T) {
	ctx := context.Background()
	// Original Auth schedule/deadline from the agreed synthetic producer v1
	// fixture. This baseline demonstrates the existing unbounded raw projection;
	// it does not pretend the current queue already consumes lifecycle authority.
	scheduledAt := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	purgeAt := time.Date(2026, time.January, 31, 0, 0, 0, 0, time.UTC)
	mr := miniredis.RunT(t)
	mr.SetTime(scheduledAt)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	q := &RedisQueue{Client: client, Prefix: "terminal-deadline"}
	affectedSession, otherSession := uuid.New(), uuid.New()
	gameID := uuid.New()
	life, err := terminalrouting.New(terminalrouting.Schedule{
		FenceID: "fixture-auth-lifetime-fence-a", CycleID: "fixture-deletion-cycle-a1", Revision: 41,
		ScheduledAt: scheduledAt, PurgeAt: purgeAt,
	})
	require.NoError(t, err)
	require.NoError(t, q.RecoverTerminalRelease(ctx, SearchRecovery{
		SessionID: affectedSession, ProfileID: uuid.New(), GameID: gameID,
		Mode: "duo", Region: "eu", Generation: 3,
	}, life))
	require.NoError(t, q.RecoverRelease(ctx, SearchRecovery{
		SessionID: otherSession, ProfileID: uuid.New(), GameID: gameID,
		Mode: "duo", Region: "eu", Generation: 3,
	}))
	// Before the original deadline both generation fences exist. Another live
	// identity must not be erased by dropping/expiring the whole shared hash.
	require.Equal(t, "3", client.Get(ctx, q.terminalGenerationKey(affectedSession)).Val())
	mr.FastForward(purgeAt.Sub(scheduledAt) + time.Second)
	otherExists, err := client.HExists(ctx, q.generationKey(), otherSession.String()).Result()
	require.NoError(t, err)
	require.True(t, otherExists, "unrelated generation fence must remain intact")
	affectedExists, err := client.HExists(ctx, q.generationKey(), affectedSession.String()).Result()
	require.NoError(t, err)
	require.False(t, affectedExists, "raw terminal session routing must be absent after its original canonical deadline")
	require.False(t, mr.Exists(q.terminalGenerationKey(affectedSession)), "the bounded raw-session key must also be gone")
}

func TestTerminalProjectionLateReceiptDoesNotRestartDeadline(t *testing.T) {
	ctx := context.Background()
	scheduled := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	deadline := scheduled.Add(30 * 24 * time.Hour)
	life, err := terminalrouting.New(terminalrouting.Schedule{
		FenceID: "fence", CycleID: "cycle", Revision: 41, ScheduledAt: scheduled, PurgeAt: deadline,
	})
	require.NoError(t, err)
	mr := miniredis.RunT(t)
	mr.SetTime(deadline.Add(-time.Second)) // receipt is nearly thirty days late
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	q := &RedisQueue{Client: client, Prefix: "late-terminal"}
	profile, oldSession, newOwner := uuid.New(), uuid.New(), uuid.New()
	require.NoError(t, q.AcquireLock(ctx, profile, newOwner))
	recovery := SearchRecovery{ProfileID: profile, SessionID: oldSession, GameID: uuid.New(), Mode: "duo", Region: "eu", Generation: 3}
	require.NoError(t, q.RecoverTerminalRelease(ctx, recovery, life))
	require.LessOrEqual(t, mr.TTL(q.terminalGenerationKey(oldSession)), time.Second)
	owner, err := client.Get(ctx, q.lockKey(profile)).Result()
	require.NoError(t, err)
	require.Equal(t, newOwner.String(), owner)
	older := recovery
	older.Generation = 2
	require.ErrorIs(t, q.RecoverTerminalRelease(ctx, older, life), ErrQueueGeneration)
	mr.FastForward(2 * time.Second)
	mr.SetTime(deadline.Add(time.Second))
	require.ErrorIs(t, q.RecoverTerminalRelease(ctx, recovery, life), ErrTerminalRoutingExpired)
	require.False(t, mr.Exists(q.terminalGenerationKey(oldSession)))
	require.False(t, client.HExists(ctx, q.generationKey(), oldSession.String()).Val())
	owner, err = client.Get(ctx, q.lockKey(profile)).Result()
	require.NoError(t, err)
	require.Equal(t, newOwner.String(), owner)
}

func TestTerminalProjectionPreservesExactInt64Generation(t *testing.T) {
	ctx := context.Background()
	start := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	life, err := terminalrouting.New(terminalrouting.Schedule{FenceID: "fence", CycleID: "cycle", Revision: 41,
		ScheduledAt: start, PurgeAt: start.Add(30 * 24 * time.Hour)})
	require.NoError(t, err)
	mr := miniredis.RunT(t)
	mr.SetTime(start)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	q := &RedisQueue{Client: client, Prefix: "exact-terminal"}
	recovery := SearchRecovery{ProfileID: uuid.New(), SessionID: uuid.New(), GameID: uuid.New(), Mode: "duo", Region: "eu", Generation: 9007199254740993}
	require.NoError(t, q.RecoverTerminalRelease(ctx, recovery, life))
	recovery.Generation--
	require.ErrorIs(t, q.RecoverTerminalRelease(ctx, recovery, life), ErrQueueGeneration)
	require.Equal(t, "9007199254740993", client.Get(ctx, q.terminalGenerationKey(recovery.SessionID)).Val())
}
