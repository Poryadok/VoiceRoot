package partyauthority

import (
	"encoding/hex"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"testing"
)

func authorityMembers() []Member {
	return []Member{{uuid.MustParse("20000000-0000-4000-8000-000000000001"), uuid.MustParse("10000000-0000-4000-8000-000000000001"), 7}, {uuid.MustParse("20000000-0000-4000-8000-000000000002"), uuid.MustParse("10000000-0000-4000-8000-000000000002"), 9}}
}
func TestTupleManifestMatchesFixedContractVector(t *testing.T) {
	members := authorityMembers()
	raw, digest, err := Encode(members)
	require.NoError(t, err)
	require.Equal(t, "00000001000000022000000000004000800000000000000110000000000040008000000000000001000000000000000720000000000040008000000000000002100000000000400080000000000000020000000000000009", hex.EncodeToString(raw))
	require.Equal(t, "8254ac93d2e9130b408c3fc1049b726dbafc0f286d42365854553ec01357c2ba", hex.EncodeToString(digest[:]))
	decoded, err := Decode(raw, digest)
	require.NoError(t, err)
	require.Equal(t, members, decoded)
}
func TestTupleManifestRejectsUnknownOrAmbiguousIdentity(t *testing.T) {
	for _, mutate := range []func([]Member) []Member{func(m []Member) []Member { return nil }, func(m []Member) []Member { m[0].AccountID = uuid.Nil; return m }, func(m []Member) []Member { m[0].ProfileID = uuid.Nil; return m }, func(m []Member) []Member { m[0].SessionEpoch = 0; return m }, func(m []Member) []Member { m[0].SessionEpoch = -1; return m }, func(m []Member) []Member { m[1].ProfileID = m[0].ProfileID; return m }, func(m []Member) []Member { m[1].AccountID = m[0].AccountID; return m }, func(m []Member) []Member { m[0], m[1] = m[1], m[0]; return m }} {
		_, _, err := Encode(mutate(authorityMembers()))
		require.ErrorIs(t, err, ErrInvalid)
	}
}
func TestStoredTupleManifestRejectsCorruptionAndVersionMismatch(t *testing.T) {
	raw, hash, err := Encode(authorityMembers())
	require.NoError(t, err)
	for _, mutate := range []func([]byte) []byte{func(b []byte) []byte { return b[:len(b)-1] }, func(b []byte) []byte { return append(b, 0) }, func(b []byte) []byte { b[3] = 2; return b }, func(b []byte) []byte { b[7] = 0; return b }, func(b []byte) []byte { b[20] ^= 1; return b }} {
		_, err := Decode(mutate(append([]byte(nil), raw...)), hash)
		require.ErrorIs(t, err, ErrInvalid)
	}
	hash[0] ^= 1
	_, err = Decode(raw, hash)
	require.ErrorIs(t, err, ErrInvalid)
}
