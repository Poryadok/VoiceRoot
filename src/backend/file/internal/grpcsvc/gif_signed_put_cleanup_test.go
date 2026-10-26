package grpcsvc

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/file/internal/r2file"
	"voice/backend/file/internal/store"
)

func TestGIFSignedPUTLateCompletionRemainsDiscoverable(t *testing.T) {
	for _, legacyCompleted := range []bool{false, true} {
		name := "pending-intent"
		if legacyCompleted {
			name = "legacy-clock-completed"
		}
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
			defer cancel()
			pool := startFileGatePostgres(t, ctx)
			files := store.NewFilesStore(pool)
			id := uuid.New()
			key := "signed-gif/" + id.String()
			body := []byte("GIF89a-accepted-source")
			_, err := files.InsertPendingFile(ctx, store.FileRow{ID: id, UploaderProfileID: uuid.New(), OriginalName: "source.gif", MimeType: "image/gif", SizeBytes: int64(len(body)), R2Key: key, Status: "pending_upload", FileType: "image", UploadIntent: "gif", ScanResult: "pending"})
			require.NoError(t, err)
			entered, release := make(chan struct{}), make(chan struct{})
			var once sync.Once
			var mu sync.Mutex
			var object []byte
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.Method {
				case http.MethodPut:
					content, readErr := io.ReadAll(r.Body)
					if readErr != nil {
						w.WriteHeader(http.StatusBadRequest)
						return
					}
					close(entered)
					<-release
					mu.Lock()
					object = content
					mu.Unlock()
					w.WriteHeader(http.StatusOK)
				case http.MethodDelete:
					mu.Lock()
					object = nil
					mu.Unlock()
					w.WriteHeader(http.StatusNoContent)
				default:
					w.WriteHeader(http.StatusMethodNotAllowed)
				}
			}))
			defer func() { once.Do(func() { close(release) }); server.Close() }()
			storage, err := r2file.NewS3R2Presigner(r2file.S3R2Config{Endpoint: server.URL, Region: "auto", AccessKeyID: "fixture", SecretAccessKey: "fixture", Bucket: "bucket"})
			require.NoError(t, err)
			url, err := storage.PresignPut(ctx, r2file.PutPresignInput{Key: key, ContentType: "image/gif", ContentLength: int64(len(body))})
			require.NoError(t, err)
			request, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(body))
			require.NoError(t, err)
			request.Header.Set("Content-Type", "image/gif")
			put := make(chan error, 1)
			go func() {
				response, putErr := server.Client().Do(request)
				if putErr == nil {
					_, putErr = io.Copy(io.Discard, response.Body)
					_ = response.Body.Close()
				}
				put <- putErr
			}()
			select {
			case <-entered:
			case <-ctx.Done():
				t.Fatal("signed source PUT did not enter storage")
			}
			require.NoError(t, files.MarkDeleted(ctx, id))
			// Advance only the old database expiry barrier, without claiming that
			// expiration cancels the already accepted HTTP request held above.
			_, err = pool.Exec(ctx, `UPDATE file_gif_cleanup SET recheck_until=clock_timestamp()-interval '1 second',not_before=clock_timestamp()-interval '1 second' WHERE object_key=$1`, key)
			require.NoError(t, err)
			cleaned, err := files.CleanupGIFOnce(ctx, storage)
			require.NoError(t, err)
			require.True(t, cleaned)
			mu.Lock()
			stored := bytes.Clone(object)
			mu.Unlock()
			require.Empty(t, stored, "first DELETE observes no committed object")
			once.Do(func() { close(release) })
			require.NoError(t, <-put)
			mu.Lock()
			stored = bytes.Clone(object)
			mu.Unlock()
			require.Equal(t, body, stored, "accepted PUT commits after successful DELETE")
			if legacyCompleted {
				_, err = pool.Exec(ctx, `UPDATE file_gif_cleanup SET completed_at=clock_timestamp() WHERE object_key=$1`, key)
				require.NoError(t, err)
			}
			_, err = pool.Exec(ctx, `UPDATE file_gif_cleanup SET not_before=clock_timestamp()-interval '1 second' WHERE object_key=$1`, key)
			require.NoError(t, err)
			// A new consumer must discover the intent, including old clock-only
			// completion records; it has no trusted acknowledgement from this client.
			restarted := store.NewFilesStore(pool)
			cleaned, err = restarted.CleanupGIFOnce(ctx, storage)
			require.NoError(t, err)
			require.True(t, cleaned, "late PUT must not become an undiscoverable orphan after URL age")
			mu.Lock()
			stored = bytes.Clone(object)
			mu.Unlock()
			require.Empty(t, stored)
			var pending bool
			require.NoError(t, pool.QueryRow(ctx, `SELECT completed_at IS NULL FROM file_gif_cleanup WHERE object_key=$1`, key).Scan(&pending))
			require.True(t, pending, "client acknowledgement and URL age do not prove remote write quiescence")
		})
	}
}
