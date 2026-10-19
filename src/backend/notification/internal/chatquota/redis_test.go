package chatquota

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestQuotaConcurrentLogicalReplayAndSuppression(t *testing.T) {
	ctx := context.Background()
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	testConcurrent(t, ctx, client)
}

func testConcurrent(t *testing.T, ctx context.Context, client *redis.Client) {
	q, err := New(client, Config{Limit: 60, Window: time.Minute})
	require.NoError(t, err)
	recipient, chat := uuid.New(), uuid.New()
	messages := make([]uuid.UUID, 80)
	var wg sync.WaitGroup
	results := make(chan bool, 80)
	errors := make(chan error, 80)
	for i := range messages {
		messages[i] = uuid.New()
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ok, err := q.Admit(ctx, recipient, chat, messages[i])
			results <- ok
			errors <- err
		}(i)
	}
	wg.Wait()
	close(results)
	close(errors)
	allowed := 0
	for err := range errors {
		require.NoError(t, err)
	}
	for ok := range results {
		if ok {
			allowed++
		}
	}
	require.Equal(t, 60, allowed)
	// Every decision is stable even when the bucket is full; replays spend nothing.
	for _, message := range messages {
		_, err := q.Admit(ctx, recipient, chat, message)
		require.NoError(t, err)
	}
	keys := keysFor(recipient, chat, messages[0])
	require.EqualValues(t, 60, client.ZCard(ctx, keys[0]).Val())
	ttl := client.TTL(ctx, keys[1]).Val()
	require.Greater(t, ttl, 191*time.Hour)
	require.LessOrEqual(t, ttl, DecisionTTL)
	other := uuid.New()
	ok, err := q.Admit(ctx, other, chat, messages[0])
	require.NoError(t, err)
	require.True(t, ok)
	// Duplicate concurrency across two independently constructed instances spends once.
	second, err := New(client, Config{Limit: 60, Window: time.Minute})
	require.NoError(t, err)
	duplicateRecipient, duplicateMessage := uuid.New(), uuid.New()
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			instance := q
			if i%2 == 0 {
				instance = second
			}
			allowed, err := instance.Admit(ctx, duplicateRecipient, chat, duplicateMessage)
			if err != nil || !allowed {
				t.Errorf("duplicate admission: %v %v", allowed, err)
			}
		}(i)
	}
	wg.Wait()
	require.EqualValues(t, 1, client.ZCard(ctx, keysFor(duplicateRecipient, chat, duplicateMessage)[0]).Val())
}

func TestQuotaRealRedisConcurrent(t *testing.T) {
	addr := os.Getenv("BE195_REDIS_ADDR")
	if addr == "" {
		t.Skip("owned real Redis runner required")
	}
	client := redis.NewClient(&redis.Options{Addr: addr})
	defer func() { _ = client.Close() }()
	testConcurrent(t, context.Background(), client)
}

func TestQuotaExpiryReplayAndBadState(t *testing.T) {
	ctx := context.Background()
	server := miniredis.RunT(t)
	now := time.Unix(2000000000, 0)
	server.SetTime(now)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	defer func() { _ = client.Close() }()
	q, err := New(client, Config{Limit: 1, Window: time.Minute})
	require.NoError(t, err)
	p, c, a, b := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	yes, err := q.Admit(ctx, p, c, a)
	require.NoError(t, err)
	require.True(t, yes)
	yes, err = q.Admit(ctx, p, c, b)
	require.NoError(t, err)
	require.False(t, yes)
	server.FastForward(time.Minute)
	server.SetTime(now.Add(time.Minute))
	yes, err = q.Admit(ctx, p, c, b)
	require.NoError(t, err)
	require.False(t, yes)
	yes, err = q.Admit(ctx, p, c, a)
	require.NoError(t, err)
	require.True(t, yes)
	require.EqualValues(t, 0, client.ZCard(ctx, keysFor(p, c, a)[0]).Val(), "admitted replay must not reinsert expired score")
	yes, err = q.Admit(ctx, p, c, uuid.New())
	require.NoError(t, err)
	require.True(t, yes)
	require.Equal(t, DecisionTTL-time.Minute, client.TTL(ctx, keysFor(p, c, a)[1]).Val(), "replay must not refresh decision TTL")
	server.FastForward(DecisionTTL)
	server.SetTime(now.Add(DecisionTTL + time.Minute))
	yes, err = q.Admit(ctx, p, c, b)
	require.NoError(t, err)
	require.True(t, yes, "expired suppression may be re-evaluated")
	bad := uuid.New()
	require.NoError(t, client.Set(ctx, keysFor(p, c, bad)[1], "corrupt", time.Hour).Err())
	_, err = q.Admit(ctx, p, c, bad)
	require.Error(t, err)
	require.NoError(t, client.Del(ctx, keysFor(p, c, a)[0]).Err())
	require.NoError(t, client.Set(ctx, keysFor(p, c, a)[0], "wrong type", time.Hour).Err())
	_, err = q.Admit(ctx, p, c, uuid.New())
	require.Error(t, err)
	_, err = q.Admit(ctx, uuid.Nil, c, a)
	require.Error(t, err)
	server.Close()
	_, err = q.Admit(ctx, p, c, uuid.New())
	require.Error(t, err)
}

func TestQuotaConfig(t *testing.T) {
	for _, env := range []map[string]string{{"NOTIFICATION_CHAT_PUSH_LIMIT": "0"}, {"NOTIFICATION_CHAT_PUSH_LIMIT": "bad"}, {"NOTIFICATION_CHAT_PUSH_WINDOW": "-1s"}, {"NOTIFICATION_CHAT_PUSH_WINDOW": "bad"}} {
		_, err := ConfigFromEnv(func(k string) (string, bool) { v, ok := env[k]; return v, ok })
		require.Error(t, err)
	}
	cfg, err := ConfigFromEnv(func(string) (string, bool) { return "", false })
	require.NoError(t, err)
	require.Equal(t, 60, cfg.Limit)
	require.Equal(t, time.Minute, cfg.Window)
}
