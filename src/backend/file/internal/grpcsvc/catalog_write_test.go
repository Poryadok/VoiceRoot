package grpcsvc

import (
	"bytes"
	"context"
	"crypto/sha256"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	chatv1 "voice.app/voice/chat/v1"
	filev1 "voice.app/voice/file/v1"
	"voice/backend/file/internal/catalogasset"
	"voice/backend/file/internal/store"
)

type pendingCatalogFixture struct {
	pool     *pgxpool.Pool
	files    *store.FilesStore
	keys     *catalogasset.ReplayKeys
	actor    catalogasset.ActorPair
	capture  store.CatalogActorCapture
	request  *filev1.ImportCatalogAssetRequest
	imported store.CatalogImport
}

func TestCatalogFetchFailureKeepsFiniteReceiptWithoutInventedSize(t *testing.T) {
	ctx := context.Background()
	f := seedPendingCatalog(t, ctx)
	require.NoError(t, f.files.FailCatalogFetch(ctx, f.imported, f.keys))
	var count int
	require.NoError(t, f.pool.QueryRow(ctx, `SELECT count(*) FROM files WHERE id=$1`, f.imported.FileID).Scan(&count))
	require.Zero(t, count)
	var completed, deadline time.Time
	var state string
	require.NoError(t, f.pool.QueryRow(ctx, `SELECT outcome,completed_at,retain_until FROM file_catalog_import_operations`).Scan(&state, &completed, &deadline))
	require.Equal(t, "failed", state)
	require.True(t, completed.Add(30*24*time.Hour).Equal(deadline))
	r := &filev1.GetCatalogAssetStatusRequest{ProtocolVersion: 1, ActorAccountId: f.actor.AccountID.String(), ActorProfileId: f.actor.ProfileID.String(), Selector: &filev1.GetCatalogAssetStatusRequest_Operation{Operation: &filev1.CatalogImportOperationSelector{OperationId: f.request.OperationId, Provider: f.request.Provider, ProviderId: f.request.ProviderId, DestinationChat: f.request.DestinationChat}}}
	out, err := f.files.CatalogStatus(ctx, r, f.keys, time.Now())
	require.NoError(t, err)
	require.Equal(t, "failed", out.Status)
	require.Equal(t, f.imported.FileID.String(), out.FileId)
	f.request.OperationId = uuid.NewString()
	_, _, err = f.files.BeginCatalogImport(ctx, f.request, f.capture, f.keys, chatv1.ChatType_CHAT_TYPE_GROUP, nil, 50<<20)
	require.ErrorIs(t, err, store.ErrCatalogTerminal, "new operation must use a new source generation after terminal failure")
}

func seedCatalogInput(t *testing.T, ctx context.Context) pendingCatalogFixture {
	t.Helper()
	var f pendingCatalogFixture
	f.pool = startR23FilePostgres(t, ctx)
	applyFileGateSQL(t, ctx, f.pool, filepath.Join("src", "backend", "migrations", "file_db", "000010_catalog_asset_import.up.sql"))
	var err error
	f.keys, err = catalogasset.NewReplayKeys("fixture", map[string][]byte{"fixture": bytes.Repeat([]byte{1}, 32)})
	require.NoError(t, err)
	f.files = store.NewFilesStore(f.pool)
	f.actor = catalogasset.ActorPair{AccountID: uuid.New(), ProfileID: uuid.New()}
	f.capture, err = f.files.CaptureCatalogActor(ctx, f.actor, f.keys)
	require.NoError(t, err)
	f.request = &filev1.ImportCatalogAssetRequest{ProtocolVersion: 1, OperationId: uuid.NewString(), SourceId: uuid.NewString(), Provider: "fixture", ProviderId: "asset-1", SourceUrl: "https://example.com/source.gif", ActorAccountId: f.actor.AccountID.String(), ActorProfileId: f.actor.ProfileID.String(), DestinationChat: &chatv1.ChatRef{Id: uuid.NewString()}, ChatAccountGeneration: uuid.NewString(), ChatProfileGeneration: uuid.NewString()}
	return f
}

func seedPendingCatalog(t *testing.T, ctx context.Context) pendingCatalogFixture {
	t.Helper()
	f := seedCatalogInput(t, ctx)
	var err error
	var created bool
	f.imported, created, err = f.files.BeginCatalogImport(ctx, f.request, f.capture, f.keys, chatv1.ChatType_CHAT_TYPE_GROUP, nil, 50<<20)
	require.NoError(t, err)
	require.True(t, created)
	return f
}

func TestCatalogWriteRequiresMeasuredFactsAndOnlyQueuesProcessing(t *testing.T) {
	ctx := context.Background()
	f := seedPendingCatalog(t, ctx)
	body := []byte("GIF89a-fetched-fixture")
	digest := sha256.Sum256(body)
	require.NoError(t, f.files.RecordCatalogFetched(ctx, f.imported, f.capture, f.keys, int64(len(body)), digest[:]))
	row, err := f.files.GetFileByID(ctx, f.imported.FileID)
	require.NoError(t, err)
	require.Equal(t, int64(len(body)), row.SizeBytes)
	require.Equal(t, "pending_upload", row.Status)
	var state string
	require.NoError(t, f.pool.QueryRow(ctx, `SELECT state FROM file_catalog_writes WHERE file_id=$1`, f.imported.FileID).Scan(&state))
	require.Equal(t, "DECLARED", state)
	require.NoError(t, f.files.StartCatalogWrite(ctx, f.imported, f.capture, f.keys))
	require.NoError(t, f.pool.QueryRow(ctx, `SELECT state FROM file_catalog_writes WHERE file_id=$1`, f.imported.FileID).Scan(&state))
	require.Equal(t, "STARTED", state)
	require.NoError(t, f.files.SettleCatalogWrite(ctx, f.imported, f.capture, f.keys, true))
	row, err = f.files.GetFileByID(ctx, f.imported.FileID)
	require.NoError(t, err)
	require.Equal(t, "processing", row.Status, "PUT success is not READY proof")
	var jobs int
	require.NoError(t, f.pool.QueryRow(ctx, `SELECT count(*) FROM file_gif_jobs WHERE file_id=$1`, f.imported.FileID).Scan(&jobs))
	require.Equal(t, 1, jobs)
	require.NoError(t, f.pool.QueryRow(ctx, `SELECT state FROM file_catalog_writes WHERE file_id=$1`, f.imported.FileID).Scan(&state))
	require.Equal(t, "SETTLED", state)
}

func TestCatalogWriteErrorRemainsUnsettledAndUnsendable(t *testing.T) {
	ctx := context.Background()
	f := seedPendingCatalog(t, ctx)
	digest := sha256.Sum256([]byte("GIF89a-source"))
	require.NoError(t, f.files.RecordCatalogFetched(ctx, f.imported, f.capture, f.keys, 13, digest[:]))
	require.NoError(t, f.files.StartCatalogWrite(ctx, f.imported, f.capture, f.keys))
	require.NoError(t, f.files.SettleCatalogWrite(ctx, f.imported, f.capture, f.keys, false))
	var state string
	require.NoError(t, f.pool.QueryRow(ctx, `SELECT state FROM file_catalog_writes WHERE file_id=$1`, f.imported.FileID).Scan(&state))
	require.Equal(t, "UNSETTLED", state)
	require.NoError(t, f.pool.QueryRow(ctx, `SELECT status FROM files WHERE id=$1`, f.imported.FileID).Scan(&state))
	require.Equal(t, "failed", state)
	require.NoError(t, f.pool.QueryRow(ctx, `SELECT state FROM file_blobs WHERE blob_id=$1`, f.imported.FileID).Scan(&state))
	require.Equal(t, "LIVE", state, "transport error is never a storage I/O fence")
	_, err := f.pool.Exec(ctx, `UPDATE file_blobs SET state='GC_PENDING',deleted_at=clock_timestamp() WHERE blob_id=$1`, f.imported.FileID)
	require.NoError(t, err)
	var deleting bool
	require.NoError(t, f.pool.QueryRow(ctx, `SELECT state,deleted_at IS NOT NULL FROM file_blobs WHERE blob_id=$1`, f.imported.FileID).Scan(&state, &deleting))
	require.Equal(t, "LIVE", state, "ordinary expiry/reference cleanup must not bypass unsettled write barrier")
	require.False(t, deleting)
	var jobs int
	require.NoError(t, f.pool.QueryRow(ctx, `SELECT count(*) FROM file_gif_jobs`).Scan(&jobs))
	require.Zero(t, jobs)
}

func TestCatalogLateSettledWriteCannotPublishAfterPurge(t *testing.T) {
	ctx := context.Background()
	f := seedPendingCatalog(t, ctx)
	digest := sha256.Sum256([]byte("GIF89a-source"))
	require.NoError(t, f.files.RecordCatalogFetched(ctx, f.imported, f.capture, f.keys, 13, digest[:]))
	require.NoError(t, f.files.StartCatalogWrite(ctx, f.imported, f.capture, f.keys))
	require.NoError(t, f.files.PurgeCatalogProfile(ctx, f.actor, f.keys))
	require.ErrorIs(t, f.files.SettleCatalogWrite(ctx, f.imported, f.capture, f.keys, true), store.ErrCatalogActorClosed)
	var jobs int
	require.NoError(t, f.pool.QueryRow(ctx, `SELECT count(*) FROM file_gif_jobs`).Scan(&jobs))
	require.Zero(t, jobs)
	var state string
	require.NoError(t, f.pool.QueryRow(ctx, `SELECT status FROM files WHERE id=$1`, f.imported.FileID).Scan(&state))
	require.Equal(t, "failed", state)
	require.NoError(t, f.pool.QueryRow(ctx, `SELECT state FROM file_catalog_writes WHERE file_id=$1`, f.imported.FileID).Scan(&state))
	require.Equal(t, "GC_PENDING", state, "only actual single-attempt PUT acknowledgement permits cleanup")
}
