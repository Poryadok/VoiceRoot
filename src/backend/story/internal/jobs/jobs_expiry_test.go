package jobs_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/pkg/integrationtest"
	"voice/backend/story/internal/jobs"
	"voice/backend/story/internal/store"
)

type recordingFileDeleter struct {
	mu      sync.Mutex
	deleted []string
}

func (r *recordingFileDeleter) DeleteFile(_ context.Context, fileID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.deleted = append(r.deleted, fileID)
	return nil
}

func (r *recordingFileDeleter) deletedIDs() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.deleted...)
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
	require.Equal(t, []string{mediaID.String()}, deleter.deletedIDs(),
		"archive purge worker must invoke FileDeleter.DeleteFile for purged story media IDs")
}

func TestStartArchivePurgeWorker_runsOnceOnStartup(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	dbCtx := context.Background()
	workerCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pool := integrationtest.StartPostgres(t, dbCtx, "storystartuppurge", "")
	_, err := pool.Exec(dbCtx, migrationSQL(t))
	require.NoError(t, err)
	st := &store.StoryStore{Pool: pool}
	mediaID := uuid.New()
	storyID := uuid.New()
	_, err = pool.Exec(dbCtx, `
INSERT INTO stories (
  id, author_profile_id, type, media_file_id, mention_profile_ids,
  visibility, expires_at, archived_until, created_at, expired_at
) VALUES ($1, $2, 'photo', $3, '[]'::jsonb, 'everyone', now() - interval '2 days', now() - interval '1 hour', now() - interval '2 days', now() - interval '1 day')`,
		storyID, uuid.New(), mediaID)
	require.NoError(t, err)

	deleter := &recordingFileDeleter{}
	jobs.StartArchivePurgeWorker(workerCtx, st, deleter, nil)

	require.Eventually(t, func() bool {
		var exists bool
		err := pool.QueryRow(dbCtx, `SELECT EXISTS(SELECT 1 FROM stories WHERE id = $1)`, storyID).Scan(&exists)
		return err == nil && !exists
	}, 5*time.Second, 25*time.Millisecond, "startup must not wait for the daily ticker")
	require.Equal(t, []string{mediaID.String()}, deleter.deletedIDs())

	_, err = jobs.RunArchivePurgeOnce(dbCtx, st, deleter, time.Now().UTC())
	require.NoError(t, err)
	require.Equal(t, []string{mediaID.String()}, deleter.deletedIDs(), "a later run is idempotent after startup cleanup")
}

func TestStartArchivePurgeWorker_stopsWithCanceledContext(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	checkCtx := context.Background()
	pool := integrationtest.StartPostgres(t, checkCtx, "storystartuppurgecancel", "")
	_, err := pool.Exec(checkCtx, migrationSQL(t))
	require.NoError(t, err)
	st := &store.StoryStore{Pool: pool}
	storyID := uuid.New()
	_, err = pool.Exec(checkCtx, `
INSERT INTO stories (
  id, author_profile_id, type, mention_profile_ids,
  visibility, expires_at, archived_until, created_at, expired_at
) VALUES ($1, $2, 'text', '[]'::jsonb, 'everyone', now() - interval '2 days', now() - interval '1 hour', now() - interval '2 days', now() - interval '1 day')`,
		storyID, uuid.New())
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	jobs.StartArchivePurgeWorker(ctx, st, nil, nil)

	time.Sleep(100 * time.Millisecond)
	var exists bool
	err = pool.QueryRow(checkCtx, `SELECT EXISTS(SELECT 1 FROM stories WHERE id = $1)`, storyID).Scan(&exists)
	require.NoError(t, err)
	require.True(t, exists, "a canceled service context must prevent archive cleanup work")
}
