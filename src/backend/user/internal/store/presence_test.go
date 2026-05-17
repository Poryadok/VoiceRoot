package store

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestPresenceStore_UpsertAndGet_online(t *testing.T) {
	ctx := context.Background()
	s := miniredis.RunT(t)
	t.Cleanup(func() { s.Close() })
	rdb := redis.NewClient(&redis.Options{Addr: s.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	st := NewPresenceStore(rdb)
	pid := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	now := time.Unix(1700000000, 0).UTC()

	err := st.Upsert(ctx, pid, PresenceUpsert{
		Status:       "online",
		StatusEnum:   1,
		GameTitle:    "Dota 2",
		CustomStatus: "brb",
		CallInfoJSON: `{"x":1}`,
		Now:          now,
	})
	require.NoError(t, err)

	got, err := st.Get(ctx, pid)
	require.NoError(t, err)
	require.True(t, got.Live)
	require.Equal(t, "online", got.Status)
	require.Equal(t, int32(1), got.StatusEnum)
	require.Equal(t, "Dota 2", got.GameTitle)
	require.Equal(t, "brb", got.CustomStatus)
	require.Equal(t, `{"x":1}`, got.CallInfoJSON)
	require.Equal(t, now.Unix(), got.LastActiveUnix)

	// Heartbeat extends TTL on presence hash
	s.FastForward(4 * time.Minute)
	err = st.Upsert(ctx, pid, PresenceUpsert{Status: "idle", StatusEnum: 2, Now: now.Add(4 * time.Minute)})
	require.NoError(t, err)
	require.True(t, s.Exists("voice:user:presence:"+pid.String()))
}

func TestPresenceStore_Get_offlineWithLastSeen(t *testing.T) {
	ctx := context.Background()
	s := miniredis.RunT(t)
	t.Cleanup(func() { s.Close() })
	rdb := redis.NewClient(&redis.Options{Addr: s.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	st := NewPresenceStore(rdb)
	pid := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	now := time.Unix(1800000000, 0).UTC()

	require.NoError(t, st.Upsert(ctx, pid, PresenceUpsert{Status: "online", StatusEnum: 1, Now: now}))
	s.FastForward(6 * time.Minute) // session key expired; last_seen remains

	got, err := st.Get(ctx, pid)
	require.NoError(t, err)
	require.False(t, got.Live)
	require.Empty(t, got.Status)
	require.Equal(t, int32(0), got.StatusEnum)
	require.Equal(t, now.Unix(), got.LastSeenUnix)
}

func TestPresenceStore_GetBulk(t *testing.T) {
	ctx := context.Background()
	s := miniredis.RunT(t)
	t.Cleanup(func() { s.Close() })
	rdb := redis.NewClient(&redis.Options{Addr: s.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	st := NewPresenceStore(rdb)
	a := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	b := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	t0 := time.Unix(1900000000, 0).UTC()

	require.NoError(t, st.Upsert(ctx, a, PresenceUpsert{Status: "dnd", StatusEnum: 3, Now: t0}))
	require.NoError(t, st.Upsert(ctx, b, PresenceUpsert{Status: "online", StatusEnum: 1, Now: t0}))
	s.FastForward(6 * time.Minute)

	out, err := st.GetMany(ctx, []uuid.UUID{a, b})
	require.NoError(t, err)
	require.Len(t, out, 2)
	require.False(t, out[a].Live)
	require.False(t, out[b].Live)
	require.Equal(t, t0.Unix(), out[a].LastSeenUnix)
	require.Equal(t, t0.Unix(), out[b].LastSeenUnix)
}

func TestPresenceStore_lastSeenKeyValue(t *testing.T) {
	ctx := context.Background()
	s := miniredis.RunT(t)
	t.Cleanup(func() { s.Close() })
	rdb := redis.NewClient(&redis.Options{Addr: s.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	pid := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	lsk := lastSeenRedisKey(pid)
	require.NoError(t, rdb.Set(ctx, lsk, strconv.FormatInt(42, 10), time.Hour).Err())

	st := NewPresenceStore(rdb)
	got, err := st.Get(ctx, pid)
	require.NoError(t, err)
	require.False(t, got.Live)
	require.Equal(t, int64(42), got.LastSeenUnix)
}
