package auditretention

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type cleanerStub struct {
	results []int64
	err     error
	calls   int
}

func (s *cleanerStub) CleanupExpiredAuditLog(context.Context, int) (int64, error) {
	s.calls++
	if s.err != nil {
		return 0, s.err
	}
	result := s.results[0]
	s.results = s.results[1:]
	return result, nil
}

func TestWorkerRunOnceDrainsFullBatches(t *testing.T) {
	cleaner := &cleanerStub{results: []int64{100, 100, 7}}
	worker := Worker{Cleaner: cleaner, BatchSize: 100}

	deleted, err := worker.RunOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(207), deleted)
	require.Equal(t, 3, cleaner.calls)
}

func TestWorkerRunOnceStopsOnCleanupFailure(t *testing.T) {
	want := errors.New("database unavailable")
	cleaner := &cleanerStub{err: want}
	worker := Worker{Cleaner: cleaner, BatchSize: 100}

	_, err := worker.RunOnce(context.Background())
	require.ErrorIs(t, err, want)
}
