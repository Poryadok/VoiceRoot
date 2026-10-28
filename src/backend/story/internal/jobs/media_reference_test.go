package jobs_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/story/internal/jobs"
	"voice/backend/story/internal/store"
)

func TestArchivePurgeAbandonedFailureDoesNotBlockRelease(t *testing.T) {
	if testing.Short() {
		t.Skip("isolated PostgreSQL")
	}
	st, ctx := startArchivePurgeStore(t)
	story, _ := seedExpiredMediaStory(t, ctx, st)
	_, err := st.Pool.Exec(ctx, `INSERT INTO story_media_reference_intents(story_id,media_file_id,available_at) VALUES($1,$2,now()-interval '1 second')`, uuid.New(), uuid.New())
	require.NoError(t, err)
	file := &recordingFileDeleter{errs: []error{errors.New("abandoned File unavailable"), nil}}
	count, err := jobs.RunArchivePurgeOnce(ctx, st, file, time.Now())
	require.Error(t, err)
	require.EqualValues(t, 1, count, "one failed abandoned claim must not starve unrelated archive cleanup")
	_, err = st.GetStory(ctx, story)
	require.ErrorIs(t, err, store.ErrNotFound)
	require.Len(t, file.deletedIDs(), 2, "an unrelated exact release still dispatches")
}
