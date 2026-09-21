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

func TestReadinessEvidenceDetectsMutationOmissionAndReordering(t *testing.T) {
	baseline, err := NewReadinessEvidence(1, 0)
	require.NoError(t, err)
	require.NoError(t, baseline.Add(1, []byte("one")))
	require.NoError(t, baseline.Add(2, []byte("two")))

	mutated, _ := NewReadinessEvidence(1, 0)
	require.NoError(t, mutated.Add(1, []byte("one")))
	require.NoError(t, mutated.Add(2, []byte("TWO")))
	require.NotEqual(t, baseline.Digest, mutated.Digest)

	omitted, _ := NewReadinessEvidence(1, 0)
	require.NoError(t, omitted.Add(1, []byte("one")))
	require.Error(t, omitted.Add(3, []byte("three")))

	reordered, _ := NewReadinessEvidence(1, 0)
	require.NoError(t, reordered.Add(2, []byte("two")))
	require.Error(t, reordered.Add(1, []byte("one")))
}

func TestReplayEvidenceResumeMatchesUninterruptedPagination(t *testing.T) {
	full, _ := NewReplayEvidenceCollector(1, 0)
	require.NoError(t, full.AddJournal(1, []byte("one")))
	require.NoError(t, full.AddJournal(2, []byte("two")))
	partial, _ := NewReplayEvidenceCollector(1, 0)
	require.NoError(t, partial.AddJournal(1, []byte("one")))
	resumed, err := ResumeReplayEvidenceCollector(partial.Evidence)
	require.NoError(t, err)
	require.NoError(t, resumed.AddJournal(2, []byte("two")))
	require.Equal(t, full.Evidence, resumed.Evidence)
}
