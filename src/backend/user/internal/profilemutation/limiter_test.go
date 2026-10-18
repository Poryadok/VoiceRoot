package profilemutation

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func testClient(t *testing.T, server *miniredis.Miniredis) *redis.Client {
	t.Helper()
	client := redis.NewClient(&redis.Options{Addr: server.Addr(), MaxRetries: -1, DialTimeout: time.Second})
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func TestLimiterConcurrentReplicasShareAccountBudget(t *testing.T) {
	server := miniredis.RunT(t)
	first, second := New(testClient(t, server)), New(testClient(t, server))
	account := uuid.New()
	var admitted atomic.Int32
	var failures atomic.Int32
	var group sync.WaitGroup
	for i := 0; i < 64; i++ {
		group.Add(1)
		go func(index int) {
			defer group.Done()
			limiter := first
			if index%2 != 0 {
				limiter = second
			}
			allowed, err := limiter.Allow(context.Background(), account, Create)
			if err != nil {
				t.Logf("concurrent admission failed: %v", err)
				failures.Add(1)
			} else if allowed {
				admitted.Add(1)
			}
		}(i)
	}
	group.Wait()
	require.Zero(t, failures.Load())
	require.EqualValues(t, 10, admitted.Load(), "two replicas must admit exactly one shared budget")
	allowed, err := second.Allow(context.Background(), account, Create)
	require.NoError(t, err)
	require.False(t, allowed)
	allowed, err = first.Allow(context.Background(), account, Delete)
	require.NoError(t, err)
	require.True(t, allowed, "delete has an independent budget")
	allowed, err = second.Allow(context.Background(), uuid.New(), Create)
	require.NoError(t, err)
	require.True(t, allowed, "another account is independent")
}

func TestLimiterRollingHourAndDeniedRequestsDoNotExtendRetention(t *testing.T) {
	server := miniredis.RunT(t)
	start := time.Unix(1700000000, 0).UTC()
	server.SetTime(start)
	limiter := New(testClient(t, server))
	account := uuid.New()
	admit := func(count int) {
		for i := 0; i < count; i++ {
			allowed, err := limiter.Allow(context.Background(), account, Create)
			require.NoError(t, err)
			require.True(t, allowed)
		}
	}
	advance := func(sinceStart time.Duration, duration time.Duration) {
		server.FastForward(duration)
		server.SetTime(start.Add(sinceStart))
	}
	admit(5)
	advance(30*time.Minute, 30*time.Minute)
	admit(5)
	advance(59*time.Minute, 29*time.Minute)
	keys := server.Keys()
	for _, key := range keys {
		require.Equal(t, 31*time.Minute, server.TTL(key))
	}
	allowed, err := limiter.Allow(context.Background(), account, Create)
	require.NoError(t, err)
	require.False(t, allowed)
	for _, key := range keys {
		require.Equal(t, 31*time.Minute, server.TTL(key), "denial must not extend account-key retention")
	}
	advance(time.Hour, time.Minute)
	admit(5) // Only the first five expired; the half-hour admissions still count.
	allowed, err = limiter.Allow(context.Background(), account, Create)
	require.NoError(t, err)
	require.False(t, allowed, "a fixed-window reset would incorrectly allow this")
	advance(2*time.Hour, time.Hour)
	require.Empty(t, server.Keys(), "both state keys expire after an idle hour")
	admit(1)
}

func TestLimiterFailsClosedWithoutRedisOrOnCorruptState(t *testing.T) {
	account := uuid.New()
	for _, limiter := range []*Limiter{nil, New(nil)} {
		allowed, err := limiter.Allow(context.Background(), account, Create)
		require.Error(t, err)
		require.False(t, allowed)
	}
	server := miniredis.RunT(t)
	limiter := New(testClient(t, server))
	allowed, err := limiter.Allow(context.Background(), account, Create)
	require.NoError(t, err)
	require.True(t, allowed)
	// Corrupt the actual sorted set, without depending on its key format.
	for _, key := range server.Keys() {
		kind := server.Type(key)
		if kind == "zset" {
			server.Del(key)
			require.NoError(t, server.Set(key, "not-a-sorted-set"))
		}
	}
	allowed, err = limiter.Allow(context.Background(), account, Create)
	require.Error(t, err)
	require.False(t, allowed)
	server.Close()
	allowed, err = limiter.Allow(context.Background(), uuid.New(), Delete)
	require.Error(t, err)
	require.False(t, allowed, "a Redis outage must not become an unlimited local budget")
}

func TestLimiterRejectsInvalidScopeAndCanceledContext(t *testing.T) {
	server := miniredis.RunT(t)
	limiter := New(testClient(t, server))
	for _, input := range []struct {
		account uuid.UUID
		action  string
	}{{uuid.Nil, Create}, {uuid.New(), "other"}, {uuid.New(), ""}} {
		allowed, err := limiter.Allow(context.Background(), input.account, input.action)
		require.Error(t, err)
		require.False(t, allowed)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	allowed, err := limiter.Allow(ctx, uuid.New(), Create)
	require.ErrorIs(t, err, context.Canceled)
	require.False(t, allowed)
	require.Empty(t, server.Keys())
}
