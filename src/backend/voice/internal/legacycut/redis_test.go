package legacycut

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"voice/backend/voice/internal/retirementproof"
	"voice/backend/voice/internal/store"
)

func TestLegacyRedisCapturesRealInstanceAndRejectsReplacement(t *testing.T) {
	addr := os.Getenv("VOICE_LEGACY_TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("owned isolated real Redis required")
	}
	ctx := context.Background()
	client := redis.NewClient(&redis.Options{Addr: addr})
	t.Cleanup(func() { _ = client.Close() })
	info, err := client.Info(ctx, "server").Result()
	require.NoError(t, err)
	identity, err := RedisInstanceIdentity(info)
	require.NoError(t, err)
	prefix := "legacy-capture-" + uuid.NewString() + ":"
	raw, err := json.Marshal(store.Call{RoomID: "room"})
	require.NoError(t, err)
	require.NoError(t, client.Set(ctx, prefix+"call:room", raw, 0).Err())
	endpoints := RedisEndpoints{Clients: map[string]*redis.Client{"historical": client}}
	e := Endpoint{ID: "historical", ResourceIdentity: identity, Database: 0, Prefix: prefix}
	snapshot, err := endpoints.Capture(ctx, e)
	require.NoError(t, err)
	require.Len(t, snapshot.Entries, 1)
	require.Equal(t, string(raw), snapshot.Entries[0].Value)
	changed := e
	changed.ResourceIdentity = retirementproof.Hash("other", []byte("replaced instance"))
	_, err = endpoints.Capture(ctx, changed)
	require.ErrorIs(t, err, retirementproof.ErrConflict)
	changed = e
	changed.Database++
	_, err = endpoints.Capture(ctx, changed)
	require.ErrorIs(t, err, retirementproof.ErrConflict)
	// TTL loss leaves a dangling locator; it is evidence, not a clean inventory.
	require.NoError(t, client.Set(ctx, prefix+"session:profile", "expired-room", 0).Err())
	diagnostic, err := endpoints.Capture(ctx, e)
	require.Error(t, err)
	require.Len(t, diagnostic.Entries, 2)
	require.EqualValues(t, 2, client.Exists(ctx, prefix+"call:room", prefix+"session:profile").Val())
}
