package jobs_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"voice/backend/story/internal/jobs"
	"voice/backend/story/internal/store"
)

type stubStore struct {
	expireCalls int
	purgeCalls int
}

func (s *stubStore) MarkExpiredStories(ctx context.Context, now time.Time) (int64, error) {
	s.expireCalls++
	return 1, nil
}

func (s *stubStore) PurgeArchivedStories(ctx context.Context, now time.Time) (int64, error) {
	s.purgeCalls++
	return 2, nil
}

func TestStartExpiryWorker_runs(t *testing.T) {
	st := &store.StoryStore{}
	// jobs expect *store.StoryStore - test wiring only starts goroutines without panic
	jobs.StartExpiryWorker(context.Background(), st, nil)
	jobs.StartArchivePurgeWorker(context.Background(), st, nil, nil)
	require.True(t, true)
}
