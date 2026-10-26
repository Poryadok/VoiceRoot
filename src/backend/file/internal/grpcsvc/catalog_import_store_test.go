package grpcsvc

import (
	"context"
	"crypto/sha256"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"bytes"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	chatv1 "voice.app/voice/chat/v1"
	filev1 "voice.app/voice/file/v1"
	"voice/backend/file/internal/catalogasset"
	"voice/backend/file/internal/store"
)

type catalogDestinationFunc func(context.Context, catalogasset.DestinationRequest) (chatv1.ChatType, error)

func (f catalogDestinationFunc) Authorize(ctx context.Context, r catalogasset.DestinationRequest) (chatv1.ChatType, error) {
	return f(ctx, r)
}

func TestCatalogImportReservationIsRealFileAndImmutableRetry(t *testing.T) {
	ctx := context.Background()
	pool := startR23FilePostgres(t, ctx)
	applyFileGateSQL(t, ctx, pool, filepath.Join("src", "backend", "migrations", "file_db", "000010_catalog_asset_import.up.sql"))
	keys, err := catalogasset.NewReplayKeys("fixture", map[string][]byte{"fixture": bytes.Repeat([]byte{1}, 32)})
	require.NoError(t, err)
	files := store.NewFilesStore(pool)
	actor := catalogasset.ActorPair{AccountID: uuid.New(), ProfileID: uuid.New()}
	capture, err := files.CaptureCatalogActor(ctx, actor, keys)
	require.NoError(t, err)
	r := &filev1.ImportCatalogAssetRequest{ProtocolVersion: 1, OperationId: uuid.NewString(), SourceId: uuid.NewString(), Provider: "fixture", ProviderId: "asset-1", SourceUrl: "https://example.com/source.gif?secret=private", ActorAccountId: actor.AccountID.String(), ActorProfileId: actor.ProfileID.String(), DestinationChat: &chatv1.ChatRef{Id: uuid.NewString()}, ChatAccountGeneration: uuid.NewString(), ChatProfileGeneration: uuid.NewString()}
	first, created, err := files.BeginCatalogImport(ctx, r, capture, keys, chatv1.ChatType_CHAT_TYPE_GROUP, nil, 50<<20)
	require.NoError(t, err)
	require.True(t, created)
	var state string
	var size int64
	var unknown bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT status,size_bytes,catalog_source_size_unknown FROM files WHERE id=$1`, first.FileID).Scan(&state, &size, &unknown))
	require.Equal(t, "pending_upload", state)
	require.Zero(t, size)
	require.True(t, unknown)
	statusRequest := &filev1.GetCatalogAssetStatusRequest{ProtocolVersion: 1, ActorAccountId: r.ActorAccountId, ActorProfileId: r.ActorProfileId, Selector: &filev1.GetCatalogAssetStatusRequest_Operation{Operation: &filev1.CatalogImportOperationSelector{OperationId: r.OperationId, Provider: r.Provider, ProviderId: r.ProviderId, DestinationChat: r.DestinationChat}}}
	recovered, err := files.CatalogStatus(ctx, statusRequest, keys, time.Now())
	require.NoError(t, err)
	require.Equal(t, first.FileID.String(), recovered.FileId)
	require.Equal(t, r.ChatAccountGeneration, recovered.ChatAccountGeneration)
	statusRequest.GetOperation().ProviderId = "changed-selection"
	_, err = files.CatalogStatus(ctx, statusRequest, keys, time.Now())
	require.ErrorIs(t, err, store.ErrCatalogConflict)
	statusRequest.GetOperation().ProviderId = r.ProviderId
	_, err = files.GetFileByID(ctx, first.FileID)
	require.Error(t, err, "unknown internal size must not become public measured metadata")
	second, created, err := files.BeginCatalogImport(ctx, r, capture, keys, chatv1.ChatType_CHAT_TYPE_GROUP, nil, 50<<20)
	require.NoError(t, err)
	require.False(t, created)
	require.Equal(t, first.FileID, second.FileID)
	r.SourceUrl = "https://example.com/rotated.gif"
	_, _, err = files.BeginCatalogImport(ctx, r, capture, keys, chatv1.ChatType_CHAT_TYPE_GROUP, nil, 50<<20)
	require.ErrorIs(t, err, store.ErrCatalogConflict)
	require.NoError(t, files.PurgeCatalogProfile(ctx, actor, keys))
	_, err = files.CatalogStatus(ctx, statusRequest, keys, time.Now())
	require.ErrorIs(t, err, store.ErrCatalogTerminal, "missing raw receipt plus fence is never a fresh import")
	_, _, err = files.BeginCatalogImport(ctx, r, capture, keys, chatv1.ChatType_CHAT_TYPE_GROUP, nil, 50<<20)
	require.Error(t, err, "old operation cannot bootstrap after purge")
	_, err = files.CaptureCatalogActor(ctx, actor, keys)
	require.ErrorIs(t, err, store.ErrCatalogActorClosed, "ordinary live lookup cannot restore gate")
}

func TestCatalogOperationCompactionPreservesIndependentActiveAssetStatus(t *testing.T) {
	ctx := context.Background()
	f := seedCatalogPurgeFixture(t, ctx, true)
	files := store.NewFilesStore(f.pool)
	var destination uuid.UUID
	require.NoError(t, f.pool.QueryRow(ctx, `SELECT destination_chat_id FROM file_catalog_import_operations`).Scan(&destination))
	public := &filev1.ImportCatalogAssetRequest{ProtocolVersion: 1, OperationId: f.operation.String(), ActorAccountId: f.actor.AccountID.String(), ActorProfileId: f.actor.ProfileID.String(), Provider: "fixture", ProviderId: "asset-1", DestinationChat: &chatv1.ChatRef{Id: destination.String()}}
	raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(public)
	require.NoError(t, err)
	hash := sha256.Sum256(raw)
	_, err = f.pool.Exec(ctx, `UPDATE file_catalog_import_operations SET selection_sha256=$1`, hash[:])
	require.NoError(t, err)
	_, err = f.pool.Exec(ctx, `UPDATE files SET status='processing'; UPDATE files SET status='ready'`)
	require.NoError(t, err)
	var completed, deadline time.Time
	require.NoError(t, f.pool.QueryRow(ctx, `SELECT completed_at,retain_until FROM file_catalog_import_operations`).Scan(&completed, &deadline))
	require.WithinDuration(t, completed.Add(30*24*time.Hour), deadline, time.Microsecond)
	r := &filev1.GetCatalogAssetStatusRequest{ProtocolVersion: 1, ActorAccountId: public.ActorAccountId, ActorProfileId: public.ActorProfileId, Selector: &filev1.GetCatalogAssetStatusRequest_Operation{Operation: &filev1.CatalogImportOperationSelector{OperationId: public.OperationId, Provider: public.Provider, ProviderId: public.ProviderId, DestinationChat: public.DestinationChat}}}
	out, err := files.CatalogStatus(ctx, r, f.keys, completed.Add(time.Hour))
	require.NoError(t, err)
	require.Equal(t, "ready", out.Status)
	deny := false
	destinationCalls := 0
	server := &CatalogAssetGRPC{Files: files, Keys: f.keys, Actors: catalogActorSnapshotFunc(func(context.Context, []catalogasset.ActorPair) ([]catalogasset.ActorState, error) {
		return []catalogasset.ActorState{catalogasset.ActorLive}, nil
	}), Destination: catalogDestinationFunc(func(_ context.Context, request catalogasset.DestinationRequest) (chatv1.ChatType, error) {
		destinationCalls++
		require.Equal(t, f.actor, request.Actor)
		require.Equal(t, destination.String(), request.Destination.Id)
		require.Equal(t, out.ChatAccountGeneration, request.ChatAccountGeneration.String())
		require.Equal(t, out.ChatProfileGeneration, request.ChatProfileGeneration.String())
		if deny {
			return 0, errors.New("current membership denied")
		}
		return chatv1.ChatType_CHAT_TYPE_GROUP, nil
	})}
	actual, err := server.GetCatalogAssetStatus(catalogVerifiedContext(t, r, filev1.CatalogAssetService_GetCatalogAssetStatus_FullMethodName, "chat"), r)
	require.NoError(t, err)
	require.Equal(t, f.file.String(), actual.FileId)
	deny = true
	actual, err = server.GetCatalogAssetStatus(catalogVerifiedContext(t, r, filev1.CatalogAssetService_GetCatalogAssetStatus_FullMethodName, "chat"), r)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	require.Nil(t, actual)
	require.Equal(t, 2, destinationCalls)
	var after time.Time
	require.NoError(t, f.pool.QueryRow(ctx, `SELECT retain_until FROM file_catalog_import_operations`).Scan(&after))
	require.True(t, deadline.Equal(after), "poll must not refresh retention")
	r.GetOperation().ProviderId = "changed-body-after-compaction"
	_, err = files.CatalogStatus(ctx, r, f.keys, deadline)
	require.ErrorIs(t, err, store.ErrCatalogTerminal, "compacted receipt does not retain changed-body comparison promises")
	var count int
	require.NoError(t, f.pool.QueryRow(ctx, `SELECT count(*) FROM file_catalog_import_operations`).Scan(&count))
	require.Zero(t, count)
	r.Selector = &filev1.GetCatalogAssetStatusRequest_Asset{Asset: &filev1.CatalogAssetSelector{SourceId: f.source.String(), FileId: f.file.String(), Provider: "fixture", ProviderId: "asset-1"}}
	out, err = files.CatalogStatus(ctx, r, f.keys, deadline)
	require.NoError(t, err)
	require.Equal(t, "ready", out.Status)
	require.Empty(t, out.TerminalFenceId)
	require.Empty(t, out.OperationId)
	require.Nil(t, out.DestinationChat)
	require.NoError(t, files.PurgeCatalogProfile(ctx, f.actor, f.keys))
	out, err = files.CatalogStatus(ctx, r, f.keys, deadline)
	require.NoError(t, err)
	require.Equal(t, "deleted", out.Status)
	require.NotEmpty(t, out.TerminalFenceId)
	require.Empty(t, out.ChatAccountGeneration)
	require.Empty(t, out.ChatProfileGeneration)
}
