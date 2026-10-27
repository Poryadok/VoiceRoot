package grpcsvc

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
	"voice/backend/file/internal/gifproc"
	"voice/backend/file/internal/jobs"
	"voice/backend/file/internal/store"
)

type gifWorkerObjects struct {
	data          map[string][]byte
	failThumbnail bool
}

func (o *gifWorkerObjects) ReadObject(_ context.Context, key string, max int64) ([]byte, error) {
	b, ok := o.data[key]
	if !ok || int64(len(b)) > max {
		return nil, errors.New("read failure")
	}
	return b, nil
}
func (o *gifWorkerObjects) PutObject(_ context.Context, key, mime string, b []byte) error {
	o.data[key] = append([]byte(nil), b...)
	if o.failThumbnail && strings.HasSuffix(key, "thumb.webp") {
		o.failThumbnail = false
		return errors.New("ambiguous thumbnail write")
	}
	return nil
}
func (o *gifWorkerObjects) DeleteObject(_ context.Context, key string) error {
	delete(o.data, key)
	return nil
}

func TestGIFWorkerRecoversPartialWritesWithoutDeletingWinner(t *testing.T) {
	ctx := context.Background()
	pool := startFileGatePostgres(t, ctx)
	files := store.NewFilesStore(pool)
	id := uuid.New()
	raw := []byte("source bytes verified before fake processing")
	_, err := files.InsertPendingFile(ctx, store.FileRow{ID: id, UploaderProfileID: uuid.New(), OriginalName: "source.gif", MimeType: "image/gif", SizeBytes: int64(len(raw)), R2Key: "source/" + id.String(), Status: "pending_upload", FileType: "image", UploadIntent: "gif", ScanResult: "pending"})
	require.NoError(t, err)
	row, err := files.ConfirmGIFUpload(ctx, id, shaHex(raw))
	require.NoError(t, err)
	objects := &gifWorkerObjects{data: map[string][]byte{row.R2Key: raw}, failThumbnail: true}
	// The fake isolates durable storage recovery. Real codec behavior has its own
	// mandatory runtime-image suite; these bytes do not qualify MP4 encoding.
	convert := func(context.Context, []byte) (gifproc.Output, error) {
		return gifproc.Output{Full: []byte(strings.Repeat("output", 100)), Thumbnail: []byte("thumbnail"), Width: 16, Height: 16, DurationSeconds: 1}, nil
	}
	worked, err := jobs.RunGIFProcessingOnce(ctx, files, objects, convert)
	require.True(t, worked)
	require.Error(t, err)
	row, err = files.GetFileByID(ctx, id)
	require.NoError(t, err)
	require.Equal(t, "processing", row.Status)
	require.Nil(t, row.DeliveredFull)
	_, err = pool.Exec(ctx, `UPDATE file_gif_jobs SET lease_until=clock_timestamp()-interval '1 second',next_attempt_at=clock_timestamp() WHERE file_id=$1`, id)
	require.NoError(t, err)
	worked, err = jobs.RunGIFProcessingOnce(ctx, files, objects, convert)
	require.NoError(t, err)
	require.True(t, worked)
	row, err = files.GetFileByID(ctx, id)
	require.NoError(t, err)
	require.Equal(t, "ready", row.Status)
	require.Greater(t, row.DeliveredFull.SizeBytes, row.SizeBytes)
	_, err = pool.Exec(ctx, `UPDATE file_gif_cleanup SET not_before=clock_timestamp()-interval '1 second' WHERE file_id=$1`, id)
	require.NoError(t, err)
	for i := 0; i < 3; i++ {
		worked, err = files.CleanupGIFOnce(ctx, objects)
		require.NoError(t, err)
		require.True(t, worked)
	}
	require.Len(t, objects.data, 2)
	require.Contains(t, objects.data, row.DeliveredFull.Key)
	require.Contains(t, objects.data, row.DeliveredThumbnail.Key)
	require.NotContains(t, objects.data, row.R2Key)
}

func TestGIFWorkerMarksMalformedInputFailedDurably(t *testing.T) {
	ctx := context.Background()
	pool := startFileGatePostgres(t, ctx)
	files := store.NewFilesStore(pool)
	id := uuid.New()
	raw := []byte("malformed GIF")
	_, err := files.InsertPendingFile(ctx, store.FileRow{ID: id, UploaderProfileID: uuid.New(), OriginalName: "source.gif", MimeType: "image/gif", SizeBytes: int64(len(raw)), R2Key: id.String(), Status: "pending_upload", FileType: "image", UploadIntent: "gif", ScanResult: "pending"})
	require.NoError(t, err)
	_, err = files.ConfirmGIFUpload(ctx, id, shaHex(raw))
	require.NoError(t, err)
	objects := &gifWorkerObjects{data: map[string][]byte{id.String(): raw}}
	worked, err := jobs.RunGIFProcessingOnce(ctx, files, objects, nil)
	require.NoError(t, err)
	require.True(t, worked)
	row, err := files.GetFileByID(ctx, id)
	require.NoError(t, err)
	require.Equal(t, "failed", row.Status)
	var state string
	require.NoError(t, pool.QueryRow(ctx, `SELECT state FROM file_gif_jobs WHERE file_id=$1`, id).Scan(&state))
	require.Equal(t, "failed", state)
	worked, err = jobs.RunGIFProcessingOnce(ctx, files, objects, nil)
	require.NoError(t, err)
	require.False(t, worked)
}
