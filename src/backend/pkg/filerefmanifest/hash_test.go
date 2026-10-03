package filerefmanifest

import (
	"encoding/hex"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestEmptyProducerIsBoundToOwnerSpaceAndGeneration(t *testing.T) {
	deletion := uuid.MustParse("00000000-0000-4000-8000-000000000002")
	space := uuid.MustParse("00000000-0000-4000-8000-000000000001")
	digest, err := Hash(deletion, space, 7, 2, nil)
	require.NoError(t, err)
	require.Equal(t, "31aa90394029f964ac1a37799075a05be375153251cc3f75e1583f4af6d48697", hex.EncodeToString(digest))
	for _, args := range []struct {
		g uint64
		p uint32
	}{{8, 2}, {7, 1}, {7, 3}} {
		other, err := Hash(deletion, space, args.g, args.p, nil)
		require.NoError(t, err)
		require.NotEqual(t, digest, other)
	}
	_, err = Hash(deletion, uuid.Nil, 7, 2, nil)
	require.Error(t, err)
}
