package store

import (
	"context"
	"encoding/hex"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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
	out, err := lookupCatalogFileTx(ctx, tx, actor, id, now, false)
	if err != nil {
		return nil, err
	}
	return out, tx.Commit(ctx)
}

func lookupCatalogFileTx(ctx context.Context, tx pgx.Tx, actor catalogasset.ActorPair, id uuid.UUID, now time.Time, exclusiveBlob bool) (*filev1.LookupCatalogAssetResponse, error) {
	var catalog bool
	if err := tx.QueryRow(ctx, `SELECT catalog_asset FROM files WHERE id=$1 FOR SHARE`, id).Scan(&catalog); err != nil {
		return nil, err
	}
	if !catalog {
		return &filev1.LookupCatalogAssetResponse{Classification: &filev1.LookupCatalogAssetResponse_OrdinaryFile{OrdinaryFile: true}}, nil
	}
	row, err := getFileByID(ctx, tx, id, "")
	if err != nil {
		return nil, err
	}
	snapshot := &filev1.ValidateCatalogAssetResponse{FileId: id.String(), ActorAccountId: actor.AccountID.String(), ActorProfileId: actor.ProfileID.String(), Intent: row.UploadIntent, ScanResult: row.ScanResult, FileType: row.FileType}
	state := catalogasset.LifecycleState{Status: row.Status, IsE2E: row.IsE2E, ExpiresAt: row.ExpiresAt}
	var source uuid.UUID
	var head *uuid.UUID
	var revision *int64
	var protocol int
	lockClause := " FOR SHARE OF a,b,ag,pg"
	if exclusiveBlob {
		lockClause = " FOR SHARE OF a,ag,pg FOR UPDATE OF b"
	}
	err = tx.QueryRow(ctx, `SELECT a.source_id,a.provider,a.provider_id,a.source_sha256,f.preview_metadata_revision,b.state,b.deleted_at IS NOT NULL,a.authority_protocol,a.selection_head_id,a.selection_revision
FROM file_catalog_assets a JOIN files f ON f.id=a.file_id JOIN file_blobs b ON b.blob_id=f.blob_id
JOIN file_catalog_actor_gates ag ON ag.id=a.account_gate_id JOIN file_catalog_actor_gates pg ON pg.id=a.profile_gate_id
WHERE a.file_id=$1 AND a.actor_account_id=$2 AND a.actor_profile_id=$3
AND ag.actor_kind='account' AND pg.actor_kind='profile' AND ag.state='LIVE' AND pg.state='LIVE'
AND ag.generation=a.account_generation AND pg.generation=a.profile_generation`+lockClause, id, actor.AccountID, actor.ProfileID).Scan(&source, &snapshot.Provider, &snapshot.ProviderId, &snapshot.SourceSha256, &snapshot.ObjectRevision, &state.BlobState, &state.BlobDeleting, &protocol, &head, &revision)
	if err != nil {
		return nil, err
	}
	snapshot.SourceId = source.String()
	if protocol != 2 || head == nil || revision == nil {
		return nil, catalogasset.ErrAdmission
	}
	current, isCurrent, err := readCatalogSelectionTx(ctx, tx, *head, *revision, source, id)
	if err != nil {
		return nil, err
	}
	if !isCurrent {
		return nil, catalogasset.ErrAdmission
	}
	snapshot.SelectionRevision = uint64(*revision)
	snapshot.CurrentSelectionRevision = uint64(current)
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
	request := &filev1.ValidateCatalogAssetRequest{ProtocolVersion: 2, SourceId: snapshot.SourceId, Provider: snapshot.Provider, ProviderId: snapshot.ProviderId, FileId: id.String(), ActorAccountId: actor.AccountID.String(), ActorProfileId: actor.ProfileID.String()}
	if err = catalogasset.ValidateReady(request, snapshot, state, now); err != nil {
		return nil, err
	}
	return &filev1.LookupCatalogAssetResponse{Classification: &filev1.LookupCatalogAssetResponse_Catalog{Catalog: snapshot}}, nil
}

// GuardCatalogReferenceTx reuses File's READY/current-source authority inside
// the durable reference transaction. It grants no user or destination access;
// Messaging independently authorizes those before calling lifecycle acquisition.
// Ordinary pre-catalog schemas/files keep their existing lifecycle behavior.
func GuardCatalogReferenceTx(ctx context.Context, tx pgx.Tx, id uuid.UUID, now time.Time) (bool, error) {
	var catalog bool
	if err := tx.QueryRow(ctx, `SELECT COALESCE((to_jsonb(f)->>'catalog_asset')::boolean,false) FROM files f WHERE id=$1`, id).Scan(&catalog); err != nil {
		return false, err
	}
	if !catalog {
		return false, nil
	}
	var actor catalogasset.ActorPair
	if err := tx.QueryRow(ctx, `SELECT actor_account_id,actor_profile_id FROM file_catalog_assets WHERE file_id=$1`, id).Scan(&actor.AccountID, &actor.ProfileID); err != nil {
		return true, err
	}
	out, err := lookupCatalogFileTx(ctx, tx, actor, id, now, true)
	if err != nil {
		return true, err
	}
	if out.GetCatalog() == nil {
		return true, catalogasset.ErrAdmission
	}
	return true, nil
}

// ValidateCatalogFile uses the same locked reader as classification and binds
// its canonical result to the exact tuple approved independently by Chat.
func (s *FilesStore) ValidateCatalogFile(ctx context.Context, request *filev1.ValidateCatalogAssetRequest, now time.Time) (*filev1.ValidateCatalogAssetResponse, error) {
	if request == nil || request.ProtocolVersion != 2 {
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
