package jobs_test

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
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

type purgeBarrierFileDeleter struct {
	started        chan struct{}
	release        chan struct{}
	store          *store.StoryStore
	storyID        uuid.UUID
	callbackLookup error
	deleted        []string
}

// archivePurgeOutboxScenario is the test-only seam every hosted outbox test will
// receive from the GREEN implementation. Its channels are explicit barriers;
// tests must advance DB time through the implementation's injected clock and
// must never use a sleep to observe a lease or an external File call.
type archivePurgeOutboxScenario struct {
	stageCommitted  chan struct{}
	addCommitted    chan struct{}
	purgeAttempted  chan struct{}
	fileCallStarted chan struct{}
	fileCallRelease chan struct{}
	firstLeaseLost  chan struct{}
	secondLeaseDone chan struct{}
	scannerRerun    chan struct{}
}

func newArchivePurgeOutboxScenario() archivePurgeOutboxScenario {
	return archivePurgeOutboxScenario{
		stageCommitted:  make(chan struct{}, 1),
		addCommitted:    make(chan struct{}, 1),
		purgeAttempted:  make(chan struct{}, 1),
		fileCallStarted: make(chan struct{}, 1),
		fileCallRelease: make(chan struct{}),
		firstLeaseLost:  make(chan struct{}, 1),
		secondLeaseDone: make(chan struct{}, 1),
		scannerRerun:    make(chan struct{}, 1),
	}
}

func requireHostedArchivePurgeScaffold(t *testing.T, scenario string) {
	t.Helper()
	if testing.Short() {
		t.Skip("hosted-only archive outbox scaffold: " + scenario)
	}
	t.Skip("RED scaffold: wire " + scenario + " to StageArchivePurgeBatch and the leased dispatcher")
}

func startArchivePurgeStore(t *testing.T) (*store.StoryStore, context.Context) {
	t.Helper()
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "storyoutbox", "")
	_, err := pool.Exec(ctx, migrationSQL(t))
	require.NoError(t, err)
	return &store.StoryStore{Pool: pool}, ctx
}

func seedExpiredMediaStory(t *testing.T, ctx context.Context, st *store.StoryStore) (uuid.UUID, uuid.UUID) {
	t.Helper()
	storyID, mediaID := uuid.New(), uuid.New()
	_, err := st.Pool.Exec(ctx, `INSERT INTO stories (id,author_profile_id,type,media_file_id,mention_profile_ids,visibility,expires_at,archived_until,created_at,expired_at) VALUES ($1,$2,'photo',$3,'[]','everyone',now()-interval '32 days',now()-interval '1 hour',now()-interval '32 days',now()-interval '31 days')`, storyID, uuid.New(), mediaID)
	require.NoError(t, err)
	return storyID, mediaID
}

func (d *purgeBarrierFileDeleter) DeleteFile(ctx context.Context, fileID string) error {
	_, d.callbackLookup = d.store.GetStory(ctx, d.storyID)
	d.started <- struct{}{}
	<-d.release
	d.deleted = append(d.deleted, fileID)
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
	b4, err := os.ReadFile(filepath.Join(dir, "000004_archive_purge_outbox.up.sql"))
	require.NoError(t, err)
	return string(b1) + "\n" + string(b2) + "\n" + string(b3) + "\n" + string(b4)
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

// TestArchivePurgeWorker_addDuringPurgeCannotLeaveBrokenHighlight is a deterministic
// regression contract for docs/features/stories.md. The current list -> File -> delete
// flow is intentionally RED: AddToHighlight currently succeeds while File deletion is
// blocked, then the media is removed. The DB-first outbox implementation must instead
// make Add observe NotFound after the logical purge commits, before File is called.
func TestArchivePurgeWorker_addDuringPurgeCannotLeaveBrokenHighlight(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "storypurgeaddbarrier", "")
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

	deleter := &purgeBarrierFileDeleter{
		started: make(chan struct{}, 1),
		release: make(chan struct{}),
		store:   st,
		storyID: storyID,
	}
	purgeDone := make(chan error, 1)
	go func() {
		_, err := jobs.RunArchivePurgeOnce(ctx, st, deleter, time.Now().UTC())
		purgeDone <- err
	}()
	<-deleter.started
	addErr := st.AddToHighlight(ctx, highlightID, profileID, storyID)
	close(deleter.release)
	require.NoError(t, <-purgeDone)
	require.ErrorIs(t, addErr, store.ErrNotFound, "purge that reached its linearization point must win before File deletion")
	require.Equal(t, []string{mediaID.String()}, deleter.deleted, "File deletion runs only after the purge has made Add return NotFound")
	require.ErrorIs(t, deleter.callbackLookup, store.ErrNotFound,
		"DeleteFile callback entry must observe the typed logical-deletion result")
}

// TestArchivePurgeOutboxMigrationContract is intentionally RED until migration 000004
// supplies Sol's Story-owned durable outbox schema. It keeps the outbox/lease contract
// executable without requiring a local database harness.
func TestArchivePurgeOutboxMigrationContract(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	dir := filepath.Join(filepath.Dir(file), "..", "..", "..", "migrations", "story_db")
	migration, err := os.ReadFile(filepath.Join(dir, "000004_archive_purge_outbox.up.sql"))
	require.NoError(t, err)
	sql := string(migration)
	for _, want := range []string{
		"highlight_stories (story_id)",
		"stories (archived_until, id)",
		"WHERE expired_at IS NOT NULL",
		"CREATE TABLE story_media_deletion_outbox",
		"operation_id UUID PRIMARY KEY",
		"story_id UUID NOT NULL",
		"CHECK (operation_id = story_id)",
		"media_file_id UUID NOT NULL",
		"available_at TIMESTAMPTZ NOT NULL",
		"lease_token UUID",
		"lease_until TIMESTAMPTZ",
		"attempt_count BIGINT NOT NULL",
		"last_error TEXT",
		"last_error_at TIMESTAMPTZ",
		"created_at TIMESTAMPTZ NOT NULL",
		"updated_at TIMESTAMPTZ NOT NULL",
	} {
		require.Contains(t, sql, want)
	}
	require.Regexp(t, regexp.MustCompile(`(?s)CREATE INDEX.*ON story_media_deletion_outbox.*\(available_at`), sql,
		"ready/lease claim index must make available outbox work claimable")
	require.NotRegexp(t, regexp.MustCompile(`(?is)\bREFERENCES\s+stories\b`), sql,
		"outbox must survive DB-first deletion and therefore cannot FK to stories")
}

// The scenarios below deliberately compile during RED while the durable outbox
// API does not yet exist. They reserve test names, channel barriers, and exact
// hosted-only invariants before implementation; GREEN work replaces the skip
// helper with concrete StoryStore/job calls without changing the assertions.
func TestArchivePurgeOutbox_postCommitCrashRecovery(t *testing.T) {
	s := newArchivePurgeOutboxScenario()
	_ = s.stageCommitted
	requireHostedArchivePurgeScaffold(t, "post-commit crash: restart dispatcher, retain absent Story, and deliver one durable operation")
}

func TestArchivePurgeOutbox_retriesFailureAndAmbiguousFileResult(t *testing.T) {
	s := newArchivePurgeOutboxScenario()
	_ = s.fileCallStarted
	_ = s.fileCallRelease
	requireHostedArchivePurgeScaffold(t, "definite and ambiguous File failures: persist retry/backoff and finish idempotently")
}

func TestArchivePurgeOutbox_twoDispatchersRejectStaleAck(t *testing.T) {
	s := newArchivePurgeOutboxScenario()
	_ = s.firstLeaseLost
	_ = s.secondLeaseDone
	requireHostedArchivePurgeScaffold(t, "two dispatcher lease race: reclaim expired lease and reject stale acknowledgement")
}

func TestArchivePurgeOutbox_finalUnlinkConcurrentWithAdd(t *testing.T) {
	s := newArchivePurgeOutboxScenario()
	_ = s.stageCommitted
	_ = s.fileCallStarted
	requireHostedArchivePurgeScaffold(t, "final unlink versus Add: committed link retains media or purge makes Add NotFound")
}

func TestArchivePurgeOutbox_addFirstMakesPurgeSkipWithoutFileCall(t *testing.T) {
	s := newArchivePurgeOutboxScenario()
	_ = s.addCommitted
	_ = s.purgeAttempted
	_ = s.fileCallStarted
	requireHostedArchivePurgeScaffold(t, "Add-first versus purge: after Add commit, StageArchivePurgeBatch skips the Story and FileDeleter has zero calls")
}

func TestArchivePurgeOutbox_textOnlyAndScannerRerunAreIdempotent(t *testing.T) {
	s := newArchivePurgeOutboxScenario()
	_ = s.stageCommitted
	_ = s.scannerRerun
	requireHostedArchivePurgeScaffold(t, "text-only plus scanner rerun: logical delete has no outbox row; media operation uses immutable story ID and is not duplicated")
}
