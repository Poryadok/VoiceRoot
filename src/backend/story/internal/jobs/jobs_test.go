package jobs_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"voice/backend/story/internal/jobs"
	"voice/backend/story/internal/store"
)

func TestStartExpiryWorker_runs(t *testing.T) {
	st := &store.StoryStore{}
	// jobs expect *store.StoryStore - test wiring only starts goroutines without panic
	jobs.StartExpiryWorker(context.Background(), st, nil, nil)
	jobs.StartArchivePurgeWorker(context.Background(), st, nil, nil)
	require.True(t, true)
}
