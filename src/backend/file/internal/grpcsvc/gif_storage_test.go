package grpcsvc

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/file/internal/store"
)

type gifCleanupDeleter struct {
	fail bool
	keys []string
}

func (d *gifCleanupDeleter) DeleteObject(_ context.Context, key string) error {
	if d.fail {
		d.fail = false
		return errors.New("ambiguous storage failure")
	}
	d.keys = append(d.keys, key)
	return nil
}

func gifQueuedFixture(t *testing.T) (context.Context, *store.FilesStore, uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	pool := startFileGatePostgres(t, ctx)
	files := store.NewFilesStore(pool)
	id := uuid.New()
	_, err := files.InsertPendingFile(ctx, store.FileRow{ID: id, UploaderProfileID: uuid.New(), OriginalName: "source.gif", MimeType: "image/gif", SizeBytes: 42, R2Key: "source/" + id.String(), Status: "pending_upload", FileType: "image", UploadIntent: "gif", ScanResult: "pending"})
	require.NoError(t, err)
	_, err = files.ConfirmGIFUpload(ctx, id, strings.Repeat("a", 64))
	require.NoError(t, err)
	return ctx, files, id
}

func gifRepresentations(job *store.GIFJob) (*store.Representation, *store.Representation) {
	return &store.Representation{Key: job.FullKey, MIMEType: "video/mp4", SizeBytes: 1024, SHA256: strings.Repeat("b", 64), Filename: "animation.mp4", ProcessorVersion: store.GIFProcessorVersion}, &store.Representation{Key: job.ThumbnailKey, MIMEType: "image/webp", SizeBytes: 128, SHA256: strings.Repeat("c", 64), Filename: "thumbnail.webp", ProcessorVersion: store.GIFProcessorVersion}
}

func TestGIFJobRecoveryFencesStaleAttemptAndPreservesSource(t *testing.T) {
	ctx, files, id := gifQueuedFixture(t)
	first, err := files.ClaimGIFJob(ctx)
	require.NoError(t, err)
	require.NotNil(t, first)
	concurrent, err := files.ClaimGIFJob(ctx)
	require.NoError(t, err)
	require.Nil(t, concurrent)
	_, err = files.Pool.Exec(ctx, `UPDATE file_gif_jobs SET lease_until=clock_timestamp()-interval '1 second' WHERE file_id=$1`, id)
	require.NoError(t, err)
	second, err := files.ClaimGIFJob(ctx)
	require.NoError(t, err)
	require.NotNil(t, second)
	require.NotEqual(t, first.Token, second.Token)
	full, thumb := gifRepresentations(first)
	committed, err := files.FinishGIFJob(ctx, first, full, thumb, 16, 16, 2, false)
	require.NoError(t, err)
	require.False(t, committed)
	full, thumb = gifRepresentations(second)
	committed, err = files.FinishGIFJob(ctx, second, full, thumb, 16, 16, 2, false)
	require.NoError(t, err)
	require.True(t, committed)
	row, err := files.GetFileByID(ctx, id)
	require.NoError(t, err)
	require.Equal(t, "image/gif", row.MimeType)
	require.EqualValues(t, 42, row.SizeBytes)
	require.Equal(t, strings.Repeat("a", 64), *row.SHA256Hash)
	require.Equal(t, "video", row.FileType)
	require.Equal(t, "ready", row.Status)
	require.Equal(t, full, row.DeliveredFull)
	var retained int
	require.NoError(t, files.Pool.QueryRow(ctx, `SELECT count(*) FROM file_gif_cleanup WHERE retained AND file_id=$1`, id).Scan(&retained))
	require.Equal(t, 2, retained)
	var ready int
	require.NoError(t, files.Pool.QueryRow(ctx, `SELECT count(*) FROM file_preview_event_outbox WHERE file_id=$1`, id).Scan(&ready))
	require.Equal(t, 2, ready, "processing and ready snapshots must commit once")
}

func TestGIFConcurrentConfirmationCreatesOneDurableJob(t *testing.T) {
	ctx := context.Background()
	pool := startFileGatePostgres(t, ctx)
	files := store.NewFilesStore(pool)
	id := uuid.New()
	_, err := files.InsertPendingFile(ctx, store.FileRow{ID: id, UploaderProfileID: uuid.New(), OriginalName: "source.gif", MimeType: "image/gif", SizeBytes: 42, R2Key: id.String(), Status: "pending_upload", FileType: "image", UploadIntent: "gif", ScanResult: "pending"})
	require.NoError(t, err)
	start := make(chan struct{})
	results := make(chan error, 8)
	for i := 0; i < 8; i++ {
		go func() { <-start; _, err := files.ConfirmGIFUpload(ctx, id, strings.Repeat("a", 64)); results <- err }()
	}
	close(start)
	for i := 0; i < 8; i++ {
		require.NoError(t, <-results)
	}
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM file_gif_jobs WHERE file_id=$1`, id).Scan(&count))
	require.Equal(t, 1, count)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM file_preview_event_outbox WHERE file_id=$1`, id).Scan(&count))
	require.Equal(t, 1, count)
}

func TestGIFJobCannotResurrectDeletedFile(t *testing.T) {
	ctx, files, id := gifQueuedFixture(t)
	job, err := files.ClaimGIFJob(ctx)
	require.NoError(t, err)
	require.NoError(t, files.MarkDeleted(ctx, id))
	full, thumb := gifRepresentations(job)
	ok, err := files.FinishGIFJob(ctx, job, full, thumb, 16, 16, 1, false)
	require.NoError(t, err)
	require.False(t, ok)
	row, err := files.GetFileByID(ctx, id)
	require.NoError(t, err)
	require.Equal(t, "deleted", row.Status)
	require.Nil(t, row.DeliveredFull)
}

func TestGIFDurableCleanupRetriesWithoutDeletingWinner(t *testing.T) {
	ctx, files, id := gifQueuedFixture(t)
	first, err := files.ClaimGIFJob(ctx)
	require.NoError(t, err)
	_, err = files.Pool.Exec(ctx, `UPDATE file_gif_jobs SET lease_until=clock_timestamp()-interval '1 second' WHERE file_id=$1`, id)
	require.NoError(t, err)
	second, err := files.ClaimGIFJob(ctx)
	require.NoError(t, err)
	full, thumb := gifRepresentations(second)
	ok, err := files.FinishGIFJob(ctx, second, full, thumb, 16, 16, 1, false)
	require.NoError(t, err)
	require.True(t, ok)
	_, err = files.Pool.Exec(ctx, `UPDATE file_gif_cleanup SET not_before=clock_timestamp()-interval '1 second' WHERE file_id=$1`, id)
	require.NoError(t, err)
	deleter := &gifCleanupDeleter{fail: true}
	ok, err = files.CleanupGIFOnce(ctx, deleter)
	require.Error(t, err)
	require.False(t, ok)
	for i := 0; i < 3; i++ {
		ok, err = files.CleanupGIFOnce(ctx, deleter)
		require.NoError(t, err)
		require.True(t, ok)
	}
	ok, err = files.CleanupGIFOnce(ctx, deleter)
	require.NoError(t, err)
	require.False(t, ok)
	require.Contains(t, deleter.keys, first.FullKey)
	require.Contains(t, deleter.keys, first.ThumbnailKey)
	require.NotContains(t, deleter.keys, second.FullKey)
	require.NotContains(t, deleter.keys, second.ThumbnailKey)
	var completed int
	require.NoError(t, files.Pool.QueryRow(ctx, `SELECT count(*) FROM file_gif_cleanup WHERE file_id=$1 AND completed_at IS NOT NULL`, id).Scan(&completed))
	require.Equal(t, 2, completed, "public original stays unresolved without a storage write fence")
}

func TestGIFOriginalCleanupRechecksSignedPUTReplay(t *testing.T) {
	ctx, files, _ := gifQueuedFixture(t)
	job, err := files.ClaimGIFJob(ctx)
	require.NoError(t, err)
	full, thumb := gifRepresentations(job)
	ok, err := files.FinishGIFJob(ctx, job, full, thumb, 16, 16, 1, false)
	require.NoError(t, err)
	require.True(t, ok)
	objects := &gifWorkerObjects{data: map[string][]byte{job.Row.R2Key: []byte("original")}}
	ok, err = files.CleanupGIFOnce(ctx, objects)
	require.NoError(t, err)
	require.True(t, ok)
	require.NotContains(t, objects.data, job.Row.R2Key)
	var pending bool
	require.NoError(t, files.Pool.QueryRow(ctx, `SELECT completed_at IS NULL FROM file_gif_cleanup WHERE object_key=$1`, job.Row.R2Key).Scan(&pending))
	require.True(t, pending)
	objects.data[job.Row.R2Key] = []byte("replayed signed PUT")
	_, err = files.Pool.Exec(ctx, `UPDATE file_gif_cleanup SET not_before=clock_timestamp()-interval '1 second',recheck_until=clock_timestamp()-interval '1 second' WHERE object_key=$1`, job.Row.R2Key)
	require.NoError(t, err)
	ok, err = files.CleanupGIFOnce(ctx, objects)
	require.NoError(t, err)
	require.True(t, ok)
	require.NotContains(t, objects.data, job.Row.R2Key)
	require.NoError(t, files.Pool.QueryRow(ctx, `SELECT completed_at IS NULL FROM file_gif_cleanup WHERE object_key=$1`, job.Row.R2Key).Scan(&pending))
	require.True(t, pending, "URL age does not fence an already accepted signed PUT")
}

func TestGIFDeletionReconcilesOriginalPUTReplay(t *testing.T) {
	ctx, files, id := gifQueuedFixture(t)
	for _, name := range []string{"000004_file_reference_lifecycle.up.sql", "000005_reference_gc_receipts.up.sql"} {
		applyFileGateSQL(t, ctx, files.Pool, filepath.Join("src", "backend", "migrations", "file_db", name))
	}
	row, err := files.GetFileByID(ctx, id)
	require.NoError(t, err)
	objects := &gifWorkerObjects{data: map[string][]byte{row.R2Key: []byte("original")}}
	deleted, err := files.DeleteLegacyFile(ctx, id, nil, objects)
	require.NoError(t, err)
	require.True(t, deleted)
	objects.data[row.R2Key] = []byte("replayed PUT after blob GC_COMPLETE")
	cleaned, err := files.CleanupGIFOnce(ctx, objects)
	require.NoError(t, err)
	require.True(t, cleaned, "terminal GIF must retain original reconciliation authority")
	require.NotContains(t, objects.data, row.R2Key)
	var pending bool
	require.NoError(t, files.Pool.QueryRow(ctx, `SELECT completed_at IS NULL FROM file_gif_cleanup WHERE object_key=$1`, row.R2Key).Scan(&pending))
	require.True(t, pending)
}

func TestGIFReadyAndDescriptorRollbackWithPreviewOutboxFailure(t *testing.T) {
	ctx, files, id := gifQueuedFixture(t)
	job, err := files.ClaimGIFJob(ctx)
	require.NoError(t, err)
	_, err = files.Pool.Exec(ctx, `CREATE FUNCTION reject_gif_preview() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected preview persistence failure'; END $$;
 CREATE TRIGGER reject_gif_preview BEFORE INSERT ON file_preview_event_outbox FOR EACH ROW EXECUTE FUNCTION reject_gif_preview();`)
	require.NoError(t, err)
	full, thumb := gifRepresentations(job)
	ok, err := files.FinishGIFJob(ctx, job, full, thumb, 16, 16, 1, false)
	require.Error(t, err)
	require.False(t, ok)
	row, err := files.GetFileByID(ctx, id)
	require.NoError(t, err)
	require.Equal(t, "processing", row.Status)
	require.Nil(t, row.DeliveredFull)
	require.Nil(t, row.ConvertedR2Key)
	_, err = files.Pool.Exec(ctx, `DROP TRIGGER reject_gif_preview ON file_preview_event_outbox`)
	require.NoError(t, err)
	ok, err = files.FinishGIFJob(ctx, job, full, thumb, 16, 16, 1, false)
	require.NoError(t, err)
	require.True(t, ok)
}
