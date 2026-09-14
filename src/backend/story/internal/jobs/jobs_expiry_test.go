package jobs_test

import (
	"context"
	"errors"
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
	errs    []error
}

func (r *recordingFileDeleter) DeleteFile(_ context.Context, fileID string) error {
	r.deleted = append(r.deleted, fileID)
	if len(r.errs) != 0 {
		err := r.errs[0]
		r.errs = r.errs[1:]
		return err
	}
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

type blockingFileDeleter struct {
	started  chan struct{}
	release  chan struct{}
	finished chan struct{}
}

func (d *blockingFileDeleter) DeleteFile(context.Context, string) error {
	d.started <- struct{}{}
	<-d.release
	d.finished <- struct{}{}
	return nil
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

func TestArchivePurgeOutbox_stageSurvivesRestart(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	st, ctx := startArchivePurgeStore(t)
	storyID, mediaID := seedExpiredMediaStory(t, ctx, st)
	batch, err := st.StageArchivePurgeBatch(ctx, 1)
	require.NoError(t, err)
	require.EqualValues(t, 1, batch.Stories)
	_, err = st.GetStory(ctx, storyID)
	require.ErrorIs(t, err, store.ErrNotFound)
	ops, err := st.ClaimMediaDeletion(ctx, 1, time.Minute)
	require.NoError(t, err)
	require.Len(t, ops, 1)
	require.Equal(t, mediaID, ops[0].MediaFileID)
	ok, err := st.CompleteMediaDeletion(ctx, ops[0].OperationID, ops[0].LeaseToken)
	require.NoError(t, err)
	require.True(t, ok)
}

func TestArchivePurgeOutbox_failureRequeuesAndStaleLeaseCannotComplete(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	st, ctx := startArchivePurgeStore(t)
	_, _ = seedExpiredMediaStory(t, ctx, st)
	_, err := st.StageArchivePurgeBatch(ctx, 1)
	require.NoError(t, err)
	first, err := st.ClaimMediaDeletion(ctx, 1, time.Minute)
	require.NoError(t, err)
	require.Len(t, first, 1)
	ok, err := st.FailMediaDeletion(ctx, first[0].OperationID, first[0].LeaseToken, errors.New("definite failure"))
	require.NoError(t, err)
	require.True(t, ok)
	_, err = st.Pool.Exec(ctx, `UPDATE story_media_deletion_outbox SET available_at=now()-interval '1 second'`)
	require.NoError(t, err)
	second, err := st.ClaimMediaDeletion(ctx, 1, time.Minute)
	require.NoError(t, err)
	require.Len(t, second, 1)
	ok, err = st.CompleteMediaDeletion(ctx, first[0].OperationID, first[0].LeaseToken)
	require.NoError(t, err)
	require.False(t, ok)
	ok, err = st.CompleteMediaDeletion(ctx, second[0].OperationID, second[0].LeaseToken)
	require.NoError(t, err)
	require.True(t, ok)
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

func TestArchivePurgeOutbox_postCommitCrashRecovery(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	st, ctx := startArchivePurgeStore(t)
	storyID, mediaID := seedExpiredMediaStory(t, ctx, st)
	batch, err := st.StageArchivePurgeBatch(ctx, 1)
	require.NoError(t, err)
	require.EqualValues(t, 1, batch.Stories)
	_, err = st.GetStory(ctx, storyID)
	require.ErrorIs(t, err, store.ErrNotFound)

	// Simulate a process crash after File accepted the deletion but before the
	// dispatcher CAS acknowledgement: the durable row must be leased again.
	first, err := st.ClaimMediaDeletion(ctx, 1, time.Minute)
	require.NoError(t, err)
	require.Len(t, first, 1)
	require.Equal(t, mediaID, first[0].MediaFileID)
	firstFileCall := &recordingFileDeleter{}
	require.NoError(t, firstFileCall.DeleteFile(ctx, mediaID.String()))
	require.Equal(t, []string{mediaID.String()}, firstFileCall.deleted,
		"the first dispatcher reached File before its process crashed")
	_, err = st.Pool.Exec(ctx, `UPDATE story_media_deletion_outbox SET lease_until=now()-interval '1 second' WHERE operation_id=$1`, first[0].OperationID)
	require.NoError(t, err)

	deleter := &recordingFileDeleter{}
	n, err := jobs.RunArchivePurgeOnce(ctx, st, deleter, time.Now().UTC())
	require.NoError(t, err)
	require.Zero(t, n, "recovery dispatch must not restage a deleted story")
	require.Equal(t, []string{mediaID.String()}, deleter.deleted,
		"recovery retries the immutable id; File NotFound is mapped to success by clients.FileDeleter")
	var count int
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM story_media_deletion_outbox WHERE operation_id=$1`, storyID).Scan(&count))
	require.Zero(t, count)
}

func TestArchivePurgeOutbox_retriesFailureAndAmbiguousFileResult(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	st, ctx := startArchivePurgeStore(t)
	storyID, mediaID := seedExpiredMediaStory(t, ctx, st)
	deleter := &recordingFileDeleter{errs: []error{errors.New("transport response lost"), errors.New("definite File failure")}}

	_, err := jobs.RunArchivePurgeOnce(ctx, st, deleter, time.Now().UTC())
	require.NoError(t, err)
	for attempt, wantError := range []string{"transport response lost", "definite File failure"} {
		var attempts int64
		var lastError *string
		var lastErrorAt *time.Time
		var availableAt, updatedAt time.Time
		require.NoError(t, st.Pool.QueryRow(ctx, `SELECT attempt_count,last_error,last_error_at,available_at,updated_at FROM story_media_deletion_outbox WHERE operation_id=$1`, storyID).Scan(&attempts, &lastError, &lastErrorAt, &availableAt, &updatedAt))
		require.EqualValues(t, attempt+1, attempts)
		require.NotNil(t, lastError)
		require.Contains(t, *lastError, wantError)
		require.NotNil(t, lastErrorAt)
		require.Equal(t, time.Second<<attempt, availableAt.Sub(updatedAt),
			"failure must persist capped attempt-dependent exponential backoff using DB time")
		_, err = st.Pool.Exec(ctx, `UPDATE story_media_deletion_outbox SET available_at=now()-interval '1 second' WHERE operation_id=$1`, storyID)
		require.NoError(t, err)
		if attempt == 0 {
			_, err = jobs.RunArchivePurgeOnce(ctx, st, deleter, time.Now().UTC())
			require.NoError(t, err)
		}
	}
	_, err = jobs.RunArchivePurgeOnce(ctx, st, deleter, time.Now().UTC())
	require.NoError(t, err)
	require.Equal(t, []string{mediaID.String(), mediaID.String(), mediaID.String()}, deleter.deleted,
		"both an ambiguous response and a definite failure retry by the immutable file id")
	var count int
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM story_media_deletion_outbox WHERE operation_id=$1`, storyID).Scan(&count))
	require.Zero(t, count, "successful third delivery removes only its leased durable operation")
}

func TestArchivePurgeOutbox_fileDeadlineRequeuesWithoutBlockingLaterTick(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	st, ctx := startArchivePurgeStore(t)
	storyID, _ := seedExpiredMediaStory(t, ctx, st)
	timedOut := &blockingFileDeleter{started: make(chan struct{}, 1), release: make(chan struct{}), finished: make(chan struct{}, 1)}
	runDone := make(chan error, 1)
	go func() {
		_, err := jobs.RunArchivePurgeOnceWithFileTimeout(ctx, st, timedOut, time.Now().UTC(), 20*time.Millisecond)
		runDone <- err
	}()
	<-timedOut.started
	require.NoError(t, <-runDone, "deadline failure is recorded as retryable work rather than blocking the dispatcher")

	var attempts int64
	var lastError *string
	var leaseToken *uuid.UUID
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT attempt_count,last_error,lease_token FROM story_media_deletion_outbox WHERE operation_id=$1`, storyID).Scan(&attempts, &lastError, &leaseToken))
	require.EqualValues(t, 1, attempts)
	require.NotNil(t, lastError)
	require.Contains(t, *lastError, context.DeadlineExceeded.Error())
	require.Nil(t, leaseToken, "timed-out File call releases its lease for a later dispatcher tick")

	// The timed-out implementation ignores cancellation. A second tick must
	// still return immediately and persist its retry instead of spawning a
	// second stuck File goroutine.
	secondStoryID, _ := seedExpiredMediaStory(t, ctx, st)
	_, err := jobs.RunArchivePurgeOnceWithFileTimeout(ctx, st, timedOut, time.Now().UTC(), time.Second)
	require.NoError(t, err)
	var busyError *string
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT last_error FROM story_media_deletion_outbox WHERE operation_id=$1`, secondStoryID).Scan(&busyError))
	require.NotNil(t, busyError)
	require.Contains(t, *busyError, "already in flight")

	close(timedOut.release)
	<-timedOut.finished
}

func TestArchivePurgeOutbox_twoDispatchersRejectStaleAck(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	st, ctx := startArchivePurgeStore(t)
	storyID, mediaID := seedExpiredMediaStory(t, ctx, st)
	_, err := st.StageArchivePurgeBatch(ctx, 1)
	require.NoError(t, err)
	firstDeleter := &purgeBarrierFileDeleter{started: make(chan struct{}, 1), release: make(chan struct{}), store: st, storyID: storyID}
	firstDone := make(chan error, 1)
	go func() {
		_, runErr := jobs.RunArchivePurgeOnce(ctx, st, firstDeleter, time.Now().UTC())
		firstDone <- runErr
	}()
	<-firstDeleter.started
	_, err = st.Pool.Exec(ctx, `UPDATE story_media_deletion_outbox SET lease_until=now()-interval '1 second' WHERE operation_id=$1`, storyID)
	require.NoError(t, err)
	secondDeleter := &recordingFileDeleter{}
	_, err = jobs.RunArchivePurgeOnce(ctx, st, secondDeleter, time.Now().UTC())
	require.NoError(t, err)
	require.Equal(t, []string{mediaID.String()}, secondDeleter.deleted)
	close(firstDeleter.release)
	require.NoError(t, <-firstDone, "stale completion is a harmless CAS miss after a second dispatcher finished")
	var count int
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM story_media_deletion_outbox WHERE operation_id=$1`, storyID).Scan(&count))
	require.Zero(t, count)
}

func TestArchivePurgeOutbox_finalUnlinkConcurrentWithAdd(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	st, ctx := startArchivePurgeStore(t)
	storyID, mediaID := seedExpiredMediaStory(t, ctx, st)
	profileID := uuid.New()
	_, err := st.Pool.Exec(ctx, `UPDATE stories SET author_profile_id=$2 WHERE id=$1`, storyID, profileID)
	require.NoError(t, err)
	first, err := st.CreateHighlight(ctx, profileID, "First", "everyone")
	require.NoError(t, err)
	second, err := st.CreateHighlight(ctx, profileID, "Second", "everyone")
	require.NoError(t, err)
	require.NoError(t, st.AddToHighlight(ctx, first.ID, profileID, storyID))

	// Holding the Story lock creates a deterministic barrier: both membership
	// transactions acquire their owning Highlight first and then contend for the
	// same Story. Whichever commits first, the Add commit must leave one link.
	lockTx, err := st.Pool.Begin(ctx)
	require.NoError(t, err)
	var locked int
	err = lockTx.QueryRow(ctx, `SELECT 1 FROM stories WHERE id=$1 FOR UPDATE`, storyID).Scan(&locked)
	require.NoError(t, err)
	arrived := make(chan struct{}, 2)
	st.BeforeHighlightStoryLock = func() { arrived <- struct{}{} }
	defer func() { st.BeforeHighlightStoryLock = nil }()
	start := make(chan struct{})
	removeDone := make(chan error, 1)
	addDone := make(chan error, 1)
	go func() {
		<-start
		removeDone <- st.RemoveFromHighlight(ctx, first.ID, profileID, storyID)
	}()
	go func() {
		<-start
		addDone <- st.AddToHighlight(ctx, second.ID, profileID, storyID)
	}()
	close(start)
	<-arrived
	<-arrived
	require.NoError(t, lockTx.Commit(ctx))
	require.NoError(t, <-removeDone)
	require.NoError(t, <-addDone)
	var links int
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM highlight_stories WHERE story_id=$1`, storyID).Scan(&links))
	require.Equal(t, 1, links)

	deleter := &recordingFileDeleter{}
	n, err := jobs.RunArchivePurgeOnce(ctx, st, deleter, time.Now().UTC())
	require.NoError(t, err)
	require.Zero(t, n, "the concurrent Add commit preserves a link when the prior final link is removed")
	require.Empty(t, deleter.deleted)
	require.NoError(t, st.RemoveFromHighlight(ctx, second.ID, profileID, storyID))

	// Last unlink has a distinct outcome: only after the final link commits may
	// the scanner stage an irreversible File deletion.
	n, err = jobs.RunArchivePurgeOnce(ctx, st, deleter, time.Now().UTC())
	require.NoError(t, err)
	require.EqualValues(t, 1, n)
	require.Equal(t, []string{mediaID.String()}, deleter.deleted)
}

func TestArchivePurgeOutbox_addFirstMakesPurgeSkipWithoutFileCall(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	st, ctx := startArchivePurgeStore(t)
	storyID, _ := seedExpiredMediaStory(t, ctx, st)
	profileID := uuid.New()
	_, err := st.Pool.Exec(ctx, `UPDATE stories SET author_profile_id=$2 WHERE id=$1`, storyID, profileID)
	require.NoError(t, err)
	highlight, err := st.CreateHighlight(ctx, profileID, "Permanent", "everyone")
	require.NoError(t, err)
	require.NoError(t, st.AddToHighlight(ctx, highlight.ID, profileID, storyID))
	deleter := &recordingFileDeleter{}
	n, err := jobs.RunArchivePurgeOnce(ctx, st, deleter, time.Now().UTC())
	require.NoError(t, err)
	require.Zero(t, n)
	require.Empty(t, deleter.deleted, "an Add commit before scanner selection wins and prevents File deletion")
	_, err = st.GetStory(ctx, storyID)
	require.NoError(t, err)
}

func TestArchivePurgeOutbox_textOnlyAndScannerRerunAreIdempotent(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	st, ctx := startArchivePurgeStore(t)
	textID := uuid.New()
	_, err := st.Pool.Exec(ctx, `INSERT INTO stories (id,author_profile_id,type,text_content,mention_profile_ids,visibility,expires_at,archived_until,created_at,expired_at) VALUES ($1,$2,'text','gone','[]','everyone',now()-interval '32 days',now()-interval '1 hour',now()-interval '32 days',now()-interval '31 days')`, textID, uuid.New())
	require.NoError(t, err)
	mediaStoryID, mediaID := seedExpiredMediaStory(t, ctx, st)
	batch, err := st.StageArchivePurgeBatch(ctx, 100)
	require.NoError(t, err)
	require.EqualValues(t, 2, batch.Stories)
	secondBatch, err := st.StageArchivePurgeBatch(ctx, 100)
	require.NoError(t, err)
	require.Zero(t, secondBatch.Stories, "scanner rerun after committed logical deletion cannot duplicate work")
	var textOps, mediaOps int
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM story_media_deletion_outbox WHERE operation_id=$1`, textID).Scan(&textOps))
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM story_media_deletion_outbox WHERE operation_id=$1 AND story_id=$1 AND media_file_id=$2`, mediaStoryID, mediaID).Scan(&mediaOps))
	require.Zero(t, textOps, "text-only archive cleanup has no File outbox operation")
	require.Equal(t, 1, mediaOps, "one immutable story id produces exactly one media deletion operation")
}
