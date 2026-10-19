package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

type lastSeenMemory struct {
	values  map[uuid.UUID]int64
	failure error
}

func (m *lastSeenMemory) Observe(_ context.Context, id uuid.UUID, at time.Time) error {
	if m.failure != nil {
		return m.failure
	}
	if at.Unix() > m.values[id] {
		m.values[id] = at.Unix()
	}
	return nil
}
func (m *lastSeenMemory) GetMany(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]int64, error) {
	if m.failure != nil {
		return nil, m.failure
	}
	out := map[uuid.UUID]int64{}
	for _, id := range ids {
		out[id] = m.values[id]
	}
	return out, nil
}

func TestPresenceDurableSurvivesCacheLoss(t *testing.T) {
	ctx := context.Background()
	cache := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: cache.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	durable := &lastSeenMemory{values: map[uuid.UUID]int64{}}
	s := NewDurablePresenceStore(rdb, durable)
	id := uuid.New()
	at := time.Unix(1700000000, 0).UTC()
	for _, status := range []string{"online", "invisible", "offline"} {
		at = at.Add(time.Minute)
		require.NoError(t, s.Upsert(ctx, id, PresenceUpsert{Status: status, Now: at}))
		require.Equal(t, at.Unix(), durable.values[id], "every heartbeat/offline boundary must persist")
	}
	cache.FlushAll()
	// Reconstruct the service after both session and 30-day cache are gone.
	s = NewDurablePresenceStore(rdb, durable)
	got, err := s.Get(ctx, id)
	require.NoError(t, err)
	require.False(t, got.Live)
	require.Equal(t, at.Unix(), got.LastSeenUnix)
	batch, err := s.GetMany(ctx, []uuid.UUID{id, id})
	require.NoError(t, err)
	require.Len(t, batch, 1)
	require.Equal(t, got, batch[id])
	// Older requests and cache entries cannot roll back the durable timestamp.
	require.NoError(t, s.Upsert(ctx, id, PresenceUpsert{Status: "online", GameTitle: "Game", Now: at.Add(-time.Minute)}))
	got, err = s.Get(ctx, id)
	require.NoError(t, err)
	require.True(t, got.Live)
	require.Equal(t, "Game", got.GameTitle)
	require.Equal(t, at.Unix(), got.LastSeenUnix)
}

func TestPresenceDurableFailureDoesNotAcknowledgeOrServeStaleCache(t *testing.T) {
	ctx := context.Background()
	cache := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: cache.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	durable := &lastSeenMemory{values: map[uuid.UUID]int64{}}
	s := NewDurablePresenceStore(rdb, durable)
	id := uuid.New()
	require.NoError(t, s.Upsert(ctx, id, PresenceUpsert{Status: "online", Now: time.Now()}))
	durable.failure = errors.New("durable unavailable")
	_, err := s.UpsertAndGetPrevious(ctx, id, PresenceUpsert{Status: "offline", Now: time.Now()})
	require.ErrorIs(t, err, durable.failure)
	_, err = s.Get(ctx, id)
	require.ErrorIs(t, err, durable.failure)
	_, err = s.GetMany(ctx, []uuid.UUID{id})
	require.ErrorIs(t, err, durable.failure)
}
