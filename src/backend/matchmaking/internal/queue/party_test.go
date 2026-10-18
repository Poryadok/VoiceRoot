package queue

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"os"
	"sync"
	"testing"
	"time"
)

func realPartyQueue(t *testing.T) *RedisQueue {
	t.Helper()
	addr := os.Getenv("BE011_TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("real Redis unavailable: set BE011_TEST_REDIS_ADDR")
	}
	client := redis.NewClient(&redis.Options{Addr: addr})
	require.NoError(t, client.Ping(context.Background()).Err())
	q := &RedisQueue{Client: client, Prefix: "be011:" + uuid.NewString()}
	t.Cleanup(func() {
		ctx := context.Background()
		var cursor uint64
		for {
			keys, next, err := client.Scan(ctx, cursor, q.Prefix+":*", 100).Result()
			if err != nil {
				break
			}
			if len(keys) > 0 {
				_ = client.Del(ctx, keys...).Err()
			}
			cursor = next
			if cursor == 0 {
				break
			}
		}
		_ = client.Close()
	})
	return q
}
func recoveryParty() []SearchRecovery {
	game := uuid.New()
	at := time.Now().UTC()
	return []SearchRecovery{{GameID: game, Mode: "duo", Region: "eu", SessionID: uuid.New(), ProfileID: uuid.New(), CreatedAt: at, Generation: 1}, {GameID: game, Mode: "duo", Region: "eu", SessionID: uuid.New(), ProfileID: uuid.New(), CreatedAt: at, Generation: 1}}
}
func TestRedisPartyIsAtomicAcrossConflictingProfileAndReplay(t *testing.T) {
	q := realPartyQueue(t)
	ctx := context.Background()
	party := recoveryParty()
	other := uuid.New()
	require.NoError(t, q.AcquireLock(ctx, party[1].ProfileID, other))
	require.ErrorIs(t, q.RecoverParty(ctx, party), ErrLockHeld)
	require.Equal(t, int64(0), q.Client.Exists(ctx, q.lockKey(party[0].ProfileID)).Val())
	require.Zero(t, q.Client.ZCard(ctx, q.queueKey(party[0].GameID, party[0].Mode, party[0].Region)).Val())
	require.NoError(t, q.ReleaseLock(ctx, party[1].ProfileID, other))
	require.NoError(t, q.RecoverParty(ctx, party))
	require.NoError(t, q.RecoverParty(ctx, party))
	require.Equal(t, int64(2), q.Client.ZCard(ctx, q.queueKey(party[0].GameID, party[0].Mode, party[0].Region)).Val())
	for _, s := range party {
		require.Equal(t, s.SessionID.String(), q.Client.Get(ctx, q.lockKey(s.ProfileID)).Val())
	}
}
func TestRedisPartyWrongTypeCannotLeavePartialLocks(t *testing.T) {
	q := realPartyQueue(t)
	ctx := context.Background()
	party := recoveryParty()
	key := q.queueKey(party[0].GameID, party[0].Mode, party[0].Region)
	require.NoError(t, q.Client.Set(ctx, key, "wrong-type", 0).Err())
	require.Error(t, q.RecoverParty(ctx, party))
	for _, s := range party {
		require.Zero(t, q.Client.Exists(ctx, q.lockKey(s.ProfileID)).Val())
	}
	require.Zero(t, q.Client.Exists(ctx, q.generationKey()).Val())
}
func TestRedisReleaseWinsEqualGenerationAndLargeIntegerFenceIsExact(t *testing.T) {
	q := realPartyQueue(t)
	ctx := context.Background()
	party := recoveryParty()
	party[0].Generation = 9007199254740993
	party[1].Generation = 9007199254740993
	require.NoError(t, q.RecoverRelease(ctx, party[0]))
	require.ErrorIs(t, q.RecoverSearch(ctx, party[0]), ErrQueueGeneration, "release must win equal generation")
	older := party[0]
	older.Generation--
	require.ErrorIs(t, q.RecoverSearch(ctx, older), ErrQueueGeneration, "Lua double rounding cannot equate adjacent int64 generations")
	require.ErrorIs(t, q.RecoverParty(ctx, party), ErrQueueGeneration)
	require.Zero(t, q.Client.Exists(ctx, q.lockKey(party[1].ProfileID)).Val())
	party[0].Generation++
	party[1].Generation++
	require.NoError(t, q.RecoverParty(ctx, party))
	stale := party[0]
	stale.Generation--
	require.ErrorIs(t, q.EnqueueScopedGeneration(ctx, stale.SpaceID, stale.GameID, stale.Mode, stale.Region, stale.SessionID, stale.CreatedAt, stale.Generation), ErrQueueGeneration)
	require.NoError(t, q.DequeueScopedGeneration(ctx, stale.SpaceID, stale.GameID, stale.Mode, stale.Region, stale.SessionID, stale.Generation))
	require.Equal(t, int64(2), q.Client.ZCard(ctx, q.queueKey(party[0].GameID, party[0].Mode, party[0].Region)).Val())
}

func TestRedisPartyConcurrentOverlapHasOneCompleteWinner(t *testing.T) {
	q := realPartyQueue(t)
	ctx := context.Background()
	a := recoveryParty()
	b := recoveryParty()
	for i := range b {
		b[i].GameID = a[0].GameID
	}
	b[1].ProfileID = a[1].ProfileID
	start := make(chan struct{})
	out := make(chan error, 2)
	var wg sync.WaitGroup
	for _, party := range [][]SearchRecovery{a, b} {
		wg.Add(1)
		go func(p []SearchRecovery) { defer wg.Done(); <-start; out <- q.RecoverParty(ctx, p) }(party)
	}
	close(start)
	wg.Wait()
	close(out)
	wins := 0
	losses := 0
	for err := range out {
		if err == nil {
			wins++
		} else if errors.Is(err, ErrLockHeld) {
			losses++
		} else {
			require.NoError(t, err)
		}
	}
	require.Equal(t, 1, wins)
	require.Equal(t, 1, losses)
	require.Equal(t, int64(2), q.Client.ZCard(ctx, q.queueKey(a[0].GameID, a[0].Mode, a[0].Region)).Val())
}

func TestRedisPartyACLDenialLeavesNoPartialMutation(t *testing.T) {
	q := realPartyQueue(t)
	ctx := context.Background()
	user := "party_" + uuid.NewString()
	password := "isolated-party-fixture"
	require.NoError(t, q.Client.Do(ctx, "ACL", "SETUSER", user, "reset", "on", ">"+password, "~"+q.Prefix+":*", "+hello", "+ping", "+eval", "+type", "+get", "+hget", "+set", "+hset").Err())
	t.Cleanup(func() { _ = q.Client.Do(context.Background(), "ACL", "DELUSER", user).Err() })
	limited := redis.NewClient(&redis.Options{Addr: os.Getenv("BE011_TEST_REDIS_ADDR"), Username: user, Password: password})
	defer limited.Close()
	party := recoveryParty()
	require.ErrorIs(t, (&RedisQueue{Client: limited, Prefix: q.Prefix}).RecoverParty(ctx, party), ErrQueueUnavailable)
	for _, s := range party {
		require.Zero(t, q.Client.Exists(ctx, q.lockKey(s.ProfileID)).Val())
	}
	require.Zero(t, q.Client.Exists(ctx, q.generationKey()).Val())
	require.Zero(t, q.Client.Exists(ctx, q.queueKey(party[0].GameID, party[0].Mode, party[0].Region)).Val())
}
