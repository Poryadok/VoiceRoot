package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	chatv1 "voice.app/voice/chat/v1"
	filev1 "voice.app/voice/file/v1"
	"voice/backend/file/internal/store"
)

func TestCatalogReferenceAcquireRequiresCurrentReadySource(t *testing.T) {
	ctx := context.Background()
	f := seedCatalogPurgeFixture(t, ctx, true)
	seedCatalogReadyRepresentation(t, ctx, f)
	s := &FileGRPC{files: store.NewFilesStore(f.pool)}
	request := func(file uuid.UUID) *filev1.AcquireFileReferencesRequest {
		return &filev1.AcquireFileReferencesRequest{ProtocolVersion: 1, OperationId: uuid.NewString(), ProducerId: filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_MESSAGING, References: []*filev1.FileReferenceKey{{FileId: file.String(), OwnerType: filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_MESSAGE, OwnerId: uuid.NewString()}}}
	}
	acquire := func(r *filev1.AcquireFileReferencesRequest) (*filev1.AcquireFileReferencesResponse, error) {
		return s.AcquireFileReferences(catalogVerifiedContext(t, r, filev1.FileService_AcquireFileReferences_FullMethodName, "messaging"), r)
	}
	_, err := f.pool.Exec(ctx, `UPDATE files SET status='processing' WHERE id=$1`, f.file)
	require.NoError(t, err)
	_, err = acquire(request(f.file))
	require.Error(t, err, "catalog reference cannot be acquired before READY")
	_, err = f.pool.Exec(ctx, `UPDATE files SET status='ready' WHERE id=$1`, f.file)
	require.NoError(t, err)
	r := request(f.file)
	first, err := acquire(r)
	require.NoError(t, err)
	ca, cp := uuid.New(), uuid.New()
	require.NoError(t, f.pool.QueryRow(ctx, `SELECT chat_account_generation,chat_profile_generation FROM file_catalog_assets WHERE file_id=$1`, f.file).Scan(&ca, &cp))
	capture, err := s.files.CaptureCatalogActor(ctx, f.actor, f.keys)
	require.NoError(t, err)
	_, created, err := s.files.BeginCatalogImport(ctx, &filev1.ImportCatalogAssetRequest{ProtocolVersion: 2, OperationId: uuid.NewString(), SourceId: uuid.NewString(), ActorAccountId: f.actor.AccountID.String(), ActorProfileId: f.actor.ProfileID.String(), Provider: "fixture", ProviderId: "asset-1", SourceUrl: "https://example.com/source.gif", DestinationChat: &chatv1.ChatRef{Id: uuid.NewString()}, ChatAccountGeneration: ca.String(), ChatProfileGeneration: cp.String()}, capture, f.keys, chatv1.ChatType_CHAT_TYPE_GROUP, nil, 50<<20)
	require.NoError(t, err)
	require.True(t, created)
	_, err = acquire(request(f.file))
	require.Error(t, err, "new reference operation must recheck current selection")
	replay, err := acquire(r)
	require.NoError(t, err, "original completed operation preserves existing history")
	require.Equal(t, first.Receipt.ReceiptId, replay.Receipt.ReceiptId)
	var count int
	require.NoError(t, f.pool.QueryRow(ctx, `SELECT count(*) FROM file_references WHERE file_id=$1`, f.file).Scan(&count))
	require.Equal(t, 1, count)
}

func TestOrdinaryReferenceAcquireKeepsPreCatalogProcessingBehavior(t *testing.T) {
	ctx := context.Background()
	pool := startR23FilePostgres(t, ctx) // Deliberately no catalog schema.
	file := uuid.New()
	_, err := pool.Exec(ctx, `INSERT INTO files(id,uploader_profile_id,original_name,mime_type,size_bytes,r2_key,status,file_type,scan_result) VALUES($1,$2,'ordinary.png','image/png',42,'ordinary/source.png','processing','image','skipped')`, file, uuid.New())
	require.NoError(t, err)
	s := &FileGRPC{files: store.NewFilesStore(pool)}
	r := &filev1.AcquireFileReferencesRequest{ProtocolVersion: 1, OperationId: uuid.NewString(), ProducerId: filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_MESSAGING, References: []*filev1.FileReferenceKey{{FileId: file.String(), OwnerType: filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_MESSAGE, OwnerId: uuid.NewString()}}}
	_, err = s.AcquireFileReferences(catalogVerifiedContext(t, r, filev1.FileService_AcquireFileReferences_FullMethodName, "messaging"), r)
	require.NoError(t, err)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM file_references WHERE file_id=$1`, file).Scan(&count))
	require.Equal(t, 1, count)
}

func TestCatalogConcurrentReferencesSerializeAtBlobWithoutLockUpgrade(t *testing.T) {
	ctx := context.Background()
	f := seedCatalogPurgeFixture(t, ctx, true)
	seedCatalogReadyRepresentation(t, ctx, f)
	s := &FileGRPC{files: store.NewFilesStore(f.pool)}
	results := make(chan error, 2)
	start := make(chan struct{})
	for range 2 {
		r := &filev1.AcquireFileReferencesRequest{ProtocolVersion: 1, OperationId: uuid.NewString(), ProducerId: filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_MESSAGING, References: []*filev1.FileReferenceKey{{FileId: f.file.String(), OwnerType: filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_MESSAGE, OwnerId: uuid.NewString()}}}
		verified := catalogVerifiedContext(t, r, filev1.FileService_AcquireFileReferences_FullMethodName, "messaging")
		go func() { <-start; _, err := s.AcquireFileReferences(verified, r); results <- err }()
	}
	close(start)
	for range 2 {
		require.NoError(t, <-results)
	}
	var count int
	require.NoError(t, f.pool.QueryRow(ctx, `SELECT count(*) FROM file_references WHERE file_id=$1`, f.file).Scan(&count))
	require.Equal(t, 2, count)
}
