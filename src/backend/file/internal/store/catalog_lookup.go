package store

import (
	"context"
	"encoding/hex"
	"time"

	"github.com/google/uuid"
	filev1 "voice.app/voice/file/v1"
	"voice/backend/file/internal/catalogasset"
)

// LookupCatalogFile classifies an actual File under its immutable server marker.
// Ordinary is explicit only for an existing noncatalog row; unavailable catalog
// state and a missing private mapping never fall back to ordinary attachment ACL.
func (s *FilesStore) LookupCatalogFile(ctx context.Context, actor catalogasset.ActorPair, id uuid.UUID, now time.Time) (*filev1.LookupCatalogAssetResponse, error) {
	if actor.AccountID == uuid.Nil || actor.ProfileID == uuid.Nil || id == uuid.Nil {
		return nil, catalogasset.ErrAdmission
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var catalog bool
	if err = tx.QueryRow(ctx, `SELECT catalog_asset FROM files WHERE id=$1 FOR SHARE`, id).Scan(&catalog); err != nil {
		return nil, err
	}
	if !catalog {
		return &filev1.LookupCatalogAssetResponse{Classification: &filev1.LookupCatalogAssetResponse_OrdinaryFile{OrdinaryFile: true}}, tx.Commit(ctx)
	}
	row, err := getFileByID(ctx, tx, id, "")
	if err != nil {
		return nil, err
	}
	snapshot := &filev1.ValidateCatalogAssetResponse{FileId: id.String(), ActorAccountId: actor.AccountID.String(), ActorProfileId: actor.ProfileID.String(), Intent: row.UploadIntent, ScanResult: row.ScanResult, FileType: row.FileType}
	state := catalogasset.LifecycleState{Status: row.Status, IsE2E: row.IsE2E, ExpiresAt: row.ExpiresAt}
	var source uuid.UUID
	err = tx.QueryRow(ctx, `SELECT a.source_id,a.provider,a.provider_id,a.source_sha256,f.preview_metadata_revision,b.state,b.deleted_at IS NOT NULL
FROM file_catalog_assets a JOIN files f ON f.id=a.file_id JOIN file_blobs b ON b.blob_id=f.blob_id
JOIN file_catalog_actor_gates ag ON ag.id=a.account_gate_id JOIN file_catalog_actor_gates pg ON pg.id=a.profile_gate_id
WHERE a.file_id=$1 AND a.actor_account_id=$2 AND a.actor_profile_id=$3
AND ag.actor_kind='account' AND pg.actor_kind='profile' AND ag.state='LIVE' AND pg.state='LIVE'
AND ag.generation=a.account_generation AND pg.generation=a.profile_generation
FOR SHARE OF a,b,ag,pg`, id, actor.AccountID, actor.ProfileID).Scan(&source, &snapshot.Provider, &snapshot.ProviderId, &snapshot.SourceSha256, &snapshot.ObjectRevision, &state.BlobState, &state.BlobDeleting)
	if err != nil {
		return nil, err
	}
	snapshot.SourceId = source.String()
	state.ObjectRevision = snapshot.ObjectRevision
	if row.SHA256Hash == nil || row.DeliveredFull == nil {
		return nil, catalogasset.ErrAdmission
	}
	state.SourceSHA256, err = hex.DecodeString(*row.SHA256Hash)
	if err != nil {
		return nil, catalogasset.ErrAdmission
	}
	snapshot.DeliveredSha256, err = hex.DecodeString(row.DeliveredFull.SHA256)
	if err != nil {
		return nil, catalogasset.ErrAdmission
	}
	snapshot.DeliveredSizeBytes = row.DeliveredFull.SizeBytes
	snapshot.DeliveredMimeType = row.DeliveredFull.MIMEType
	if row.DeliveredFull.Key == "" || row.DeliveredFull.ProcessorVersion != GIFProcessorVersion {
		return nil, catalogasset.ErrAdmission
	}
	request := &filev1.ValidateCatalogAssetRequest{ProtocolVersion: 1, SourceId: snapshot.SourceId, Provider: snapshot.Provider, ProviderId: snapshot.ProviderId, FileId: id.String(), ActorAccountId: actor.AccountID.String(), ActorProfileId: actor.ProfileID.String()}
	if err = catalogasset.ValidateReady(request, snapshot, state, now); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &filev1.LookupCatalogAssetResponse{Classification: &filev1.LookupCatalogAssetResponse_Catalog{Catalog: snapshot}}, nil
}

// ValidateCatalogFile uses the same locked reader as classification and binds
// its canonical result to the exact tuple approved independently by Chat.
func (s *FilesStore) ValidateCatalogFile(ctx context.Context, request *filev1.ValidateCatalogAssetRequest, now time.Time) (*filev1.ValidateCatalogAssetResponse, error) {
	if request == nil || request.ProtocolVersion != 1 {
		return nil, catalogasset.ErrAdmission
	}
	account, e := uuid.Parse(request.ActorAccountId)
	if e != nil {
		return nil, catalogasset.ErrAdmission
	}
	profile, e := uuid.Parse(request.ActorProfileId)
	if e != nil {
		return nil, catalogasset.ErrAdmission
	}
	id, e := uuid.Parse(request.FileId)
	if e != nil {
		return nil, catalogasset.ErrAdmission
	}
	out, e := s.LookupCatalogFile(ctx, catalogasset.ActorPair{AccountID: account, ProfileID: profile}, id, now)
	if e != nil {
		return nil, e
	}
	snapshot := out.GetCatalog()
	if snapshot == nil {
		return nil, catalogasset.ErrAdmission
	}
	if snapshot.SourceId != request.SourceId || snapshot.Provider != request.Provider || snapshot.ProviderId != request.ProviderId || snapshot.ActorAccountId != request.ActorAccountId || snapshot.ActorProfileId != request.ActorProfileId || snapshot.FileId != request.FileId {
		return nil, catalogasset.ErrAdmission
	}
	return snapshot, nil
}
