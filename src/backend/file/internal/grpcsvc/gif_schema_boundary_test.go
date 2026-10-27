package grpcsvc

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"voice/backend/file/internal/gifproc"
	"voice/backend/file/internal/jobs"
	"voice/backend/file/internal/store"
)

type gifSchemaObjects struct {
	*gifWorkerObjects
	ordinaryPuts int
}

func (o *gifSchemaObjects) PutObject(ctx context.Context, key, mime string, body []byte) error {
	o.ordinaryPuts++
	return o.gifWorkerObjects.PutObject(ctx, key, mime, body)
}

// The partial schema must reject before either derivative PUT. A genuinely
// ledger-free ordinary deployment keeps its existing worker behavior.
func TestGIFWorkerRejectsPartialTrackedSchema(t *testing.T) {
	for _, partial := range []bool{false, true} {
		name := "no-ledger-legacy"
		if partial {
			name = "ledger10-without-owner-kind"
		}
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			pool := startR23FilePostgres(t, ctx)
			if partial {
				applyFileGateSQL(t, ctx, pool, filepath.Join("src", "backend", "migrations", "file_db", "000010_catalog_asset_import.up.sql"))
			}
			files := store.NewFilesStore(pool)
			id := uuid.New()
			raw := []byte("ordinary source verified before injected codec")
			_, err := files.InsertPendingFile(ctx, store.FileRow{ID: id, UploaderProfileID: uuid.New(), OriginalName: "ordinary.gif", MimeType: "image/gif", SizeBytes: int64(len(raw)), R2Key: "ordinary/" + id.String(), Status: "pending_upload", FileType: "image", UploadIntent: "gif", ScanResult: "pending"})
			require.NoError(t, err)
			row, err := files.ConfirmGIFUpload(ctx, id, shaHex(raw))
			require.NoError(t, err)
			objects := &gifSchemaObjects{gifWorkerObjects: &gifWorkerObjects{data: map[string][]byte{row.R2Key: raw}}}
			worked, runErr := jobs.RunGIFProcessingOnce(ctx, files, objects, func(context.Context, []byte) (gifproc.Output, error) {
				return gifproc.Output{Full: []byte("measured video fixture"), Thumbnail: []byte("measured thumbnail fixture"), Width: 16, Height: 16, DurationSeconds: 1}, nil
			})
			row, err = files.GetFileByID(ctx, id)
			require.NoError(t, err)
			t.Logf("partial=%t worked=%t ordinary_puts=%d status=%s run_error=%v", partial, worked, objects.ordinaryPuts, row.Status, runErr)
			if partial {
				assert.ErrorIs(t, runErr, store.ErrTrackedWrite)
				assert.False(t, worked)
				assert.Zero(t, objects.ordinaryPuts, "partial tracked schema must never fall back to ordinary PUT")
				assert.Equal(t, "processing", row.Status)
				assert.Nil(t, row.DeliveredFull)
				var jobState string
				require.NoError(t, pool.QueryRow(ctx, `SELECT state FROM file_gif_jobs WHERE file_id=$1`, id).Scan(&jobState))
				assert.Equal(t, "queued", jobState, "schema rejection must not claim an unfenced attempt")
				return
			}
			require.NoError(t, runErr)
			require.True(t, worked)
			require.Equal(t, 2, objects.ordinaryPuts)
			require.Equal(t, "ready", row.Status)
			require.NotNil(t, row.DeliveredFull)
		})
	}
}
