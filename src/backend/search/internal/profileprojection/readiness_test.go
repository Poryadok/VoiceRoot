package profileprojection

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestReadinessEvidenceBindsDomainGenerationAndContiguousEvents(t *testing.T) {
	a, err := NewReadinessEvidence(2, 9)
	require.NoError(t, err)
	require.NoError(t, a.Add(10, []byte("one")))
	require.NoError(t, a.Add(11, []byte("two")))
	b, err := NewReadinessEvidence(2, 9)
	require.NoError(t, err)
	require.NoError(t, b.Add(10, []byte("one")))
	require.NoError(t, b.Add(11, []byte("two")))
	require.Equal(t, a, b)
	require.Error(t, b.Add(13, []byte("gap")))
	c, _ := NewReadinessEvidence(3, 9)
	require.NoError(t, c.Add(10, []byte("one")))
	require.NotEqual(t, a.Digest, c.Digest)
}
