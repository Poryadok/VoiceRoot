package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"
	filev1 "voice.app/voice/file/v1"
	"voice/backend/file/internal/jobs"
	"voice/backend/file/internal/pdfproc"
	"voice/backend/file/internal/store"
)

type pdfStorage struct {
	keys   map[string][]byte
	fail   bool
	writes int
}

func (p *pdfStorage) ReadObject(_ context.Context, k string, _ int64) ([]byte, error) {
	return p.keys[k], nil
}
func (p *pdfStorage) PutObject(_ context.Context, k, _ string, b []byte) error {
	p.keys[k] = b
	p.writes++
	if p.fail {
		return errors.New("lost upload response")
	}
	return nil
}
func (p *pdfStorage) DeleteObject(_ context.Context, k string) error { delete(p.keys, k); return nil }

type pdfRenderer struct {
	err   error
	calls int
}

func (p *pdfRenderer) Render(context.Context, []byte) ([]byte, error) {
	p.calls++
	return []byte("bounded png"), p.err
}

func TestPDFConfirmQueuesThenWorkerRetriesAndExpiryPurgesBothKeys(t *testing.T) {
	ctx := context.Background()
	pool := startFilePostgres(t, ctx)
	defer pool.Close()
	files := store.NewFilesStore(pool)
	id, owner := uuid.New(), uuid.New()
	expiry := time.Now().Add(time.Hour)
	_, err := files.InsertPendingFile(ctx, store.FileRow{ID: id, UploaderProfileID: owner, OriginalName: "manual.pdf", MimeType: "application/pdf", SizeBytes: int64(len(defaultUploadBytes)), R2Key: "original.pdf", Status: "pending_upload", FileType: "document", ScanResult: "pending", ExpiresAt: &expiry})
	require.NoError(t, err)
	client := startFileGRPC(t, pool, &recordingPresigner{})
	call := metadata.NewOutgoingContext(ctx, metadata.Pairs("x-voice-profile-id", owner.String()))
	response, err := client.ConfirmUpload(call, &filev1.ConfirmUploadRequest{FileId: id.String(), Sha256Hash: defaultUploadHash})
	require.NoError(t, err)
	require.Equal(t, "processing", response.FileMetadata.Status)
	storage := &pdfStorage{keys: map[string][]byte{"original.pdf": defaultUploadBytes}, fail: true}
	renderer := &pdfRenderer{}
	worked, err := jobs.RunPDFThumbnailOnce(ctx, files, storage, renderer)
	require.True(t, worked)
	require.Error(t, err)
	row, err := files.GetFileByID(ctx, id)
	require.NoError(t, err)
	require.Equal(t, "processing", row.Status)
	require.NotNil(t, row.ThumbnailR2Key)
	key := *row.ThumbnailR2Key
	require.Contains(t, storage.keys, key)
	var blobKey string
	require.NoError(t, pool.QueryRow(ctx, `SELECT thumbnail_r2_key FROM file_blobs WHERE blob_id=(SELECT blob_id FROM files WHERE id=$1)`, id).Scan(&blobKey))
	require.Equal(t, key, blobKey)
	storage.fail = false
	worked, err = jobs.RunPDFThumbnailOnce(ctx, files, storage, renderer)
	require.True(t, worked)
	require.NoError(t, err)
	row, err = files.GetFileByID(ctx, id)
	require.NoError(t, err)
	require.Equal(t, "ready", row.Status)
	require.Equal(t, key, *row.ThumbnailR2Key)
	require.Nil(t, row.ConvertedR2Key)
	require.Equal(t, "original.pdf", row.R2Key)
	require.Equal(t, int64(len(defaultUploadBytes)), row.SizeBytes)
	require.Equal(t, expiry.Unix(), row.ExpiresAt.Unix())
	require.Equal(t, defaultUploadHash, *row.SHA256Hash)
	worked, err = jobs.RunPDFThumbnailOnce(ctx, files, storage, renderer)
	require.False(t, worked)
	require.NoError(t, err)
	require.Equal(t, 2, renderer.calls)
	var readyEvents int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM file_preview_event_outbox WHERE file_id=$1`, id).Scan(&readyEvents))
	require.Equal(t, 2, readyEvents)
	_, err = pool.Exec(ctx, `UPDATE files SET expires_at=now()-interval '1 day' WHERE id=$1`, id)
	require.NoError(t, err)
	_, err = jobs.RunExpiryPurgeOnce(ctx, files, storage, nil, time.Now())
	require.NoError(t, err)
	require.Empty(t, storage.keys)
}

func TestPDFWorkerMalformedE2EAndCrashOrphanCleanup(t *testing.T) {
	ctx := context.Background()
	pool := startFilePostgres(t, ctx)
	defer pool.Close()
	files := store.NewFilesStore(pool)
	insert := func(e2e bool) uuid.UUID {
		id := uuid.New()
		_, err := files.InsertPendingFile(ctx, store.FileRow{ID: id, UploaderProfileID: uuid.New(), OriginalName: "doc.pdf", MimeType: "application/pdf", SizeBytes: 8, R2Key: id.String(), Status: "pending_upload", FileType: "document", ScanResult: "pending", IsE2E: e2e})
		require.NoError(t, err)
		_, err = files.ConfirmUpload(ctx, id, "hash")
		require.NoError(t, err)
		return id
	}
	encrypted := insert(true)
	id := insert(false)
	storage := &pdfStorage{keys: map[string][]byte{id.String(): []byte("original")}}
	renderer := &pdfRenderer{err: pdfproc.ErrInvalidPDF}
	worked, err := jobs.RunPDFThumbnailOnce(ctx, files, storage, renderer)
	require.True(t, worked)
	require.NoError(t, err)
	row, err := files.GetFileByID(ctx, id)
	require.NoError(t, err)
	require.Equal(t, "failed", row.Status)
	require.Zero(t, storage.writes)
	row, err = files.GetFileByID(ctx, encrypted)
	require.NoError(t, err)
	require.Nil(t, row.ThumbnailR2Key)
	// A crash after reservation and a partial PUT still leaves a tracked GC key.
	orphan := insert(false)
	storage.keys[orphan.String()] = []byte("original")
	renderer.err = errors.New("temporary renderer failure")
	worked, err = jobs.RunPDFThumbnailOnce(ctx, files, storage, renderer)
	require.True(t, worked)
	require.Error(t, err)
	row, err = files.GetFileByID(ctx, orphan)
	require.NoError(t, err)
	require.NotNil(t, row.ThumbnailR2Key)
	delete(storage.keys, id.String())
	storage.keys[row.R2Key] = []byte("original")
	storage.keys[*row.ThumbnailR2Key] = []byte("partial")
	completed, err := files.DeleteLegacyFile(ctx, orphan, nil, storage)
	require.NoError(t, err)
	require.True(t, completed)
	require.Empty(t, storage.keys)
	worked, err = jobs.RunPDFThumbnailOnce(ctx, files, storage, renderer)
	require.False(t, worked)
	require.NoError(t, err)
}

func TestPDFWorkerSerializesDeleteAndSkipsConcurrentClaim(t *testing.T) {
	ctx := context.Background()
	pool := startFilePostgres(t, ctx)
	defer pool.Close()
	files := store.NewFilesStore(pool)
	id := uuid.New()
	_, err := files.InsertPendingFile(ctx, store.FileRow{ID: id, UploaderProfileID: uuid.New(), OriginalName: "doc.pdf", MimeType: "application/pdf", SizeBytes: 8, R2Key: "original", Status: "pending_upload", FileType: "document", ScanResult: "pending"})
	require.NoError(t, err)
	_, err = files.ConfirmUpload(ctx, id, "hash")
	require.NoError(t, err)
	started, release := make(chan struct{}), make(chan struct{})
	processed := make(chan error, 1)
	storage := &pdfStorage{keys: map[string][]byte{"original": []byte("original")}}
	go func() {
		_, err := files.ProcessPDFThumbnailOnce(ctx, func(_ context.Context, row store.FileRow) (bool, error) {
			close(started)
			<-release
			storage.keys[*row.ThumbnailR2Key] = []byte("preview")
			return false, nil
		})
		processed <- err
	}()
	select {
	case <-started:
	case <-time.After(20 * time.Second):
		t.Fatal("worker did not acquire row")
	}
	worked, err := files.ProcessPDFThumbnailOnce(ctx, func(context.Context, store.FileRow) (bool, error) { t.Error("duplicate processing"); return false, nil })
	require.NoError(t, err)
	require.False(t, worked)
	deleted := make(chan error, 1)
	go func() { _, err := files.DeleteLegacyFile(ctx, id, nil, storage); deleted <- err }()
	select {
	case err := <-deleted:
		t.Fatalf("deletion passed active processing lock: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	close(release)
	require.NoError(t, <-processed)
	require.NoError(t, <-deleted)
	require.Empty(t, storage.keys)
}
