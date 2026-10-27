package grpcsvc

import (
	"context"
	"crypto/sha256"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"voice/backend/file/internal/r2file"
	"voice/backend/file/internal/store"
)

func TestCatalogDirectGIFCleanupCannotRaceTrackedSourcePUT(t *testing.T) {
	for _, acknowledged := range []bool{true, false} {
		name := "ack"
		if !acknowledged {
			name = "lost-ack"
		}
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
			defer cancel()
			f := seedPendingCatalog(t, ctx)
			body := []byte("GIF89a-source")
			digest := sha256.Sum256(body)
			_, err := f.pool.Exec(ctx, `UPDATE files SET created_at=clock_timestamp()-interval '2 hours' WHERE id=$1`, f.imported.FileID)
			require.NoError(t, err)
			require.NoError(t, f.files.RecordCatalogFetched(ctx, f.imported, f.capture, f.keys, int64(len(body)), digest[:]))
			require.NoError(t, f.files.StartCatalogWrite(ctx, f.imported, f.capture, f.keys))
			entered := make(chan struct{})
			release := make(chan struct{})
			var releaseOnce sync.Once
			var deletes atomic.Int32
			var mu sync.Mutex
			var stored []byte
			r2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.Method {
				case http.MethodPut:
					b, e := io.ReadAll(r.Body)
					if e != nil {
						http.Error(w, "read", 400)
						return
					}
					close(entered)
					<-release
					mu.Lock()
					stored = b
					mu.Unlock()
					if acknowledged {
						w.WriteHeader(200)
					} else {
						w.WriteHeader(503)
					}
				case http.MethodDelete:
					deletes.Add(1)
					mu.Lock()
					stored = nil
					mu.Unlock()
					w.WriteHeader(204)
				default:
					w.WriteHeader(405)
				}
			}))
			defer func() { releaseOnce.Do(func() { close(release) }); r2.Close() }()
			storage, err := r2file.NewS3R2Presigner(r2file.S3R2Config{Endpoint: r2.URL, Region: "auto", AccessKeyID: "fixture", SecretAccessKey: "fixture", Bucket: "bucket"})
			require.NoError(t, err)
			put := make(chan error, 1)
			go func() { put <- storage.PutCatalogSource(ctx, f.imported.R2Key, body) }()
			select {
			case <-entered:
			case <-ctx.Done():
				t.Fatal("source PUT did not enter owned server")
			}
			require.NoError(t, f.files.MarkDeleted(ctx, f.imported.FileID))
			cleaned, err := f.files.CleanupGIFOnce(ctx, storage)
			require.NoError(t, err)
			require.False(t, cleaned, "File age/deletion is not an I/O fence for the submitted source PUT")
			require.Zero(t, deletes.Load())
			var completed bool
			require.NoError(t, f.pool.QueryRow(ctx, `SELECT completed_at IS NOT NULL FROM file_gif_cleanup WHERE object_key=$1`, f.imported.R2Key).Scan(&completed))
			require.False(t, completed)
			releaseOnce.Do(func() { close(release) })
			putErr := <-put
			if acknowledged {
				require.NoError(t, putErr)
				require.ErrorIs(t, f.files.SettleCatalogWrite(ctx, f.imported, f.capture, f.keys, true), store.ErrCatalogActorClosed)
			} else {
				require.Error(t, putErr)
				require.NoError(t, f.files.SettleCatalogWrite(ctx, f.imported, f.capture, f.keys, false))
			}
			cleaned, err = f.files.CleanupGIFOnce(ctx, storage)
			require.NoError(t, err)
			mu.Lock()
			defer mu.Unlock()
			if acknowledged {
				require.True(t, cleaned)
				require.EqualValues(t, 1, deletes.Load())
				require.Empty(t, stored)
			} else {
				require.False(t, cleaned)
				require.Zero(t, deletes.Load())
				require.Equal(t, body, stored, "lost ACK stays explicit reconciliation work, never falsely complete")
			}
		})
	}
}
