package grpcsvc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"voice/backend/file/internal/gifproc"
	"voice/backend/file/internal/jobs"
	"voice/backend/file/internal/r2file"
	"voice/backend/file/internal/store"
)

func TestCatalogDerivedGIFWritePinSurvivesDeletionAndLatePUT(t *testing.T) {
	for _, ack := range []bool{true, false} {
		name := "ack"
		if !ack {
			name = "lost-ack"
		}
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
			defer cancel()
			f := seedPendingCatalog(t, ctx)
			raw := []byte("GIF89a-source")
			digest := sha256.Sum256(raw)
			// Source admission is isolated here. The native integration separately
			// proves actual source bytes, conversion and durable reference commit.
			require.NoError(t, f.files.RecordCatalogFetched(ctx, f.imported, f.capture, f.keys, int64(len(raw)), digest[:]))
			require.NoError(t, f.files.StartCatalogWrite(ctx, f.imported, f.capture, f.keys))
			require.NoError(t, f.files.SettleCatalogWrite(ctx, f.imported, f.capture, f.keys, true))
			entered, release := make(chan struct{}), make(chan struct{})
			var once sync.Once
			var puts, deletes atomic.Int32
			var mu sync.Mutex
			stored := map[string][]byte{}
			r2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.Method {
				case http.MethodGet:
					w.Write(raw)
				case http.MethodPut:
					b, e := io.ReadAll(r.Body)
					if e != nil {
						http.Error(w, "read", 400)
						return
					}
					puts.Add(1)
					if strings.HasSuffix(r.URL.Path, "full.mp4") {
						close(entered)
						<-release
					}
					mu.Lock()
					stored[r.URL.Path] = b
					mu.Unlock()
					if ack {
						w.WriteHeader(200)
					} else {
						w.WriteHeader(503)
					}
				case http.MethodDelete:
					deletes.Add(1)
					mu.Lock()
					delete(stored, r.URL.Path)
					mu.Unlock()
					w.WriteHeader(204)
				default:
					w.WriteHeader(405)
				}
			}))
			defer func() { once.Do(func() { close(release) }); r2.Close() }()
			objects, err := r2file.NewS3R2Presigner(r2file.S3R2Config{Endpoint: r2.URL, Region: "auto", AccessKeyID: "fixture", SecretAccessKey: "fixture", Bucket: "bucket"})
			require.NoError(t, err)
			done := make(chan error, 1)
			go func() {
				_, e := jobs.RunGIFProcessingOnce(ctx, f.files, objects, func(context.Context, []byte) (gifproc.Output, error) {
					return gifproc.Output{Full: []byte("measured-mp4-output"), Thumbnail: []byte("measured-webp-output"), Width: 16, Height: 16, DurationSeconds: 1}, nil
				})
				done <- e
			}()
			select {
			case <-entered:
			case <-ctx.Done():
				t.Fatal("derived PUT did not enter owned HTTP server")
			}
			var state string
			require.NoError(t, f.pool.QueryRow(ctx, `SELECT state FROM file_catalog_writes WHERE file_id=$1 AND owner_kind='gif_full'`, f.imported.FileID).Scan(&state))
			require.Equal(t, "STARTED", state)
			require.NoError(t, f.files.MarkDeleted(ctx, f.imported.FileID))
			_, err = f.pool.Exec(ctx, `UPDATE file_gif_jobs SET lease_until=clock_timestamp()-interval '1 second'; UPDATE file_gif_cleanup SET not_before=clock_timestamp()-interval '1 second',recheck_until=clock_timestamp()-interval '1 second'`)
			require.NoError(t, err)
			cleaned, err := f.files.CleanupGIFOnce(ctx, objects)
			require.NoError(t, err)
			require.False(t, cleaned, "expired processing lease cannot fence a submitted derived PUT")
			require.Zero(t, deletes.Load())
			once.Do(func() { close(release) })
			require.Error(t, <-done, "deleted File cannot publish or start its thumbnail")
			require.EqualValues(t, 1, puts.Load(), "one SDK attempt, no thumbnail after authority loss")
			require.NoError(t, f.pool.QueryRow(ctx, `SELECT state FROM file_catalog_writes WHERE file_id=$1 AND owner_kind='gif_full'`, f.imported.FileID).Scan(&state))
			if ack {
				require.Equal(t, "SETTLED", state)
			} else {
				require.Equal(t, "UNSETTLED", state)
			}
			require.NoError(t, f.pool.QueryRow(ctx, `SELECT status FROM files WHERE id=$1`, f.imported.FileID).Scan(&state))
			require.NotEqual(t, "ready", state)
			for range 4 {
				cleaned, err = f.files.CleanupGIFOnce(ctx, objects)
				require.NoError(t, err)
				if !cleaned {
					break
				}
			}
			mu.Lock()
			defer mu.Unlock()
			if ack {
				require.Empty(t, stored)
				require.Positive(t, deletes.Load())
			} else {
				require.Len(t, stored, 1)
				require.Zero(t, deletes.Load())
			}
		})
	}
}

func TestCatalogGIFFinishCannotBypassUnsettledOutputAuthority(t *testing.T) {
	ctx := context.Background()
	f := seedPendingCatalog(t, ctx)
	raw := []byte("GIF89a-source")
	digest := sha256.Sum256(raw)
	require.NoError(t, f.files.RecordCatalogFetched(ctx, f.imported, f.capture, f.keys, int64(len(raw)), digest[:]))
	require.NoError(t, f.files.StartCatalogWrite(ctx, f.imported, f.capture, f.keys))
	require.NoError(t, f.files.SettleCatalogWrite(ctx, f.imported, f.capture, f.keys, true))
	job, err := f.files.ClaimGIFJob(ctx)
	require.NoError(t, err)
	require.True(t, job.TrackedWrites)
	full := &store.Representation{Key: job.FullKey, MIMEType: "video/mp4", Filename: "full.mp4", SizeBytes: 13, SHA256: hex.EncodeToString(digest[:]), ProcessorVersion: store.GIFProcessorVersion}
	thumb := &store.Representation{Key: job.ThumbnailKey, MIMEType: "image/webp", Filename: "thumb.webp", SizeBytes: 13, SHA256: hex.EncodeToString(digest[:]), ProcessorVersion: store.GIFProcessorVersion}
	require.NoError(t, f.files.PrepareGIFTrackedWrites(ctx, job, full, thumb))
	job.TrackedWrites = false // The input struct cannot waive the durable mode.
	finished, err := f.files.FinishGIFJob(ctx, job, full, thumb, 16, 16, 1, false)
	require.Error(t, err)
	require.False(t, finished)
	var count int
	require.NoError(t, f.pool.QueryRow(ctx, `SELECT count(*) FROM file_gif_outputs WHERE file_id=$1`, f.imported.FileID).Scan(&count))
	require.Zero(t, count)
}

func TestCatalogSourceCannotUseOrdinaryGIFConfirmation(t *testing.T) {
	ctx := context.Background()
	f := seedPendingCatalog(t, ctx)
	raw := []byte("GIF89a-source")
	digest := sha256.Sum256(raw)
	require.NoError(t, f.files.RecordCatalogFetched(ctx, f.imported, f.capture, f.keys, int64(len(raw)), digest[:]))
	_, err := f.files.ConfirmGIFUpload(ctx, f.imported.FileID, hex.EncodeToString(digest[:]))
	require.ErrorIs(t, err, store.ErrGIFConfirmationConflict, "ordinary confirmation cannot promote a protected source import")
	var state string
	require.NoError(t, f.pool.QueryRow(ctx, `SELECT status FROM files WHERE id=$1`, f.imported.FileID).Scan(&state))
	require.Equal(t, "pending_upload", state)
}
