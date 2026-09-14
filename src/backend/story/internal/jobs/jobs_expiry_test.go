package jobs_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/pkg/integrationtest"
	"voice/backend/story/internal/jobs"
	"voice/backend/story/internal/store"
)

type recordingFileDeleter struct {
	deleted []string
}

func (r *recordingFileDeleter) DeleteFile(_ context.Context, fileID string) error {
	r.deleted = append(r.deleted, fileID)
	return nil
}

func migrationSQL(t *testing.T) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	dir := filepath.Join(filepath.Dir(file), "..", "..", "..", "migrations", "story_db")
	b1, err := os.ReadFile(filepath.Join(dir, "000001_init.up.sql"))
	require.NoError(t, err)
	b2, err := os.ReadFile(filepath.Join(dir, "000002_visibility_audience.up.sql"))
	require.NoError(t, err)
	b3, err := os.ReadFile(filepath.Join(dir, "000003_hidden_from_feed.up.sql"))
	require.NoError(t, err)
	return string(b1) + "\n" + string(b2) + "\n" + string(b3)
}

// TestArchivePurgeWorker_invokesFileDeleter documents stories (docs/features/stories.md) archive cleanup:
// purge worker must call FileDeleter.DeleteFile for each purged story media_file_id.
func TestArchivePurgeWorker_invokesFileDeleter(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "storypurge", "")
	_, err := pool.Exec(ctx, migrationSQL(t))
	require.NoError(t, err)
	st := &store.StoryStore{Pool: pool}
	mediaID := uuid.New()
	_, err = pool.Exec(ctx, `
INSERT INTO stories (
  id, author_profile_id, type, media_file_id, mention_profile_ids,
  visibility, expires_at, archived_until, created_at, expired_at
) VALUES ($1, $2, 'photo', $3, '[]'::jsonb, 'everyone', now() - interval '2 days', now() - interval '1 hour', now() - interval '2 days', now() - interval '1 day')`,
		uuid.New(), uuid.New(), mediaID)
	require.NoError(t, err)

	deleter := &recordingFileDeleter{}
	n, err := jobs.RunArchivePurgeOnce(ctx, st, deleter, time.Now().UTC())
	require.NoError(t, err)
	require.Equal(t, int64(1), n)
	require.Equal(t, []string{mediaID.String()}, deleter.deleted,
		"archive purge worker must invoke FileDeleter.DeleteFile for purged story media IDs")
}

func TestArchivePurgeWorker_retainsHighlightedMediaUntilUnlinked(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "storyhighlightpurge", "")
	_, err := pool.Exec(ctx, migrationSQL(t))
	require.NoError(t, err)
	st := &store.StoryStore{Pool: pool}
	profileID := uuid.New()
	storyID := uuid.New()
	mediaID := uuid.New()
	highlightID := uuid.New()
	_, err = pool.Exec(ctx, `
INSERT INTO stories (
  id, author_profile_id, type, media_file_id, mention_profile_ids,
  visibility, expires_at, archived_until, created_at, expired_at
) VALUES ($1, $2, 'photo', $3, '[]'::jsonb, 'everyone', now() - interval '32 days', now() - interval '1 hour', now() - interval '32 days', now() - interval '31 days')`,
		storyID, profileID, mediaID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO highlights (id, profile_id, name, visibility) VALUES ($1, $2, 'Permanent', 'everyone')`, highlightID, profileID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO highlight_stories (highlight_id, story_id) VALUES ($1, $2)`, highlightID, storyID)
	require.NoError(t, err)

	deleter := &recordingFileDeleter{}
	n, err := jobs.RunArchivePurgeOnce(ctx, st, deleter, time.Now().UTC())
	require.NoError(t, err)
	require.Zero(t, n)
	require.Empty(t, deleter.deleted, "retained Highlight media must not be deleted")
	var links int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM highlight_stories WHERE highlight_id = $1 AND story_id = $2`, highlightID, storyID).Scan(&links))
	require.Equal(t, 1, links)

	require.NoError(t, st.RemoveFromHighlight(ctx, highlightID, profileID, storyID))
	n, err = jobs.RunArchivePurgeOnce(ctx, st, deleter, time.Now().UTC())
	require.NoError(t, err)
	require.Equal(t, int64(1), n)
	require.Equal(t, []string{mediaID.String()}, deleter.deleted)
}
