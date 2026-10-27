package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"
	filev1 "voice.app/voice/file/v1"
	"voice/backend/file/internal/jobs"
	"voice/backend/file/internal/pdfproc"
	"voice/backend/file/internal/r2file"
	"voice/backend/file/internal/store"
)

func startPDFPostgres(t *testing.T, ctx context.Context) *pgxpool.Pool {
	pool := startFilePostgres(t, ctx)
	for _, name := range []string{"000009_gif_processing", "000010_catalog_asset_import", "000011_tracked_write_owner_kind"} {
		applyFileSQL(t, ctx, pool, filepath.Join("src", "backend", "migrations", "file_db", name+".up.sql"))
	}
	return pool
}

type pdfStorage struct {
	keys           map[string][]byte
	fail, readFail bool
	writes         int
}

func (p *pdfStorage) ReadObject(_ context.Context, k string, _ int64) ([]byte, error) {
	if p.readFail {
		return nil, errors.New("temporary read failure")
	}
	return p.keys[k], nil
}
func (p *pdfStorage) PutTrackedObject(_ context.Context, w r2file.TrackedPut, b []byte) error {
	p.keys[w.Key] = b
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
	pool := startPDFPostgres(t, ctx)
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
	storage := &pdfStorage{keys: map[string][]byte{"original.pdf": defaultUploadBytes}, readFail: true}
	renderer := &pdfRenderer{}
	worked, err := jobs.RunPDFThumbnailOnce(ctx, files, storage, renderer)
	require.True(t, worked)
	require.Error(t, err)
	row, err := files.GetFileByID(ctx, id)
	require.NoError(t, err)
	require.Equal(t, "processing", row.Status)
	require.Nil(t, row.ThumbnailR2Key)
	require.Zero(t, storage.writes)
	storage.readFail = false
	worked, err = jobs.RunPDFThumbnailOnce(ctx, files, storage, renderer)
	require.True(t, worked)
	require.NoError(t, err)
	row, err = files.GetFileByID(ctx, id)
	require.NoError(t, err)
	require.Equal(t, "ready", row.Status)
	require.NotNil(t, row.ThumbnailR2Key)
	key := *row.ThumbnailR2Key
	require.Contains(t, storage.keys, key)
	require.Nil(t, row.ConvertedR2Key)
	require.Equal(t, "original.pdf", row.R2Key)
	require.Equal(t, int64(len(defaultUploadBytes)), row.SizeBytes)
	require.Equal(t, expiry.Unix(), row.ExpiresAt.Unix())
	require.Equal(t, defaultUploadHash, *row.SHA256Hash)
	var blobKey, state string
	require.NoError(t, pool.QueryRow(ctx, `SELECT thumbnail_r2_key FROM file_blobs WHERE blob_id=(SELECT blob_id FROM files WHERE id=$1)`, id).Scan(&blobKey))
	require.Equal(t, key, blobKey)
	require.NoError(t, pool.QueryRow(ctx, `SELECT state FROM file_catalog_writes WHERE object_key=$1 AND owner_kind='pdf_thumbnail'`, key).Scan(&state))
	require.Equal(t, "SETTLED", state)
	_, err = pool.Exec(ctx, `INSERT INTO file_catalog_writes(object_key,file_id,generation,owner_kind,state,expected_sha256,expected_size_bytes) VALUES('invalid-kind',$1,$2,'other','DECLARED',decode($3,'hex'),1)`, id, uuid.New(), defaultUploadHash)
	var constraint *pgconn.PgError
	require.ErrorAs(t, err, &constraint)
	require.Equal(t, "23514", constraint.Code)
	require.Equal(t, "file_catalog_writes_owner_kind_check", constraint.ConstraintName)
	down, err := os.ReadFile(filepath.Join(fileRepoRoot(t), "src", "backend", "migrations", "file_db", "000011_tracked_write_owner_kind.down.sql"))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(down))
	require.ErrorContains(t, err, "cannot erase retained derived-object write custody")
	worked, err = jobs.RunPDFThumbnailOnce(ctx, files, storage, renderer)
	require.False(t, worked)
	require.NoError(t, err)
	require.Equal(t, 1, renderer.calls)
	require.Equal(t, 1, storage.writes)
	var events int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM file_preview_event_outbox WHERE file_id=$1`, id).Scan(&events))
	require.Equal(t, 2, events)
	_, err = pool.Exec(ctx, `UPDATE files SET expires_at=now()-interval '1 day' WHERE id=$1`, id)
	require.NoError(t, err)
	_, err = jobs.RunExpiryPurgeOnce(ctx, files, storage, nil, time.Now())
	require.NoError(t, err)
	require.Empty(t, storage.keys)
}

func TestPDFWorkerMalformedE2EAndLostACKPinsCleanup(t *testing.T) {
	ctx := context.Background()
	pool := startPDFPostgres(t, ctx)
	defer pool.Close()
	files := store.NewFilesStore(pool)
	insert := func(e2e bool) uuid.UUID {
		id := uuid.New()
		_, err := files.InsertPendingFile(ctx, store.FileRow{ID: id, UploaderProfileID: uuid.New(), OriginalName: "doc.pdf", MimeType: "application/pdf", SizeBytes: 8, R2Key: id.String(), Status: "pending_upload", FileType: "document", ScanResult: "pending", IsE2E: e2e})
		require.NoError(t, err)
		_, err = files.ConfirmUpload(ctx, id, sha256Hex([]byte("original")))
		require.NoError(t, err)
		return id
	}
	encrypted, id := insert(true), insert(false)
	storage := &pdfStorage{keys: map[string][]byte{id.String(): []byte("original")}}
	renderer := &pdfRenderer{err: pdfproc.ErrInvalidPDF}
	worked, err := jobs.RunPDFThumbnailOnce(ctx, files, storage, renderer)
	require.True(t, worked)
	require.NoError(t, err)
	row, err := files.GetFileByID(ctx, id)
	require.NoError(t, err)
	require.Equal(t, "failed", row.Status)
	require.Zero(t, storage.writes)
	var failedBlob string
	require.NoError(t, pool.QueryRow(ctx, `SELECT state FROM file_blobs WHERE blob_id=(SELECT blob_id FROM files WHERE id=$1)`, id).Scan(&failedBlob))
	require.Equal(t, "GC_PENDING", failedBlob, "known no-write failure must not strand the original outside ready-only expiry")
	row, err = files.GetFileByID(ctx, encrypted)
	require.NoError(t, err)
	require.Nil(t, row.ThumbnailR2Key)
	tampered := insert(false)
	storage.keys[tampered.String()] = []byte("tampered")
	renderer.err = nil
	beforeRender := renderer.calls
	worked, err = jobs.RunPDFThumbnailOnce(ctx, files, storage, renderer)
	require.True(t, worked)
	require.NoError(t, err)
	row, err = files.GetFileByID(ctx, tampered)
	require.NoError(t, err)
	require.Equal(t, "failed", row.Status, "async processing must use the confirmed source digest")
	require.Equal(t, beforeRender, renderer.calls, "changed source bytes must not reach the renderer")
	orphan := insert(false)
	storage.keys[orphan.String()] = []byte("original")
	renderer.err = nil
	storage.fail = true
	worked, err = jobs.RunPDFThumbnailOnce(ctx, files, storage, renderer)
	require.True(t, worked)
	require.Error(t, err)
	row, err = files.GetFileByID(ctx, orphan)
	require.NoError(t, err)
	require.NotNil(t, row.ThumbnailR2Key)
	var state string
	require.NoError(t, pool.QueryRow(ctx, `SELECT state FROM file_catalog_writes WHERE object_key=$1`, *row.ThumbnailR2Key).Scan(&state))
	require.Equal(t, "UNSETTLED", state)
	completed, err := files.DeleteLegacyFile(ctx, orphan, nil, storage)
	require.Error(t, err)
	require.False(t, completed, "a missing ACK cannot be converted to deletion-complete")
	// HEAD-not-found/absence and a worker restart do not fence a late PUT.
	delete(storage.keys, *row.ThumbnailR2Key)
	storage.fail = false
	worked, err = jobs.RunPDFThumbnailOnce(ctx, store.NewFilesStore(pool), storage, &pdfRenderer{})
	require.False(t, worked)
	require.NoError(t, err)
	require.Equal(t, 1, storage.writes)
	collected, err := files.RunReferenceGCOnce(ctx, storage, 100)
	require.NoError(t, err)
	require.EqualValues(t, 2, collected, "known malformed/tampered originals collect; ambiguous PUT stays pinned")
	require.NoError(t, pool.QueryRow(ctx, `SELECT state FROM file_catalog_writes WHERE object_key=$1`, *row.ThumbnailR2Key).Scan(&state))
	require.Equal(t, "UNSETTLED", state)
}

func TestPDFWorkerSerializesDeleteAndSkipsConcurrentClaim(t *testing.T) {
	ctx := context.Background()
	pool := startPDFPostgres(t, ctx)
	defer pool.Close()
	files := store.NewFilesStore(pool)
	id := uuid.New()
	_, err := files.InsertPendingFile(ctx, store.FileRow{ID: id, UploaderProfileID: uuid.New(), OriginalName: "doc.pdf", MimeType: "application/pdf", SizeBytes: 8, R2Key: "original", Status: "pending_upload", FileType: "document", ScanResult: "pending"})
	require.NoError(t, err)
	_, err = files.ConfirmUpload(ctx, id, sha256Hex([]byte("original")))
	require.NoError(t, err)
	started, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	defer unblock()
	processed := make(chan error, 1)
	objects := &pdfRaceObjects{keys: map[string]bool{"original": true}}
	prepare := func(context.Context, store.FileRow) ([]byte, bool, error) { return []byte("preview"), false, nil }
	call, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		_, err := files.ProcessPDFThumbnailOnce(call, prepare, func(_ context.Context, w store.TrackedWriteIntent, _ []byte) error {
			close(started)
			<-release
			objects.put(w.Key)
			cancel()
			return nil
		})
		processed <- err
	}()
	select {
	case <-started:
	case <-time.After(20 * time.Second):
		t.Fatal("worker did not reach PUT")
	}
	worked, err := files.ProcessPDFThumbnailOnce(ctx, prepare, func(context.Context, store.TrackedWriteIntent, []byte) error { t.Error("duplicate PUT"); return nil })
	require.NoError(t, err)
	require.False(t, worked)
	completed, err := files.DeleteLegacyFile(ctx, id, nil, objects)
	require.Error(t, err)
	require.False(t, completed)
	unblock()
	require.Error(t, <-processed)
	row, err := files.GetFileByID(ctx, id)
	require.NoError(t, err)
	require.Equal(t, "deleted", row.Status)
	// The exact ACK was persisted despite cancellation. A new worker must queue
	// physical GC for the retired File, rather than strand a SETTLED object LIVE.
	worked, err = files.ProcessPDFThumbnailOnce(ctx, func(context.Context, store.FileRow) ([]byte, bool, error) {
		t.Error("retired settled output was rendered again")
		return nil, false, nil
	}, func(context.Context, store.TrackedWriteIntent, []byte) error {
		t.Error("retired settled output was uploaded again")
		return nil
	})
	require.True(t, worked)
	require.ErrorIs(t, err, store.ErrPDFPublicationLost)
	collected, err := files.RunReferenceGCOnce(ctx, objects, 1)
	require.NoError(t, err)
	require.EqualValues(t, 1, collected)
	require.Zero(t, objects.count())
}
