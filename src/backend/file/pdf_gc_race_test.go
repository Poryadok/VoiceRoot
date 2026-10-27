package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	filev1 "voice.app/voice/file/v1"
	"voice/backend/file/internal/grpcsvc"
	"voice/backend/file/internal/r2file"
	"voice/backend/file/internal/store"
	"voice/backend/pkg/principal"
)

type pdfRaceObjects struct {
	mu   sync.Mutex
	keys map[string]bool
}

func (p *pdfRaceObjects) DeleteObject(_ context.Context, key string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.keys, key)
	return nil
}
func (p *pdfRaceObjects) put(key string) { p.mu.Lock(); defer p.mu.Unlock(); p.keys[key] = true }
func (p *pdfRaceObjects) count() int     { p.mu.Lock(); defer p.mu.Unlock(); return len(p.keys) }

func TestPDFWriteSurvivesSessionLossBeforeLastReferenceGC(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool := startPDFPostgres(t, ctx)
	defer pool.Close()
	files := store.NewFilesStore(pool)
	id := uuid.New()
	_, err := files.InsertPendingFile(ctx, store.FileRow{ID: id, UploaderProfileID: uuid.New(), OriginalName: "doc.pdf", MimeType: "application/pdf", SizeBytes: 8, R2Key: "original", Status: "pending_upload", FileType: "document", ScanResult: "pending"})
	require.NoError(t, err)
	_, err = files.ConfirmUpload(ctx, id, sha256Hex([]byte("original")))
	require.NoError(t, err)
	service := grpcsvc.New(grpcsvc.Deps{Files: files})
	ref := &filev1.FileReferenceKey{FileId: id.String(), OwnerType: filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_MESSAGE, OwnerId: uuid.NewString()}
	acquire := &filev1.AcquireFileReferencesRequest{ProtocolVersion: 1, OperationId: uuid.NewString(), ProducerId: filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_MESSAGING, References: []*filev1.FileReferenceKey{ref}}
	hash, err := principal.RequestHash(acquire)
	require.NoError(t, err)
	bound := principal.WithVerified(ctx, principal.Principal{Kind: "service", Subject: "service:messaging", Issuer: "messaging", Audience: "file", RPC: filev1.FileService_AcquireFileReferences_FullMethodName, RequestID: uuid.NewString(), RequestHash: hash})
	_, err = service.AcquireFileReferences(bound, acquire)
	require.NoError(t, err, "processing-file admission must remain valid")
	cfg := pool.Config().Copy()
	appName := "pdf-write-session-" + uuid.NewString()
	cfg.ConnConfig.RuntimeParams["application_name"] = appName
	workerPool, err := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)
	defer workerPool.Close()
	worker := store.NewFilesStore(workerPool)
	started, finish := make(chan struct{}), make(chan struct{})
	var finishOnce sync.Once
	unblock := func() { finishOnce.Do(func() { close(finish) }) }
	defer unblock()
	objects := &pdfRaceObjects{keys: map[string]bool{"original": true}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		_, _ = io.Copy(io.Discard, r.Body)
		close(started)
		<-finish
		objects.put(strings.TrimPrefix(r.URL.Path, "/pdf-test/"))
		w.WriteHeader(http.StatusOK)
	}))
	defer func() { unblock(); server.Close() }()
	writer, err := r2file.NewS3R2Presigner(r2file.S3R2Config{Endpoint: server.URL, Region: "auto", Bucket: "pdf-test", AccessKeyID: "test-access", SecretAccessKey: "test-secret"})
	require.NoError(t, err)
	result := make(chan error, 1)
	go func() {
		_, err := worker.ProcessPDFThumbnailOnce(ctx, func(context.Context, store.FileRow) ([]byte, bool, error) { return []byte("preview"), false, nil }, func(_ context.Context, w store.TrackedWriteIntent, _ []byte) error {
			return writer.PutTrackedObject(ctx, r2file.TrackedPut{Key: w.Key, Kind: w.Kind, ContentType: "image/png", ExpectedSHA256: w.ExpectedSHA256, ExpectedSize: w.ExpectedSize}, []byte("preview"))
		})
		result <- err
	}()
	select {
	case <-started:
	case <-ctx.Done():
		t.Fatal("processing did not reach PUT")
	}
	var killed bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE application_name=$1 AND state IN ('idle','idle in transaction')`, appName).Scan(&killed))
	require.True(t, killed)
	release := &filev1.ReleaseFileReferencesRequest{ProtocolVersion: 1, OperationId: uuid.NewString(), ProducerId: filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_MESSAGING, References: []*filev1.FileReferenceKey{ref}}
	hash, err = principal.RequestHash(release)
	require.NoError(t, err)
	bound = principal.WithVerified(ctx, principal.Principal{Kind: "service", Subject: "service:messaging", Issuer: "messaging", Audience: "file", RPC: filev1.FileService_ReleaseFileReferences_FullMethodName, RequestID: uuid.NewString(), RequestHash: hash})
	released, err := service.ReleaseFileReferences(bound, release)
	require.NoError(t, err)
	require.EqualValues(t, 1, released.Receipt.ReleasedCount)
	collected, err := files.RunReferenceGCOnce(ctx, objects, 1)
	require.NoError(t, err)
	// A detached late PUT is still live even though PostgreSQL released its lock.
	// GC must retain durable custody rather than report completion before it ends.
	require.Zero(t, collected, "GC completed while a late PDF PUT could recreate a key")
	var state string
	require.NoError(t, pool.QueryRow(ctx, `SELECT state FROM file_blobs WHERE blob_id=(SELECT blob_id FROM files WHERE id=$1)`, id).Scan(&state))
	require.NotEqual(t, "GC_COMPLETE", state)
	unblock()
	require.ErrorIs(t, <-result, store.ErrPDFPublicationLost, "the stale worker must not publish READY after last-reference release")
	collected, err = files.RunReferenceGCOnce(ctx, objects, 1)
	require.NoError(t, err)
	require.EqualValues(t, 1, collected)
	require.Zero(t, objects.count())
}

func TestPDFResumeSettledWriteWithoutRepeatingPUT(t *testing.T) {
	ctx := context.Background()
	pool := startPDFPostgres(t, ctx)
	defer pool.Close()
	files := store.NewFilesStore(pool)
	id := uuid.New()
	_, err := files.InsertPendingFile(ctx, store.FileRow{ID: id, UploaderProfileID: uuid.New(), OriginalName: "doc.pdf", MimeType: "application/pdf", SizeBytes: 8, R2Key: "original", Status: "pending_upload", FileType: "document", ScanResult: "pending"})
	require.NoError(t, err)
	_, err = files.ConfirmUpload(ctx, id, sha256Hex([]byte("original")))
	require.NoError(t, err)
	call, cancel := context.WithCancel(ctx)
	defer cancel()
	writes := 0
	worked, err := files.ProcessPDFThumbnailOnce(call, func(context.Context, store.FileRow) ([]byte, bool, error) { return []byte("preview"), false, nil }, func(context.Context, store.TrackedWriteIntent, []byte) error { writes++; cancel(); return nil })
	require.True(t, worked)
	require.Error(t, err)
	row, err := files.GetFileByID(ctx, id)
	require.NoError(t, err)
	require.Equal(t, "processing", row.Status)
	require.NotNil(t, row.ThumbnailR2Key)
	key := *row.ThumbnailR2Key
	worked, err = files.ProcessPDFThumbnailOnce(ctx, func(context.Context, store.FileRow) ([]byte, bool, error) {
		t.Error("settled output was rendered again")
		return nil, false, nil
	}, func(context.Context, store.TrackedWriteIntent, []byte) error { writes++; return nil })
	require.True(t, worked)
	require.NoError(t, err)
	require.Equal(t, 1, writes)
	row, err = files.GetFileByID(ctx, id)
	require.NoError(t, err)
	require.Equal(t, "ready", row.Status)
	require.Equal(t, key, *row.ThumbnailR2Key)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM file_catalog_writes WHERE file_id=$1`, id).Scan(&count))
	require.Equal(t, 1, count)
}

func TestPDFConcurrentSettledPublicationDoesNotQueueLiveBlobGC(t *testing.T) {
	ctx := context.Background()
	pool := startPDFPostgres(t, ctx)
	defer pool.Close()
	files := store.NewFilesStore(pool)
	id := uuid.New()
	_, err := files.InsertPendingFile(ctx, store.FileRow{ID: id, UploaderProfileID: uuid.New(), OriginalName: "doc.pdf", MimeType: "application/pdf", SizeBytes: 8, R2Key: "original", Status: "pending_upload", FileType: "document", ScanResult: "pending"})
	require.NoError(t, err)
	_, err = files.ConfirmUpload(ctx, id, sha256Hex([]byte("original")))
	require.NoError(t, err)
	acknowledged, resume := make(chan struct{}), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(resume) }) }
	defer unblock()
	result := make(chan error, 1)
	go func() {
		_, err := files.ProcessPDFThumbnailOnce(ctx, func(context.Context, store.FileRow) ([]byte, bool, error) { return []byte("preview"), false, nil }, func(_ context.Context, w store.TrackedWriteIntent, _ []byte) error {
			// Model the exact successful receipt persisted before the first owner's
			// publication. A second queue reader may observe this real SQL window.
			err := files.SettleTrackedWrite(ctx, w.Key, w.Generation, true)
			if err != nil {
				return err
			}
			close(acknowledged)
			<-resume
			return nil
		})
		result <- err
	}()
	select {
	case <-acknowledged:
	case <-time.After(20 * time.Second):
		t.Fatal("receipt not committed")
	}
	worked, err := files.ProcessPDFThumbnailOnce(ctx, func(context.Context, store.FileRow) ([]byte, bool, error) {
		t.Error("settled bytes rendered twice")
		return nil, false, nil
	}, func(context.Context, store.TrackedWriteIntent, []byte) error {
		t.Error("settled bytes uploaded twice")
		return nil
	})
	require.True(t, worked)
	require.NoError(t, err)
	unblock()
	require.NoError(t, <-result)
	row, err := files.GetFileByID(ctx, id)
	require.NoError(t, err)
	require.Equal(t, "ready", row.Status)
	var state string
	require.NoError(t, pool.QueryRow(ctx, `SELECT state FROM file_blobs WHERE blob_id=(SELECT blob_id FROM files WHERE id=$1)`, id).Scan(&state))
	require.Equal(t, "LIVE", state)
}

func TestPDFPreparationFailureDoesNotStarveNextFile(t *testing.T) {
	ctx := context.Background()
	pool := startPDFPostgres(t, ctx)
	defer pool.Close()
	files := store.NewFilesStore(pool)
	ids := []uuid.UUID{uuid.New(), uuid.New()}
	for i, id := range ids {
		_, err := files.InsertPendingFile(ctx, store.FileRow{ID: id, UploaderProfileID: uuid.New(), OriginalName: "doc.pdf", MimeType: "application/pdf", SizeBytes: 8, R2Key: id.String(), Status: "pending_upload", FileType: "document", ScanResult: "pending"})
		require.NoError(t, err)
		_, err = files.ConfirmUpload(ctx, id, sha256Hex([]byte("original")))
		require.NoError(t, err)
		_, err = pool.Exec(ctx, `UPDATE files SET updated_at=timestamp '2020-01-01'+$2*interval '1 day' WHERE id=$1`, id, i)
		require.NoError(t, err)
	}
	var writes []uuid.UUID
	prepare := func(_ context.Context, row store.FileRow) ([]byte, bool, error) {
		if row.ID == ids[0] {
			return nil, false, errors.New("temporary read failure")
		}
		return []byte("preview"), false, nil
	}
	put := func(_ context.Context, w store.TrackedWriteIntent, _ []byte) error {
		writes = append(writes, w.FileID)
		return nil
	}
	worked, err := files.ProcessPDFThumbnailOnce(ctx, prepare, put)
	require.True(t, worked)
	require.Error(t, err)
	worked, err = files.ProcessPDFThumbnailOnce(ctx, prepare, put)
	require.True(t, worked)
	require.NoError(t, err)
	require.Equal(t, []uuid.UUID{ids[1]}, writes)
}
