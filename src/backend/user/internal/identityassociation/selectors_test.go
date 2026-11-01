package identityassociation

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func keysetForTest(t *testing.T, active uint32, keys map[uint32][]byte) *Keyset {
	t.Helper()
	snapshot := KeysetSnapshot{Revision: 1, ActiveVersion: active, Versions: map[uint32][32]byte{}}
	for version, key := range keys {
		snapshot.Versions[version] = sha256.Sum256(key)
	}
	set, err := NewKeyset(snapshot, keys)
	require.NoError(t, err)
	return set
}

func TestCanonicalUserSelectorGoldenDomains(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	set := keysetForTest(t, 1, map[uint32][]byte{1: key})
	id := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	prepared, err := set.PrepareSelectors(id, id)
	require.NoError(t, err)
	require.Equal(t, "ce97085e9ea4a7f58593c8a1ffa4cd28aeebf4d30073c60741836d5dfa3c8aea", hex.EncodeToString(prepared.aliases[0].account[:]))
	require.Equal(t, "12b1250ec4b5bc9a2a9f448d719873725a857bf36bd3dc9b28656ef2efc08ccb", hex.EncodeToString(prepared.aliases[0].profile[:]))
	require.NotEqual(t, prepared.aliases[0].account, prepared.aliases[0].profile)
}

func TestCustodyDigestBindsCompleteKeyset(t *testing.T) {
	k1, k2 := bytes.Repeat([]byte{1}, 32), bytes.Repeat([]byte{2}, 32)
	set := keysetForTest(t, 1, map[uint32][]byte{2: k2, 1: k1})
	digest, err := set.CustodyDigest()
	require.NoError(t, err)
	require.Equal(t, "bad6df9e4670717541efd3bc198ea51b6fb75c6d6af3189027c5fc288d7ed903", hex.EncodeToString(digest[:]))
	otherOrder := keysetForTest(t, 1, map[uint32][]byte{1: k1, 2: k2})
	otherDigest, err := otherOrder.CustodyDigest()
	require.NoError(t, err)
	require.Equal(t, digest, otherDigest)
	active := keysetForTest(t, 2, map[uint32][]byte{1: k1, 2: k2})
	activeDigest, err := active.CustodyDigest()
	require.NoError(t, err)
	require.NotEqual(t, digest, activeDigest)
	snapshot := set.Snapshot()
	snapshot.Revision++
	revised, err := NewKeyset(snapshot, map[uint32][]byte{1: k1, 2: k2})
	require.NoError(t, err)
	revisedDigest, err := revised.CustodyDigest()
	require.NoError(t, err)
	require.NotEqual(t, digest, revisedDigest)
	missing := keysetForTest(t, 1, map[uint32][]byte{1: k1})
	missingDigest, err := missing.CustodyDigest()
	require.NoError(t, err)
	require.NotEqual(t, digest, missingDigest)
	var nilKeys *Keyset
	_, err = nilKeys.CustodyDigest()
	require.ErrorIs(t, err, ErrKeysetIncomplete)
}

func TestCompleteRetainedKeysetAndPrivatePreparation(t *testing.T) {
	k1, k2 := bytes.Repeat([]byte{1}, 32), bytes.Repeat([]byte{2}, 32)
	snapshot := KeysetSnapshot{Revision: 2, ActiveVersion: 2, Versions: map[uint32][32]byte{1: sha256.Sum256(k1), 2: sha256.Sum256(k2)}}
	_, err := NewKeyset(snapshot, map[uint32][]byte{2: k2})
	require.ErrorIs(t, err, ErrKeysetIncomplete)
	set, err := NewKeyset(snapshot, map[uint32][]byte{1: k1, 2: k2})
	require.NoError(t, err)
	p, err := set.PrepareSelectors(uuid.New(), uuid.New())
	require.NoError(t, err)
	require.Len(t, p.aliases, 2)
	require.False(t, (PreparedSelectors{}).valid())
	_, err = set.PrepareSelectors(uuid.Nil, uuid.New())
	require.Error(t, err)
	// Caller mutation cannot change an already admitted keyset or preparation.
	k1[0] ^= 1
	delete(snapshot.Versions, 1)
	again, err := set.PrepareSelectors(p.profileID, p.accountID)
	require.NoError(t, err)
	require.Equal(t, p.aliases, again.aliases)
}
