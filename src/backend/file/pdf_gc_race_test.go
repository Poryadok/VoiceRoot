package main

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	filev1 "voice.app/voice/file/v1"
	"voice/backend/file/internal/grpcsvc"
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
	pool := startFilePostgres(t, ctx)
	defer pool.Close()
	files := store.NewFilesStore(pool)
	id := uuid.New()
	_, err := files.InsertPendingFile(ctx, store.FileRow{ID: id, UploaderProfileID: uuid.New(), OriginalName: "doc.pdf", MimeType: "application/pdf", SizeBytes: 8, R2Key: "original", Status: "pending_upload", FileType: "document", ScanResult: "pending"})
	require.NoError(t, err)
	_, err = files.ConfirmUpload(ctx, id, "hash")
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
	result := make(chan error, 1)
	go func() {
		_, err := worker.ProcessPDFThumbnailOnce(ctx, func(_ context.Context, row store.FileRow) (bool, error) {
			close(started)
			<-finish
			objects.put(*row.ThumbnailR2Key)
			return false, nil
		})
		result <- err
	}()
	select {
	case <-started:
	case <-ctx.Done():
		t.Fatal("processing did not reach PUT")
	}
	var killed bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE application_name=$1 AND state='idle in transaction'`, appName).Scan(&killed))
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
	require.Equal(t, "GC_PENDING", state)
	unblock()
	require.Error(t, <-result, "the killed metadata transaction cannot commit")
	collected, err = files.RunReferenceGCOnce(ctx, objects, 1)
	require.NoError(t, err)
	require.EqualValues(t, 1, collected)
	require.Zero(t, objects.count())
}
