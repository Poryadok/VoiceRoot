package catalogasset

import (
	"bytes"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestLifecycleReplayKeysSeparatePurposesActorsAndRetainRotation(t *testing.T) {
	actor := ActorPair{AccountID: uuid.New(), ProfileID: uuid.New()}
	operation, source, file := uuid.New(), uuid.New(), uuid.New()
	keys := map[string][]byte{"old": bytes.Repeat([]byte{1}, 32), "next": bytes.Repeat([]byte{2}, 32)}
	ring, err := NewReplayKeys("next", keys)
	require.NoError(t, err)
	operations, err := ring.Operation(actor, operation)
	require.NoError(t, err)
	require.Len(t, operations, 2)
	require.Equal(t, "next", operations[0].KeyVersion)
	old, err := NewReplayKeys("old", map[string][]byte{"old": keys["old"]})
	require.NoError(t, err)
	previous, err := old.Operation(actor, operation)
	require.NoError(t, err)
	require.Equal(t, previous[0], operations[1], "rotation must still find old permanent fence")
	account, err := ring.Account(actor.AccountID)
	require.NoError(t, err)
	profile, err := ring.Profile(actor)
	require.NoError(t, err)
	sources, err := ring.Source(actor, source)
	require.NoError(t, err)
	asset, err := ring.Asset(actor, source, "fixture", "asset-1", file)
	require.NoError(t, err)
	seen := map[string]bool{}
	for _, digests := range [][]ReplayDigest{operations, account, profile, sources, asset} {
		for _, digest := range digests {
			require.Len(t, digest.HMAC, 32)
			require.False(t, seen[string(digest.HMAC)])
			seen[string(digest.HMAC)] = true
			require.NotContains(t, string(digest.HMAC), actor.ProfileID.String())
		}
	}
	other := actor
	other.ProfileID = uuid.New()
	different, err := ring.Operation(other, operation)
	require.NoError(t, err)
	require.NotEqual(t, operations, different)
	changed, err := ring.Asset(actor, source, "fixture", "asset-2", file)
	require.NoError(t, err)
	require.NotEqual(t, asset, changed)
	keys["next"][0] = 99
	again, err := ring.Operation(actor, operation)
	require.NoError(t, err)
	require.Equal(t, operations, again, "caller key mutation must not change replay authority")
	again[0].HMAC[0] ^= 0xff
	again, err = ring.Operation(actor, operation)
	require.NoError(t, err)
	require.Equal(t, operations, again, "returned digest mutation must not change key state")
}

func TestLifecycleReplayKeysRejectMissingKeysAndInvalidIdentities(t *testing.T) {
	for _, keys := range []map[string][]byte{nil, {"active": {1}}, {"bad/key": bytes.Repeat([]byte{1}, 32)}} {
		_, err := NewReplayKeys("active", keys)
		require.Error(t, err)
	}
	ring, err := NewReplayKeys("active", map[string][]byte{"active": bytes.Repeat([]byte{1}, 32)})
	require.NoError(t, err)
	_, err = ring.Operation(ActorPair{}, uuid.New())
	require.Error(t, err)
	_, err = ring.Asset(ActorPair{AccountID: uuid.New(), ProfileID: uuid.New()}, uuid.New(), "fixture", "", uuid.New())
	require.Error(t, err)
	_, err = ring.Account(uuid.Nil)
	require.Error(t, err)
}
