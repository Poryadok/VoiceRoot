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

func TestReplayEvidenceCollectorRejectsWrongPhaseAndPreservesPagedOrder(t *testing.T) {
	collector, err := NewReplayEvidenceCollector(2, 2)
	require.NoError(t, err)
	require.NoError(t, collector.AddSnapshot(1, []byte("one")))
	require.NoError(t, collector.AddSnapshot(2, []byte("two")))
	require.NoError(t, collector.AddJournal(3, []byte("three")))
	require.Error(t, collector.AddJournal(2, []byte("duplicate snapshot")))
	require.Error(t, collector.AddSnapshot(4, []byte("past H")))
}
