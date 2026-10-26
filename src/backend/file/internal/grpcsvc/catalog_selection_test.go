package grpcsvc

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	chatv1 "voice.app/voice/chat/v1"
	filev1 "voice.app/voice/file/v1"
	"voice/backend/file/internal/catalogasset"
	"voice/backend/file/internal/store"
)

func TestCatalogSelectionAdmissionOrderSurvivesRetryAndRetirement(t *testing.T) {
	ctx := context.Background()
	f := seedPendingCatalog(t, ctx)
	require.EqualValues(t, 1, f.imported.SelectionRevision)
	b := proto.Clone(f.request).(*filev1.ImportCatalogAssetRequest)
	b.OperationId, b.SourceId = uuid.NewString(), uuid.NewString()
	second, created, err := f.files.BeginCatalogImport(ctx, b, f.capture, f.keys, chatv1.ChatType_CHAT_TYPE_GROUP, nil, 50<<20)
	require.NoError(t, err)
	require.True(t, created)
	require.EqualValues(t, 2, second.SelectionRevision)
	require.Equal(t, f.imported.SelectionHead, second.SelectionHead)
	old, created, err := f.files.BeginCatalogImport(ctx, f.request, f.capture, f.keys, chatv1.ChatType_CHAT_TYPE_GROUP, nil, 50<<20)
	require.NoError(t, err)
	require.False(t, created)
	require.EqualValues(t, 1, old.SelectionRevision)
	require.EqualValues(t, 2, old.CurrentSelectionRevision)
	status, err := f.files.CatalogStatus(ctx, &filev1.GetCatalogAssetStatusRequest{ProtocolVersion: 2, ActorAccountId: f.actor.AccountID.String(), ActorProfileId: f.actor.ProfileID.String(), Selector: &filev1.GetCatalogAssetStatusRequest_Asset{Asset: &filev1.CatalogAssetSelector{SourceId: f.imported.SourceID.String(), FileId: f.imported.FileID.String(), Provider: f.request.Provider, ProviderId: f.request.ProviderId}}}, f.keys, time.Now())
	require.NoError(t, err)
	require.EqualValues(t, 1, status.SelectionRevision)
	require.EqualValues(t, 2, status.CurrentSelectionRevision)
	require.NoError(t, f.files.FailCatalogFetch(ctx, second, f.keys))
	var revision int64
	var fence uuid.UUID
	require.NoError(t, f.pool.QueryRow(ctx, `SELECT revision,terminal_fence_id FROM file_catalog_selection_heads WHERE id=$1 AND current_file_id IS NULL`, second.SelectionHead).Scan(&revision, &fence))
	require.EqualValues(t, 2, revision)
	require.NotEqual(t, uuid.Nil, fence)
	old, created, err = f.files.BeginCatalogImport(ctx, f.request, f.capture, f.keys, chatv1.ChatType_CHAT_TYPE_GROUP, nil, 50<<20)
	require.NoError(t, err)
	require.False(t, created)
	require.EqualValues(t, 2, old.CurrentSelectionRevision)
	b.OperationId, b.SourceId = uuid.NewString(), uuid.NewString()
	third, created, err := f.files.BeginCatalogImport(ctx, b, f.capture, f.keys, chatv1.ChatType_CHAT_TYPE_GROUP, nil, 50<<20)
	require.NoError(t, err)
	require.True(t, created)
	require.EqualValues(t, 3, third.SelectionRevision)
}

func TestCatalogConcurrentAdmissionsHaveOneCommitOrderedCounter(t *testing.T) {
	ctx := context.Background()
	f := seedPendingCatalog(t, ctx)
	type result struct {
		revision int64
		err      error
	}
	results := make(chan result, 2)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for range 2 {
		r := proto.Clone(f.request).(*filev1.ImportCatalogAssetRequest)
		r.OperationId, r.SourceId = uuid.NewString(), uuid.NewString()
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			out, _, err := f.files.BeginCatalogImport(ctx, r, f.capture, f.keys, chatv1.ChatType_CHAT_TYPE_GROUP, nil, 50<<20)
			results <- result{out.SelectionRevision, err}
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	seen := map[int64]bool{}
	for out := range results {
		require.NoError(t, out.err)
		seen[out.revision] = true
	}
	require.Equal(t, map[int64]bool{2: true, 3: true}, seen)
	var revision int64
	require.NoError(t, f.pool.QueryRow(ctx, `SELECT revision FROM file_catalog_selection_heads WHERE id=$1`, f.imported.SelectionHead).Scan(&revision))
	require.EqualValues(t, 3, revision)
}

func TestCatalogLegacyOperationCannotBecomeFreshOrderedAdmission(t *testing.T) {
	ctx := context.Background()
	f := seedPendingCatalog(t, ctx)
	_, err := f.pool.Exec(ctx, `UPDATE file_catalog_import_operations SET selection_revision=NULL`)
	require.ErrorContains(t, err, "catalog_operation_selection_proof")
	_, err = f.pool.Exec(ctx, `UPDATE file_catalog_import_operations SET authority_protocol=1,selection_head_id=NULL,selection_revision=NULL`)
	require.NoError(t, err)
	_, _, err = f.files.BeginCatalogImport(ctx, f.request, f.capture, f.keys, chatv1.ChatType_CHAT_TYPE_GROUP, nil, 50<<20)
	require.ErrorIs(t, err, store.ErrCatalogTerminal)
	_, err = f.files.CatalogStatus(ctx, &filev1.GetCatalogAssetStatusRequest{ProtocolVersion: 2, ActorAccountId: f.actor.AccountID.String(), ActorProfileId: f.actor.ProfileID.String(), Selector: &filev1.GetCatalogAssetStatusRequest_Operation{Operation: &filev1.CatalogImportOperationSelector{OperationId: f.request.OperationId, Provider: f.request.Provider, ProviderId: f.request.ProviderId, DestinationChat: f.request.DestinationChat}}}, f.keys, time.Now())
	require.ErrorIs(t, err, store.ErrCatalogTerminal, "known v1 operation must never return fresh NotFound")
	var count int
	require.NoError(t, f.pool.QueryRow(ctx, `SELECT count(*) FROM files`).Scan(&count))
	require.Equal(t, 1, count)
	down, err := os.ReadFile(filepath.Join(fileGateRepoRoot(t), "src", "backend", "migrations", "file_db", "000012_catalog_selection_authority.down.sql"))
	require.NoError(t, err)
	_, err = f.pool.Exec(ctx, string(down))
	require.ErrorContains(t, err, "catalog selection authority must not be forgotten")
}

func TestCatalogReadySelectionCannotSurviveNewPendingAdmissionOrKeyRotation(t *testing.T) {
	ctx := context.Background()
	f := seedCatalogPurgeFixture(t, ctx, true)
	seedCatalogReadyRepresentation(t, ctx, f)
	files := store.NewFilesStore(f.pool)
	_, err := files.LookupCatalogFile(ctx, f.actor, f.file, time.Now())
	require.NoError(t, err)
	keys, err := catalogasset.NewReplayKeys("rotated", map[string][]byte{"fixture": bytes.Repeat([]byte{1}, 32), "rotated": bytes.Repeat([]byte{2}, 32)})
	require.NoError(t, err)
	capture, err := files.CaptureCatalogActor(ctx, f.actor, keys)
	require.NoError(t, err)
	var ca, cp uuid.UUID
	require.NoError(t, f.pool.QueryRow(ctx, `SELECT chat_account_generation,chat_profile_generation FROM file_catalog_assets WHERE file_id=$1`, f.file).Scan(&ca, &cp))
	r := &filev1.ImportCatalogAssetRequest{ProtocolVersion: 2, OperationId: uuid.NewString(), SourceId: uuid.NewString(), ActorAccountId: f.actor.AccountID.String(), ActorProfileId: f.actor.ProfileID.String(), Provider: "fixture", ProviderId: "asset-1", SourceUrl: "https://example.com/source.gif", DestinationChat: &chatv1.ChatRef{Id: uuid.NewString()}, ChatAccountGeneration: ca.String(), ChatProfileGeneration: cp.String()}
	b, created, err := files.BeginCatalogImport(ctx, r, capture, keys, chatv1.ChatType_CHAT_TYPE_GROUP, nil, 50<<20)
	require.NoError(t, err)
	require.True(t, created)
	require.EqualValues(t, 2, b.SelectionRevision, "key rotation must not fork the selector counter")
	_, err = files.LookupCatalogFile(ctx, f.actor, f.file, time.Now())
	require.Error(t, err, "older real READY File is superseded by pending B")
	require.NoError(t, files.FailCatalogFetch(ctx, b, keys))
	_, err = files.LookupCatalogFile(ctx, f.actor, f.file, time.Now())
	require.Error(t, err, "terminal B never restores READY A")
	var count int
	require.NoError(t, f.pool.QueryRow(ctx, `SELECT count(*) FROM file_catalog_selection_heads`).Scan(&count))
	require.Equal(t, 1, count)
	incomplete, err := catalogasset.NewReplayKeys("rotated", map[string][]byte{"rotated": bytes.Repeat([]byte{2}, 32)})
	require.NoError(t, err)
	_, err = files.CaptureCatalogActor(ctx, f.actor, incomplete)
	require.ErrorIs(t, err, catalogasset.ErrReplayKeys, "retained selector key cannot be silently retired")
	r.ProtocolVersion = 1
	_, _, err = files.BeginCatalogImport(ctx, r, capture, keys, chatv1.ChatType_CHAT_TYPE_GROUP, nil, 50<<20)
	require.Error(t, err, "protocol1 cannot create unordered authority")
}
