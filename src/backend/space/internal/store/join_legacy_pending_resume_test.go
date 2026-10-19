package store

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestLegacyPendingResumeContextGoldenAndBoundaries(t *testing.T) {
	var selection [32]byte
	for i := range selection {
		selection[i] = byte(i)
	}
	attempt := uuid.MustParse("11223344-5566-7788-99aa-bbccddeeff00")
	binding := bytes.Repeat([]byte{42}, 32)
	hash, err := legacyPendingResumeContextSHA256(selection, attempt, binding)
	require.NoError(t, err)
	// Independently calculated with Python hashlib + struct.pack('>I',1)
	// and uuid.UUID(...).bytes, not another Go implementation of the codec.
	require.Equal(t, "423d91121fe393edc0d53bc384e2ec44728aca19d5d3eb17051f17483d7ad7f8", hex.EncodeToString(hash[:]))
	require.NotEqual(t, selection, hash, "execution hash never replaces persisted selection identity")
	for _, name := range []string{"selection", "attempt", "binding"} {
		t.Run(name, func(t *testing.T) {
			s, a, b := selection, attempt, bytes.Clone(binding)
			switch name {
			case "selection":
				s[0] ^= 1
			case "attempt":
				a[0] ^= 1
			case "binding":
				b[0] ^= 1
			}
			changed, e := legacyPendingResumeContextSHA256(s, a, b)
			require.NoError(t, e)
			require.NotEqual(t, hash, changed)
		})
	}
	_, err = legacyPendingResumeContextSHA256([32]byte{}, attempt, binding)
	require.Error(t, err)
	_, err = legacyPendingResumeContextSHA256(selection, uuid.Nil, binding)
	require.Error(t, err)
	_, err = legacyPendingResumeContextSHA256(selection, attempt, binding[:31])
	require.Error(t, err)
	_, err = legacyPendingResumeContextSHA256(selection, attempt, make([]byte, 32))
	require.Error(t, err)
	var missing *JoinLegacyPendingResumeProposal
	_, err = missing.OwnerRequest()
	require.Error(t, err)
	_, err = missing.Seal(nil)
	require.Error(t, err)
}
