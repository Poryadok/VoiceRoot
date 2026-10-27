package grpcsvc

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	filev1 "voice.app/voice/file/v1"
	"voice/backend/file/internal/gifproc"
	"voice/backend/file/internal/jobs"
	"voice/backend/file/internal/r2file"
	"voice/backend/file/internal/store"
)

func TestTrackedGIFLastReferenceReleaseCannotPublishAfterLateACK(t *testing.T) {
	t.Run("publication", func(t *testing.T) { testTrackedGIFReferenceRetirement(t, false) })
	t.Run("restarted-consumer", func(t *testing.T) { testTrackedGIFReferenceRetirement(t, true) })
}

func testTrackedGIFReferenceRetirement(t *testing.T, restart bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pool := startR23FilePostgres(t, ctx)
	for _, migration := range []string{"000010_catalog_asset_import.up.sql", "000011_tracked_write_owner_kind.up.sql"} {
		applyFileGateSQL(t, ctx, pool, filepath.Join("src", "backend", "migrations", "file_db", migration))
	}
	files := store.NewFilesStore(pool)
	id := uuid.New()
	raw := []byte("GIF89a-source")
	_, err := files.InsertPendingFile(ctx, store.FileRow{ID: id, UploaderProfileID: uuid.New(), OriginalName: "ordinary.gif", MimeType: "image/gif", SizeBytes: int64(len(raw)), R2Key: "ordinary/" + id.String() + "/source.gif", Status: "pending_upload", FileType: "image", UploadIntent: "gif", ScanResult: "pending"})
	require.NoError(t, err)
	row, err := files.ConfirmGIFUpload(ctx, id, shaHex(raw))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE files SET created_at=clock_timestamp()-interval '2 hours' WHERE id=$1`, id)
	require.NoError(t, err)
	owner := &FileGRPC{files: files}
	ref := &filev1.FileReferenceKey{FileId: id.String(), OwnerType: filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_MESSAGE, OwnerId: uuid.NewString()}
	acquire := &filev1.AcquireFileReferencesRequest{ProtocolVersion: 1, OperationId: uuid.NewString(), ProducerId: filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_MESSAGING, References: []*filev1.FileReferenceKey{ref}}
	_, err = owner.AcquireFileReferences(catalogVerifiedContext(t, acquire, filev1.FileService_AcquireFileReferences_FullMethodName, "messaging"), acquire)
	require.NoError(t, err)
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	var mu sync.Mutex
	objects := map[string][]byte{"/bucket/" + row.R2Key: raw}
	r2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			mu.Lock()
			b := append([]byte(nil), objects[r.URL.Path]...)
			mu.Unlock()
			if b == nil {
				http.NotFound(w, r)
				return
			}
			w.Write(b)
		case http.MethodPut:
			b, _ := io.ReadAll(r.Body)
			if strings.HasSuffix(r.URL.Path, "full.mp4") {
				close(entered)
				<-release
			}
			mu.Lock()
			objects[r.URL.Path] = b
			mu.Unlock()
			w.WriteHeader(200)
		case http.MethodDelete:
			mu.Lock()
			delete(objects, r.URL.Path)
			mu.Unlock()
			w.WriteHeader(204)
		default:
			w.WriteHeader(405)
		}
	}))
	defer func() { once.Do(func() { close(release) }); r2.Close() }()
	storage, err := r2file.NewS3R2Presigner(r2file.S3R2Config{Endpoint: r2.URL, Region: "auto", AccessKeyID: "fixture", SecretAccessKey: "fixture", Bucket: "bucket"})
	require.NoError(t, err)
	done := make(chan error, 1)
	go func() {
		_, e := jobs.RunGIFProcessingOnce(ctx, files, storage, func(context.Context, []byte) (gifproc.Output, error) {
			return gifproc.Output{Full: []byte("measured-mp4-output"), Thumbnail: []byte("measured-webp-output"), Width: 16, Height: 16, DurationSeconds: 1}, nil
		})
		done <- e
	}()
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal("derived PUT did not enter owned server")
	}
	drop := &filev1.ReleaseFileReferencesRequest{ProtocolVersion: 1, OperationId: uuid.NewString(), ProducerId: filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_MESSAGING, References: []*filev1.FileReferenceKey{ref}}
	_, err = owner.ReleaseFileReferences(catalogVerifiedContext(t, drop, filev1.FileService_ReleaseFileReferences_FullMethodName, "messaging"), drop)
	require.NoError(t, err)
	var state string
	require.NoError(t, pool.QueryRow(ctx, `SELECT state FROM file_blobs WHERE blob_id=$1`, id).Scan(&state))
	require.Equal(t, "LIVE", state, "write pin blocks physical GC, not logical reference retirement")
	if restart {
		worked, e := jobs.RunGIFProcessingOnce(ctx, files, storage, nil)
		require.NoError(t, e)
		require.True(t, worked, "second executor retires authority without another PUT")
		require.NoError(t, pool.QueryRow(ctx, `SELECT state FROM file_blobs WHERE blob_id=$1`, id).Scan(&state))
		require.Equal(t, "LIVE", state, "logical retirement never clears the in-flight pin")
	}
	once.Do(func() { close(release) })
	require.Error(t, <-done, "last-reference retirement must win publication despite a successful PUT ACK")
	if restart {
		worked, e := jobs.RunGIFProcessingOnce(ctx, files, storage, nil)
		require.NoError(t, e)
		require.True(t, worked, "known physical ACK restores the retired GC work")
	}
	require.NoError(t, pool.QueryRow(ctx, `SELECT status FROM files WHERE id=$1`, id).Scan(&state))
	require.NotEqual(t, "ready", state)
	require.NoError(t, pool.QueryRow(ctx, `SELECT state FROM file_blobs WHERE blob_id=$1`, id).Scan(&state))
	require.Equal(t, "GC_PENDING", state, "settled retirement must restore durable GC work")
	completed, err := files.RunReferenceGCOnce(ctx, storage, 1)
	require.NoError(t, err)
	require.EqualValues(t, 1, completed)
	_, err = pool.Exec(ctx, `UPDATE file_gif_cleanup SET not_before=clock_timestamp()-interval '1 second'`)
	require.NoError(t, err)
	for range 4 {
		cleaned, e := files.CleanupGIFOnce(ctx, storage)
		require.NoError(t, e)
		if !cleaned {
			break
		}
	}
	mu.Lock()
	defer mu.Unlock()
	require.Empty(t, objects)
}

func TestTrackedGIFReconciliationRechecksReferenceThatWonBlobLock(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pool := startR23FilePostgres(t, ctx)
	for _, m := range []string{"000010_catalog_asset_import.up.sql", "000011_tracked_write_owner_kind.up.sql"} {
		applyFileGateSQL(t, ctx, pool, filepath.Join("src", "backend", "migrations", "file_db", m))
	}
	files := store.NewFilesStore(pool)
	id := uuid.New()
	raw := []byte("GIF89a-source")
	_, err := files.InsertPendingFile(ctx, store.FileRow{ID: id, UploaderProfileID: uuid.New(), OriginalName: "ordinary.gif", MimeType: "image/gif", SizeBytes: int64(len(raw)), R2Key: id.String(), Status: "pending_upload", FileType: "image", UploadIntent: "gif", ScanResult: "pending"})
	require.NoError(t, err)
	_, err = files.ConfirmGIFUpload(ctx, id, shaHex(raw))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO file_references(file_id,owner_type,owner_id,released_at) VALUES($1,1,$2,clock_timestamp())`, id, uuid.New())
	require.NoError(t, err)
	blocker, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = blocker.Rollback(context.Background()) }()
	_, err = blocker.Exec(ctx, `LOCK TABLE file_reference_operations IN SHARE MODE`)
	require.NoError(t, err)
	owner := &FileGRPC{files: files}
	r := &filev1.AcquireFileReferencesRequest{ProtocolVersion: 1, OperationId: uuid.NewString(), ProducerId: filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_MESSAGING, References: []*filev1.FileReferenceKey{{FileId: id.String(), OwnerType: filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_MESSAGE, OwnerId: uuid.NewString()}}}
	verified := catalogVerifiedContext(t, r, filev1.FileService_AcquireFileReferences_FullMethodName, "messaging")
	acquired := make(chan error, 1)
	go func() { _, e := owner.AcquireFileReferences(verified, r); acquired <- e }()
	r23WaitForBlockedTransactions(t, ctx, pool, 1) // Acquire owns blob, waits to save receipt.
	type outcome struct {
		retired bool
		err     error
	}
	reconciled := make(chan outcome, 1)
	go func() { b, e := files.ReconcileRetiredGIFWritesOnce(ctx); reconciled <- outcome{b, e} }()
	r23WaitForBlockedTransactions(t, ctx, pool, 2)
	require.NoError(t, blocker.Commit(ctx))
	require.NoError(t, <-acquired)
	out := <-reconciled
	require.NoError(t, out.err)
	require.False(t, out.retired, "a reference committed before reconciliation obtains the blob must win")
	var state string
	require.NoError(t, pool.QueryRow(ctx, `SELECT status FROM files WHERE id=$1`, id).Scan(&state))
	require.Equal(t, "processing", state)
}
