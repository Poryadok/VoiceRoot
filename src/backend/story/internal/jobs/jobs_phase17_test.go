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

	"voice/backend/story/internal/jobs"
	"voice/backend/story/internal/store"
	"voice/backend/pkg/integrationtest"
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
	root := filepath.Join(filepath.Dir(file), "..", "..", "..", "migrations", "story_db", "000001_init.up.sql")
	b, err := os.ReadFile(root)
	require.NoError(t, err)
	return string(b)
}

// TestArchivePurgeWorker_invokesFileDeleter documents Phase 17 archive cleanup:
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
