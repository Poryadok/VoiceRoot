package grpcsvc

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"voice/backend/file/internal/store"
	"voice/backend/pkg/principal"

	commonv1 "voice.app/voice/common/v1"
	filev1 "voice.app/voice/file/v1"
)

func (s *FileGRPC) AcquireFileReferences(ctx context.Context, req *filev1.AcquireFileReferencesRequest) (*filev1.AcquireFileReferencesResponse, error) {
	caller, err := requireFileService(ctx, producerService(req.GetProducerId()), filev1.FileService_AcquireFileReferences_FullMethodName, req)
	if err != nil {
		return nil, err
	}
	if err := requireLifecycleStore(s); err != nil {
		return nil, err
	}
	if req.GetProtocolVersion() != 1 || len(req.GetReferences()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "protocol_version=1 and references are required")
	}
	op, err := parseUUID("operation_id", req.GetOperationId())
	if err != nil {
		return nil, err
	}
	requestBytes, requestHash, err := lifecycleRequest(req)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	replay := &filev1.AcquireFileReferencesResponse{}
	if found, loadErr := loadReferenceOperation(ctx, s.files.Pool, caller, op, requestHash, replay); loadErr != nil {
		return nil, loadErr
	} else if found {
		return replay, nil
	}
	tx, err := s.files.Pool.Begin(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	defer func() { _ = tx.Rollback(ctx) }()
	for _, ref := range req.GetReferences() {
		ids, validationErr := parseReference(ref)
		if validationErr != nil {
			return nil, validationErr
		}
		if !producerOwnsReference(req.GetProducerId(), ref.GetOwnerType()) {
			return nil, status.Error(codes.PermissionDenied, "producer cannot acquire this owner type")
		}
		if ids.scope != nil {
			state, fenceErr := lockLiveFence(ctx, tx, *ids.scope)
			if fenceErr != nil {
				return nil, fenceErr
			}
			if state != "LIVE" {
				return nil, status.Error(codes.FailedPrecondition, "Space is not LIVE")
			}
		}
		var blobID uuid.UUID
		var blobState string
		var deletionStartedAt *time.Time
		if err := tx.QueryRow(ctx, `SELECT b.blob_id,b.state,b.deleted_at FROM files f JOIN file_blobs b ON b.blob_id=f.blob_id WHERE f.id=$1 FOR UPDATE OF b`, ids.file).Scan(&blobID, &blobState, &deletionStartedAt); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, status.Error(codes.NotFound, "file not found")
			}
			return nil, status.Error(codes.Internal, err.Error())
		}
		if (blobState != "LIVE" && blobState != "GC_PENDING") || deletionStartedAt != nil {
			return nil, status.Error(codes.FailedPrecondition, "blob already collected")
		}
		var exists bool
		err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM file_references WHERE file_id=$1 AND owner_type=$2 AND owner_id=$3 AND subresource_id IS NOT DISTINCT FROM $4 AND scope_space_id IS NOT DISTINCT FROM $5 AND released_at IS NULL)`, ids.file, int32(ref.GetOwnerType()), ids.owner, ids.subresource, ids.scope).Scan(&exists)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		if !exists {
			if _, err := tx.Exec(ctx, `INSERT INTO file_references(file_id,owner_type,owner_id,subresource_id,scope_space_id) VALUES($1,$2,$3,$4,$5)`, ids.file, int32(ref.GetOwnerType()), ids.owner, ids.subresource, ids.scope); err != nil {
				return nil, status.Error(codes.Internal, err.Error())
			}
		}
		if _, err := tx.Exec(ctx, `UPDATE file_blobs SET state='LIVE', next_attempt_at=NULL WHERE blob_id=$1 AND state='GC_PENDING'`, blobID); err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	response := &filev1.AcquireFileReferencesResponse{Receipt: &filev1.AcquireFileReferencesReceipt{ProtocolVersion: 1, ReceiptId: uuid.NewString(), OperationId: req.GetOperationId(), ProducerId: req.GetProducerId(), ReferenceCount: uint64(len(req.GetReferences())), RequestSha256: requestHash, CompletedAt: timestamppb.Now()}}
	if err := saveReferenceOperation(ctx, tx, caller, op, requestBytes, requestHash, response); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return response, nil
}

func (s *FileGRPC) ReleaseFileReferences(ctx context.Context, req *filev1.ReleaseFileReferencesRequest) (*filev1.ReleaseFileReferencesResponse, error) {
	caller, err := requireFileService(ctx, producerService(req.GetProducerId()), filev1.FileService_ReleaseFileReferences_FullMethodName, req)
	if err != nil {
		return nil, err
	}
	if err := requireLifecycleStore(s); err != nil {
		return nil, err
	}
	if req.GetProtocolVersion() != 1 || len(req.GetReferences()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "protocol_version=1 and references are required")
	}
	op, err := parseUUID("operation_id", req.GetOperationId())
	if err != nil {
		return nil, err
	}
	requestBytes, requestHash, err := lifecycleRequest(req)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	replay := &filev1.ReleaseFileReferencesResponse{}
	if found, loadErr := loadReferenceOperation(ctx, s.files.Pool, caller, op, requestHash, replay); loadErr != nil {
		return nil, loadErr
	} else if found {
		return replay, nil
	}
	tx, err := s.files.Pool.Begin(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var released uint64
	for _, ref := range req.GetReferences() {
		ids, validationErr := parseReference(ref)
		if validationErr != nil {
			return nil, validationErr
		}
		if !producerOwnsReference(req.GetProducerId(), ref.GetOwnerType()) {
			return nil, status.Error(codes.PermissionDenied, "producer cannot release this owner type")
		}
		if ids.scope != nil {
			state, fenceErr := lockLiveFence(ctx, tx, *ids.scope)
			if fenceErr != nil {
				return nil, fenceErr
			}
			if state != "LIVE" {
				return nil, status.Error(codes.FailedPrecondition, "Space is not LIVE")
			}
		}
		var blobID uuid.UUID
		if err := tx.QueryRow(ctx, `SELECT b.blob_id FROM files f JOIN file_blobs b ON b.blob_id=f.blob_id WHERE f.id=$1 FOR UPDATE OF b`, ids.file).Scan(&blobID); err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		tag, execErr := tx.Exec(ctx, `UPDATE file_references SET released_at=clock_timestamp(),release_operation_id=$6 WHERE file_id=$1 AND owner_type=$2 AND owner_id=$3 AND subresource_id IS NOT DISTINCT FROM $4 AND scope_space_id IS NOT DISTINCT FROM $5 AND released_at IS NULL`, ids.file, int32(ref.GetOwnerType()), ids.owner, ids.subresource, ids.scope, op)
		if execErr != nil {
			return nil, status.Error(codes.Internal, execErr.Error())
		}
		released += uint64(tag.RowsAffected())
		if _, err := tx.Exec(ctx, `UPDATE file_blobs b SET state='GC_PENDING',gc_operation_id=COALESCE(gc_operation_id,$2),next_attempt_at=clock_timestamp() WHERE blob_id=$1 AND NOT EXISTS(SELECT 1 FROM files f JOIN file_references r ON r.file_id=f.id WHERE f.blob_id=b.blob_id AND r.released_at IS NULL)`, blobID, uuid.New()); err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	response := &filev1.ReleaseFileReferencesResponse{Receipt: &filev1.ReleaseFileReferencesReceipt{ProtocolVersion: 1, ReceiptId: uuid.NewString(), OperationId: req.GetOperationId(), ProducerId: req.GetProducerId(), ReleasedCount: released, RequestSha256: requestHash, CompletedAt: timestamppb.Now()}}
	if err := saveReferenceOperation(ctx, tx, caller, op, requestBytes, requestHash, response); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return response, nil
}

func (s *FileGRPC) IssueFileAccessCapability(ctx context.Context, req *filev1.IssueFileAccessCapabilityRequest) (*filev1.IssueFileAccessCapabilityResponse, error) {
	caller, err := requireFileService(ctx, ownerService(req.GetReference().GetOwnerType()), filev1.FileService_IssueFileAccessCapability_FullMethodName, req)
	if err != nil {
		return nil, err
	}
	if err := requireLifecycleStore(s); err != nil {
		return nil, err
	}
	if req.GetProtocolVersion() != 1 || req.GetReference() == nil || req.GetExpiresAt() == nil || len(req.GetAllowedSurfaces()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "invalid capability")
	}
	for i, surface := range req.GetAllowedSurfaces() {
		if surface == filev1.FileReadSurface_FILE_READ_SURFACE_UNSPECIFIED || (i > 0 && surface <= req.GetAllowedSurfaces()[i-1]) {
			return nil, status.Error(codes.InvalidArgument, "allowed_surfaces must be sorted and unique")
		}
	}
	op, err := parseUUID("operation_id", req.GetOperationId())
	if err != nil {
		return nil, err
	}
	subject, err := parseUUID("subject_profile_id", req.GetSubjectProfileId())
	if err != nil {
		return nil, err
	}
	ids, err := parseReference(req.GetReference())
	if err != nil {
		return nil, err
	}
	requestBytes, requestHash, err := lifecycleRequest(req)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	replay := &filev1.IssueFileAccessCapabilityResponse{}
	if found, loadErr := loadReferenceOperation(ctx, s.files.Pool, caller, op, requestHash, replay); loadErr != nil {
		return nil, loadErr
	} else if found {
		return replay, nil
	}
	tx, err := s.files.Pool.Begin(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var now time.Time
	if err := tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&now); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	expires := req.GetExpiresAt().AsTime().UTC()
	if !expires.After(now) || expires.After(now.Add(time.Hour)) {
		return nil, status.Error(codes.InvalidArgument, "invalid capability expiry")
	}
	if ids.scope != nil {
		var state string
		if err := tx.QueryRow(ctx, `SELECT state FROM file_space_lifecycle_fences WHERE space_id=$1 FOR SHARE`, *ids.scope).Scan(&state); err != nil || state != "LIVE" {
			return nil, status.Error(codes.FailedPrecondition, "Space is not LIVE")
		}
	}
	var lockedFile uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT file_id FROM file_references WHERE file_id=$1 AND owner_type=$2 AND owner_id=$3 AND subresource_id IS NOT DISTINCT FROM $4 AND scope_space_id IS NOT DISTINCT FROM $5 AND released_at IS NULL FOR SHARE`, ids.file, int32(req.GetReference().GetOwnerType()), ids.owner, ids.subresource, ids.scope).Scan(&lockedFile); err != nil {
		return nil, status.Error(codes.FailedPrecondition, "reference is not live")
	}
	var blobState, fileState string
	if err := tx.QueryRow(ctx, `SELECT f.status,b.state FROM files f JOIN file_blobs b ON b.blob_id=f.blob_id WHERE f.id=$1 FOR SHARE OF f,b`, ids.file).Scan(&fileState, &blobState); err != nil || fileState == "deleted" || blobState != "LIVE" {
		return nil, status.Error(codes.FailedPrecondition, "file is not live")
	}
	capabilityID := uuid.New()
	surfaces := make([]int32, len(req.GetAllowedSurfaces()))
	for i, v := range req.GetAllowedSurfaces() {
		surfaces[i] = int32(v)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO file_access_capabilities(capability_id,file_id,owner_type,owner_id,subresource_id,scope_space_id,subject_profile_id,allowed_surfaces,expires_at,issue_operation_id,request_bytes,request_sha256) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, capabilityID, ids.file, int32(req.GetReference().GetOwnerType()), ids.owner, ids.subresource, ids.scope, subject, surfaces, expires, op, requestBytes, requestHash); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	response := &filev1.IssueFileAccessCapabilityResponse{Receipt: &filev1.IssueFileAccessCapabilityReceipt{ProtocolVersion: 1, ReceiptId: uuid.NewString(), OperationId: req.GetOperationId(), CapabilityId: capabilityID.String(), Reference: proto.Clone(req.GetReference()).(*filev1.FileReferenceKey), SubjectProfileId: subject.String(), AllowedSurfaces: append([]filev1.FileReadSurface(nil), req.GetAllowedSurfaces()...), ExpiresAt: timestamppb.New(expires), RequestSha256: requestHash, CompletedAt: timestamppb.Now()}}
	if err := saveReferenceOperation(ctx, tx, caller, op, requestBytes, requestHash, response); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return response, nil
}

func (s *FileGRPC) PrepareSpaceDeletionReferenceManifest(ctx context.Context, req *filev1.PrepareSpaceDeletionReferenceManifestRequest) (*filev1.PrepareSpaceDeletionReferenceManifestResponse, error) {
	_, err := requireFileService(ctx, "space", filev1.FileService_PrepareSpaceDeletionReferenceManifest_FullMethodName, req)
	if err != nil {
		return nil, err
	}
	if err := requireLifecycleStore(s); err != nil {
		return nil, err
	}
	spaceID, err := parseUUID("space_id", req.GetSpaceId())
	if err != nil {
		return nil, err
	}
	deletionID, err := parseUUID("deletion_operation_id", req.GetDeletionOperationId())
	if err != nil {
		return nil, err
	}
	if req.GetProtocolVersion() != 1 || req.GetScheduleGeneration() == 0 || req.GetChatManifest() == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid manifest prepare")
	}
	requestBytes, requestHash, err := lifecycleRequest(req)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	operationCaller := "space.prepare:" + spaceID.String() + ":" + strconv.FormatUint(req.GetScheduleGeneration(), 10)
	replay := &filev1.PrepareSpaceDeletionReferenceManifestResponse{}
	if found, loadErr := loadReferenceOperation(ctx, s.files.Pool, operationCaller, deletionID, requestHash, replay); loadErr != nil {
		return nil, loadErr
	} else if found {
		return replay, nil
	}
	chatBytes, _ := proto.MarshalOptions{Deterministic: true}.Marshal(req.GetChatManifest())
	tx, err := s.files.Pool.Begin(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var current string
	var currentGeneration uint64
	scanErr := tx.QueryRow(ctx, `SELECT state,generation FROM file_space_lifecycle_fences WHERE space_id=$1 FOR UPDATE`, spaceID).Scan(&current, &currentGeneration)
	if scanErr != nil && !errors.Is(scanErr, pgx.ErrNoRows) {
		return nil, status.Error(codes.Internal, scanErr.Error())
	}
	missingFence := errors.Is(scanErr, pgx.ErrNoRows)
	if missingFence {
		if _, err := tx.Exec(ctx, `INSERT INTO file_space_lifecycle_fences(space_id,generation,state) VALUES($1,0,'LIVE')`, spaceID); err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		current, currentGeneration = "LIVE", 0
	}
	if current != "LIVE" || (!missingFence && req.GetScheduleGeneration() != currentGeneration+1) {
		return nil, status.Error(codes.FailedPrecondition, "manifest prepare is stale, changed, or non-contiguous")
	}
	_, err = tx.Exec(ctx, `UPDATE file_space_lifecycle_fences SET generation=$2,state='FROZEN',deletion_operation_id=$3,preliminary_chat_hash=$4,final_manifest_hash=NULL,updated_at=clock_timestamp() WHERE space_id=$1`, spaceID, req.GetScheduleGeneration(), deletionID, chatBytes)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	for producer := 1; producer <= 3; producer++ {
		if _, err := tx.Exec(ctx, `INSERT INTO file_space_deletion_manifests(space_id,deletion_operation_id,schedule_generation,producer_id) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`, spaceID, deletionID, req.GetScheduleGeneration(), producer); err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	response := &filev1.PrepareSpaceDeletionReferenceManifestResponse{Receipt: &filev1.PrepareSpaceDeletionReferenceManifestReceipt{ProtocolVersion: 1, ReceiptId: uuid.NewString(), SpaceId: spaceID.String(), DeletionOperationId: deletionID.String(), ScheduleGeneration: req.GetScheduleGeneration(), AppliedState: commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, RequestSha256: requestHash, AppliedAt: timestamppb.Now()}}
	if err := saveReferenceOperation(ctx, tx, operationCaller, deletionID, requestBytes, requestHash, response); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return response, nil
}

func (s *FileGRPC) RegisterSpaceDeletionReferenceChunk(ctx context.Context, req *filev1.RegisterSpaceDeletionReferenceChunkRequest) (*filev1.RegisterSpaceDeletionReferenceChunkResponse, error) {
	_, err := requireFileService(ctx, producerService(req.GetProducerId()), filev1.FileService_RegisterSpaceDeletionReferenceChunk_FullMethodName, req)
	if err != nil {
		return nil, err
	}
	if err := requireLifecycleStore(s); err != nil {
		return nil, err
	}
	if req.GetProtocolVersion() != 1 || len(req.GetReferences()) > 1000 {
		return nil, status.Error(codes.InvalidArgument, "invalid chunk")
	}
	if err := validateSortedReferences(req.GetReferences()); err != nil {
		return nil, err
	}
	op, err := parseUUID("operation_id", req.GetOperationId())
	if err != nil {
		return nil, err
	}
	spaceID, err := parseUUID("space_id", req.GetSpaceId())
	if err != nil {
		return nil, err
	}
	for _, ref := range req.GetReferences() {
		ids, parseErr := parseReference(ref)
		if parseErr != nil {
			return nil, parseErr
		}
		if ids.scope == nil || *ids.scope != spaceID {
			return nil, status.Error(codes.InvalidArgument, "manifest reference scope does not match Space")
		}
		if !producerOwnsReference(req.GetProducerId(), ref.GetOwnerType()) {
			return nil, status.Error(codes.PermissionDenied, "producer cannot declare this owner type")
		}
	}
	deletionID, err := parseUUID("deletion_operation_id", req.GetDeletionOperationId())
	if err != nil {
		return nil, err
	}
	requestBytes, requestHash, _ := lifecycleRequest(req)
	var oldHash, receiptBytes []byte
	loadErr := s.files.Pool.QueryRow(ctx, `SELECT request_sha256,receipt_bytes FROM file_space_deletion_manifest_chunks WHERE operation_id=$1`, op).Scan(&oldHash, &receiptBytes)
	if loadErr == nil {
		if !bytes.Equal(oldHash, requestHash) {
			return nil, status.Error(codes.AlreadyExists, "operation changed")
		}
		out := &filev1.RegisterSpaceDeletionReferenceChunkResponse{}
		if err := proto.Unmarshal(receiptBytes, out); err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		return out, nil
	} else if !errors.Is(loadErr, pgx.ErrNoRows) {
		return nil, status.Error(codes.Internal, loadErr.Error())
	}
	tx, err := s.files.Pool.Begin(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var fenceState string
	var fenceGeneration uint64
	var fenceDeletion uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT state,generation,deletion_operation_id FROM file_space_lifecycle_fences WHERE space_id=$1 FOR SHARE`, spaceID).Scan(&fenceState, &fenceGeneration, &fenceDeletion); err != nil || fenceState != "FROZEN" || fenceGeneration != req.GetScheduleGeneration() || fenceDeletion != deletionID {
		return nil, status.Error(codes.FailedPrecondition, "manifest fence binding mismatch")
	}
	var count, chunks uint64
	var expectedHash []byte
	var sealedAt *time.Time
	if err := tx.QueryRow(ctx, `SELECT received_reference_count,received_chunk_count,expected_references_sha256,sealed_at FROM file_space_deletion_manifests WHERE space_id=$1 AND deletion_operation_id=$2 AND schedule_generation=$3 AND producer_id=$4 FOR UPDATE`, spaceID, deletionID, req.GetScheduleGeneration(), int32(req.GetProducerId())).Scan(&count, &chunks, &expectedHash, &sealedAt); err != nil {
		return nil, status.Error(codes.FailedPrecondition, "manifest not prepared")
	}
	if sealedAt != nil {
		return nil, status.Error(codes.FailedPrecondition, "producer already sealed")
	}
	if req.GetChunkIndex() != chunks {
		return nil, status.Error(codes.FailedPrecondition, "chunk index is not contiguous")
	}
	if chunks > 0 && !bytes.Equal(expectedHash, req.GetExpectedReferencesSha256()) {
		return nil, status.Error(codes.FailedPrecondition, "aggregate binding changed")
	}
	allRefs, err := loadManifestReferences(ctx, tx, spaceID, deletionID, req.GetScheduleGeneration(), req.GetProducerId())
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	allRefs = append(allRefs, req.GetReferences()...)
	newCount := count + uint64(len(req.GetReferences()))
	if req.GetSealsProducer() {
		if newCount != req.GetExpectedTotalCount() || !bytes.Equal(referenceManifestHash(deletionID, spaceID, req.GetScheduleGeneration(), req.GetProducerId(), allRefs), req.GetExpectedReferencesSha256()) {
			return nil, status.Error(codes.FailedPrecondition, "seal aggregate mismatch")
		}
	}
	response := &filev1.RegisterSpaceDeletionReferenceChunkResponse{Receipt: &filev1.RegisterSpaceDeletionReferenceChunkReceipt{ProtocolVersion: 1, ReceiptId: uuid.NewString(), OperationId: op.String(), DeletionOperationId: deletionID.String(), SpaceId: spaceID.String(), ScheduleGeneration: req.GetScheduleGeneration(), ChunkIndex: req.GetChunkIndex(), AcceptedCount: uint64(len(req.GetReferences())), ProducerId: req.GetProducerId(), ExpectedTotalCount: req.GetExpectedTotalCount(), ExpectedReferencesSha256: req.GetExpectedReferencesSha256(), ProducerSealed: req.GetSealsProducer(), RequestSha256: requestHash, CompletedAt: timestamppb.Now()}}
	receiptBytes, _ = proto.MarshalOptions{Deterministic: true}.Marshal(response)
	refsBytes, _ := proto.MarshalOptions{Deterministic: true}.Marshal(req)
	if _, err := tx.Exec(ctx, `INSERT INTO file_space_deletion_manifest_chunks(operation_id,space_id,deletion_operation_id,schedule_generation,producer_id,chunk_index,request_bytes,request_sha256,receipt_bytes,references_bytes) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, op, spaceID, deletionID, req.GetScheduleGeneration(), int32(req.GetProducerId()), req.GetChunkIndex(), requestBytes, requestHash, receiptBytes, refsBytes); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	_, err = tx.Exec(ctx, `UPDATE file_space_deletion_manifests SET expected_total_count=$5,expected_references_sha256=$6,received_chunk_count=received_chunk_count+1,received_reference_count=$7,sealed_at=CASE WHEN $8 THEN clock_timestamp() ELSE NULL END WHERE space_id=$1 AND deletion_operation_id=$2 AND schedule_generation=$3 AND producer_id=$4`, spaceID, deletionID, req.GetScheduleGeneration(), int32(req.GetProducerId()), req.GetExpectedTotalCount(), req.GetExpectedReferencesSha256(), newCount, req.GetSealsProducer())
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return response, nil
}

func (s *FileGRPC) ApplySpaceLifecycleFence(ctx context.Context, req *filev1.ApplySpaceLifecycleFenceRequest) (*filev1.ApplySpaceLifecycleFenceResponse, error) {
	_, err := requireFileService(ctx, "space", filev1.FileService_ApplySpaceLifecycleFence_FullMethodName, req)
	if err != nil {
		return nil, err
	}
	if err := requireLifecycleStore(s); err != nil {
		return nil, err
	}
	f := req.GetFence()
	if f == nil || f.GetProtocolVersion() != 1 || f.GetManifest() == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid fence")
	}
	spaceID, err := parseUUID("space_id", f.GetSpaceId())
	if err != nil {
		return nil, err
	}
	deletionID, err := parseUUID("deletion_operation_id", f.GetDeletionOperationId())
	if err != nil {
		return nil, err
	}
	requestBytes, requestHash, err := lifecycleRequest(req)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	operationCaller := "space.apply:" + spaceID.String() + ":" + strconv.FormatUint(f.GetGeneration(), 10) + ":" + strconv.Itoa(int(f.GetDesiredState()))
	replay := &filev1.ApplySpaceLifecycleFenceResponse{}
	if found, loadErr := loadReferenceOperation(ctx, s.files.Pool, operationCaller, deletionID, requestHash, replay); loadErr != nil {
		return nil, loadErr
	} else if found {
		return replay, nil
	}
	tx, err := s.files.Pool.Begin(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var current string
	var currentGeneration uint64
	var storedDeletion uuid.UUID
	var chatBytes, finalHash []byte
	if err := tx.QueryRow(ctx, `SELECT state,generation,deletion_operation_id,preliminary_chat_hash,final_manifest_hash FROM file_space_lifecycle_fences WHERE space_id=$1 FOR UPDATE`, spaceID).Scan(&current, &currentGeneration, &storedDeletion, &chatBytes, &finalHash); err != nil {
		return nil, status.Error(codes.FailedPrecondition, "fence not prepared")
	}
	if storedDeletion != deletionID {
		return nil, status.Error(codes.FailedPrecondition, "deletion mismatch")
	}
	state := f.GetDesiredState()
	switch state {
	case commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN:
		if current != "FROZEN" || f.GetGeneration() != currentGeneration || len(finalHash) != 0 {
			return nil, status.Error(codes.FailedPrecondition, "final freeze is stale or already applied")
		}
		root, rootErr := loadSpaceManifestRoot(ctx, tx, spaceID, deletionID, uint64(f.GetGeneration()), chatBytes)
		if rootErr != nil {
			return nil, rootErr
		}
		if !proto.Equal(root, f.GetManifest()) {
			return nil, status.Error(codes.FailedPrecondition, "manifest root mismatch")
		}
		finalHash = root.GetManifestSha256()
		current = "FROZEN"
	case commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_LIVE:
		root, rootErr := loadSpaceManifestRoot(ctx, tx, spaceID, deletionID, currentGeneration, chatBytes)
		if rootErr != nil || current != "FROZEN" || len(finalHash) == 0 || f.GetGeneration() != currentGeneration+1 || !proto.Equal(root, f.GetManifest()) {
			return nil, status.Error(codes.FailedPrecondition, "restore binding mismatch")
		}
		current = "LIVE"
	case commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED:
		root, rootErr := loadSpaceManifestRoot(ctx, tx, spaceID, deletionID, currentGeneration, chatBytes)
		if rootErr != nil || current != "FROZEN" || len(finalHash) == 0 || f.GetGeneration() != currentGeneration+1 || !proto.Equal(root, f.GetManifest()) {
			return nil, status.Error(codes.FailedPrecondition, "purge decision binding mismatch")
		}
		current = "PURGE_DECIDED"
	default:
		return nil, status.Error(codes.InvalidArgument, "unsupported fence state")
	}
	if _, err := tx.Exec(ctx, `UPDATE file_space_lifecycle_fences SET generation=$2,state=$3,final_manifest_hash=$4,updated_at=clock_timestamp() WHERE space_id=$1`, spaceID, f.GetGeneration(), current, finalHash); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	response := &filev1.ApplySpaceLifecycleFenceResponse{Receipt: &commonv1.SpaceLifecycleFenceReceipt{ProtocolVersion: 1, ReceiptId: uuid.NewString(), SpaceId: spaceID.String(), DeletionOperationId: deletionID.String(), Generation: f.GetGeneration(), ParticipantId: commonv1.ParticipantId_PARTICIPANT_ID_FILE, AppliedState: state, RequestSha256: requestHash, ManifestSha256: f.GetManifest().GetManifestSha256(), AppliedAt: timestamppb.Now()}}
	if err := saveReferenceOperation(ctx, tx, operationCaller, deletionID, requestBytes, requestHash, response); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return response, nil
}

func (s *FileGRPC) ReleaseSpaceDeletionProducerReferences(ctx context.Context, req *filev1.ReleaseSpaceDeletionProducerReferencesRequest) (*filev1.ReleaseSpaceDeletionProducerReferencesResponse, error) {
	_, err := requireFileService(ctx, producerService(req.GetProducerId()), filev1.FileService_ReleaseSpaceDeletionProducerReferences_FullMethodName, req)
	if err != nil {
		return nil, err
	}
	if err := requireLifecycleStore(s); err != nil {
		return nil, err
	}
	spaceID, err := parseUUID("space_id", req.GetSpaceId())
	if err != nil {
		return nil, err
	}
	deletionID, err := parseUUID("deletion_operation_id", req.GetDeletionOperationId())
	if err != nil {
		return nil, err
	}
	requestBytes, requestHash, _ := lifecycleRequest(req)
	var oldHash, receiptBytes []byte
	loadErr := s.files.Pool.QueryRow(ctx, `SELECT request_sha256,receipt_bytes FROM file_space_deletion_producer_releases WHERE space_id=$1 AND deletion_operation_id=$2 AND purge_generation=$3 AND producer_id=$4`, spaceID, deletionID, req.GetPurgeGeneration(), int32(req.GetProducerId())).Scan(&oldHash, &receiptBytes)
	if loadErr == nil {
		if !bytes.Equal(oldHash, requestHash) {
			return nil, status.Error(codes.AlreadyExists, "release changed")
		}
		out := &filev1.ReleaseSpaceDeletionProducerReferencesResponse{}
		if err := proto.Unmarshal(receiptBytes, out); err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		return out, nil
	} else if !errors.Is(loadErr, pgx.ErrNoRows) {
		return nil, status.Error(codes.Internal, loadErr.Error())
	}
	tx, err := s.files.Pool.Begin(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var fenceState string
	var fenceGeneration uint64
	var fenceDeletion uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT state,generation,deletion_operation_id FROM file_space_lifecycle_fences WHERE space_id=$1 FOR UPDATE`, spaceID).Scan(&fenceState, &fenceGeneration, &fenceDeletion); err != nil || fenceState != "PURGE_DECIDED" || fenceGeneration != req.GetPurgeGeneration() || fenceGeneration == 0 || req.GetSourceScheduleGeneration() != fenceGeneration-1 || fenceDeletion != deletionID {
		return nil, status.Error(codes.FailedPrecondition, "durable current purge decision required")
	}
	var expected []byte
	var count uint64
	var sealed *time.Time
	if err := tx.QueryRow(ctx, `SELECT expected_references_sha256,expected_total_count,sealed_at FROM file_space_deletion_manifests WHERE space_id=$1 AND deletion_operation_id=$2 AND schedule_generation=$3 AND producer_id=$4 FOR UPDATE`, spaceID, deletionID, req.GetSourceScheduleGeneration(), int32(req.GetProducerId())).Scan(&expected, &count, &sealed); err != nil || sealed == nil || !bytes.Equal(expected, req.GetExpectedReferencesSha256()) {
		return nil, status.Error(codes.FailedPrecondition, "producer seal mismatch")
	}
	refs, err := loadManifestReferences(ctx, tx, spaceID, deletionID, req.GetSourceScheduleGeneration(), req.GetProducerId())
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if uint64(len(refs)) != count {
		return nil, status.Error(codes.FailedPrecondition, "sealed producer count mismatch")
	}
	if !bytes.Equal(referenceManifestHash(deletionID, spaceID, req.GetSourceScheduleGeneration(), req.GetProducerId(), refs), expected) {
		return nil, status.Error(codes.FailedPrecondition, "durable producer tuple-set hash mismatch")
	}
	var released uint64
	for _, ref := range refs {
		ids, parseErr := parseReference(ref)
		if parseErr != nil || ids.scope == nil || *ids.scope != spaceID || !producerOwnsReference(req.GetProducerId(), ref.GetOwnerType()) {
			return nil, status.Error(codes.FailedPrecondition, "sealed producer contains a foreign reference")
		}
		var blobID uuid.UUID
		if err := tx.QueryRow(ctx, `SELECT b.blob_id FROM files f JOIN file_blobs b ON b.blob_id=f.blob_id WHERE f.id=$1 FOR UPDATE OF b`, ids.file).Scan(&blobID); err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		tag, execErr := tx.Exec(ctx, `UPDATE file_references SET released_at=clock_timestamp(),release_operation_id=$6 WHERE file_id=$1 AND owner_type=$2 AND owner_id=$3 AND subresource_id IS NOT DISTINCT FROM $4 AND scope_space_id IS NOT DISTINCT FROM $5 AND released_at IS NULL`, ids.file, int32(ref.GetOwnerType()), ids.owner, ids.subresource, ids.scope, deletionID)
		if execErr != nil {
			return nil, status.Error(codes.Internal, execErr.Error())
		}
		released += uint64(tag.RowsAffected())
		if _, err := tx.Exec(ctx, `UPDATE file_blobs b SET state='GC_PENDING',gc_operation_id=COALESCE(gc_operation_id,$2),next_attempt_at=clock_timestamp() WHERE blob_id=$1 AND NOT EXISTS(SELECT 1 FROM files f JOIN file_references r ON r.file_id=f.id WHERE f.blob_id=b.blob_id AND r.released_at IS NULL)`, blobID, uuid.New()); err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	response := &filev1.ReleaseSpaceDeletionProducerReferencesResponse{Receipt: &filev1.ReleaseSpaceDeletionProducerReferencesReceipt{ProtocolVersion: 1, ReceiptId: uuid.NewString(), DeletionOperationId: deletionID.String(), SpaceId: spaceID.String(), PurgeGeneration: req.GetPurgeGeneration(), SourceScheduleGeneration: req.GetSourceScheduleGeneration(), ProducerId: req.GetProducerId(), ReleasedCount: released, ExpectedReferencesSha256: expected, RequestSha256: requestHash, CompletedAt: timestamppb.Now()}}
	receiptBytes, _ = proto.MarshalOptions{Deterministic: true}.Marshal(response)
	if _, err := tx.Exec(ctx, `INSERT INTO file_space_deletion_producer_releases(space_id,deletion_operation_id,purge_generation,source_schedule_generation,producer_id,expected_references_sha256,request_bytes,request_sha256,receipt_bytes,released_count) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, spaceID, deletionID, req.GetPurgeGeneration(), req.GetSourceScheduleGeneration(), int32(req.GetProducerId()), expected, requestBytes, requestHash, receiptBytes, released); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return response, nil
}

func (s *FileGRPC) PurgeSpace(ctx context.Context, req *filev1.PurgeSpaceRequest) (*filev1.PurgeSpaceResponse, error) {
	_, err := requireFileService(ctx, "space", filev1.FileService_PurgeSpace_FullMethodName, req)
	if err != nil {
		return nil, err
	}
	if err := requireLifecycleStore(s); err != nil {
		return nil, err
	}
	p := req.GetPurge()
	if p == nil || p.GetProtocolVersion() != 1 || p.GetGeneration() == 0 || p.GetParticipantId() != commonv1.ParticipantId_PARTICIPANT_ID_FILE || p.GetManifest() == nil || p.GetPurgeDecidedAt() == nil || !p.GetPurgeDecidedAt().IsValid() {
		return nil, status.Error(codes.InvalidArgument, "invalid purge")
	}
	spaceID, err := parseUUID("space_id", p.GetSpaceId())
	if err != nil {
		return nil, err
	}
	deletionID, err := parseUUID("deletion_operation_id", p.GetDeletionOperationId())
	if err != nil {
		return nil, err
	}
	requestBytes, requestHash, _ := lifecycleRequest(req)
	var oldHash, receiptBytes []byte
	loadErr := s.files.Pool.QueryRow(ctx, `SELECT request_sha256,receipt_bytes FROM file_space_purge_receipts WHERE space_id=$1 AND deletion_operation_id=$2 AND generation=$3`, spaceID, deletionID, p.GetGeneration()).Scan(&oldHash, &receiptBytes)
	if loadErr == nil {
		if !bytes.Equal(oldHash, requestHash) {
			return nil, status.Error(codes.AlreadyExists, "purge changed")
		}
		out := &filev1.PurgeSpaceResponse{}
		if err := proto.Unmarshal(receiptBytes, out); err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		return out, nil
	} else if !errors.Is(loadErr, pgx.ErrNoRows) {
		return nil, status.Error(codes.Internal, loadErr.Error())
	}
	tx, err := s.files.Pool.Begin(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var state string
	var fenceGeneration uint64
	var finalHash, chatBytes []byte
	if err := tx.QueryRow(ctx, `SELECT state,generation,final_manifest_hash,preliminary_chat_hash FROM file_space_lifecycle_fences WHERE space_id=$1 AND deletion_operation_id=$2 FOR UPDATE`, spaceID, deletionID).Scan(&state, &fenceGeneration, &finalHash, &chatBytes); err != nil || state != "PURGE_DECIDED" || fenceGeneration != p.GetGeneration() {
		return nil, status.Error(codes.FailedPrecondition, "purge decision manifest mismatch")
	}
	root, rootErr := loadSpaceManifestRoot(ctx, tx, spaceID, deletionID, p.GetGeneration()-1, chatBytes)
	if rootErr != nil || !proto.Equal(root, p.GetManifest()) || !bytes.Equal(finalHash, root.GetManifestSha256()) {
		return nil, status.Error(codes.FailedPrecondition, "purge decision manifest mismatch")
	}
	var releaseCount int
	if err := tx.QueryRow(ctx, `
SELECT count(*)
FROM file_space_deletion_producer_releases r
JOIN file_space_deletion_manifests m
  ON m.space_id=r.space_id
 AND m.deletion_operation_id=r.deletion_operation_id
 AND m.schedule_generation=r.source_schedule_generation
 AND m.producer_id=r.producer_id
WHERE r.space_id=$1 AND r.deletion_operation_id=$2
  AND r.purge_generation=$3 AND r.source_schedule_generation=$4
  AND m.sealed_at IS NOT NULL
  AND m.expected_references_sha256=r.expected_references_sha256`, spaceID, deletionID, p.GetGeneration(), p.GetGeneration()-1).Scan(&releaseCount); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if releaseCount != 3 {
		return nil, status.Error(codes.FailedPrecondition, "all producer releases required")
	}
	response := &filev1.PurgeSpaceResponse{Receipt: &commonv1.SpacePurgeReceipt{ProtocolVersion: 1, ReceiptId: uuid.NewString(), SpaceId: spaceID.String(), DeletionOperationId: deletionID.String(), Generation: p.GetGeneration(), ParticipantId: commonv1.ParticipantId_PARTICIPANT_ID_FILE, State: commonv1.PurgeReceiptState_PURGE_RECEIPT_STATE_COMPLETED, RequestSha256: requestHash, CompletedAt: timestamppb.Now()}}
	receiptBytes, _ = proto.MarshalOptions{Deterministic: true}.Marshal(response)
	if _, err := tx.Exec(ctx, `INSERT INTO file_space_purge_receipts(space_id,deletion_operation_id,generation,request_bytes,request_sha256,manifest_sha256,receipt_bytes) VALUES($1,$2,$3,$4,$5,$6,$7)`, spaceID, deletionID, p.GetGeneration(), requestBytes, requestHash, finalHash, receiptBytes); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if _, err := tx.Exec(ctx, `UPDATE file_space_lifecycle_fences SET generation=$2,state='PURGED',updated_at=clock_timestamp() WHERE space_id=$1`, spaceID, p.GetGeneration()); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return response, nil
}

func (s *FileGRPC) GetSpacePurgeReceipt(ctx context.Context, req *filev1.GetSpacePurgeReceiptRequest) (*filev1.GetSpacePurgeReceiptResponse, error) {
	_, err := requireFileService(ctx, "space", filev1.FileService_GetSpacePurgeReceipt_FullMethodName, req)
	if err != nil {
		return nil, err
	}
	if err := requireLifecycleStore(s); err != nil {
		return nil, err
	}
	spaceID, err := parseUUID("space_id", req.GetSpaceId())
	if err != nil {
		return nil, err
	}
	deletionID, err := parseUUID("deletion_operation_id", req.GetDeletionOperationId())
	if err != nil {
		return nil, err
	}
	var requestHash, manifestHash, receiptBytes []byte
	if err := s.files.Pool.QueryRow(ctx, `SELECT request_sha256,manifest_sha256,receipt_bytes FROM file_space_purge_receipts WHERE space_id=$1 AND deletion_operation_id=$2 AND generation=$3`, spaceID, deletionID, req.GetGeneration()).Scan(&requestHash, &manifestHash, &receiptBytes); err != nil {
		return nil, status.Error(codes.NotFound, "purge receipt not found")
	}
	if !bytes.Equal(requestHash, req.GetPurgeRequestSha256()) || !bytes.Equal(manifestHash, req.GetManifestSha256()) {
		return nil, status.Error(codes.FailedPrecondition, "receipt binding mismatch")
	}
	stored := &filev1.PurgeSpaceResponse{}
	if err := proto.Unmarshal(receiptBytes, stored); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &filev1.GetSpacePurgeReceiptResponse{Receipt: stored.GetReceipt()}, nil
}

func (s *FileGRPC) fileAccessibleBySelector(ctx context.Context, fileID, profileID uuid.UUID, selector *filev1.FileAccessSelector, surface filev1.FileReadSurface) (store.FileRow, error) {
	tx, err := s.files.Pool.Begin(ctx)
	if err != nil {
		return store.FileRow{}, status.Error(codes.Internal, err.Error())
	}
	defer func() { _ = tx.Rollback(ctx) }()
	row, err := s.fileAccessibleBySelectorTx(ctx, tx, fileID, profileID, selector, surface)
	if err != nil {
		return store.FileRow{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return store.FileRow{}, status.Error(codes.Internal, err.Error())
	}
	return row, nil
}

func (s *FileGRPC) fileAccessibleBySelectorTx(ctx context.Context, tx pgx.Tx, fileID, profileID uuid.UUID, selector *filev1.FileAccessSelector, surface filev1.FileReadSurface) (store.FileRow, error) {
	if selector == nil {
		return store.FileRow{}, status.Error(codes.PermissionDenied, "exact access selector required")
	}
	var ref *filev1.FileReferenceKey
	delegated := false
	switch selected := selector.GetSelector().(type) {
	case *filev1.FileAccessSelector_Reference:
		ref = selected.Reference
	case *filev1.FileAccessSelector_CapabilityId:
		capabilityID, err := parseUUID("capability_id", selected.CapabilityId)
		if err != nil {
			return store.FileRow{}, status.Error(codes.PermissionDenied, "invalid capability")
		}
		var storedFile, owner, subject uuid.UUID
		var ownerType int32
		var subresource, scope *uuid.UUID
		var surfaces []int32
		err = tx.QueryRow(ctx, `
SELECT file_id,owner_type,owner_id,subresource_id,scope_space_id,subject_profile_id,allowed_surfaces
FROM file_access_capabilities
WHERE capability_id=$1 AND expires_at > clock_timestamp()`, capabilityID).Scan(&storedFile, &ownerType, &owner, &subresource, &scope, &subject, &surfaces)
		if err != nil || storedFile != fileID || subject != profileID {
			return store.FileRow{}, status.Error(codes.PermissionDenied, "capability denied")
		}
		allowed := false
		for _, candidate := range surfaces {
			if candidate == int32(surface) {
				allowed = true
				break
			}
		}
		if !allowed {
			return store.FileRow{}, status.Error(codes.PermissionDenied, "surface denied")
		}
		ref = &filev1.FileReferenceKey{FileId: storedFile.String(), OwnerType: filev1.FileReferenceOwnerType(ownerType), OwnerId: owner.String()}
		if subresource != nil {
			value := subresource.String()
			ref.SubresourceId = &value
		}
		if scope != nil {
			value := scope.String()
			ref.ScopeSpaceId = &value
		}
		delegated = true
	default:
		return store.FileRow{}, status.Error(codes.PermissionDenied, "exact access selector required")
	}
	return s.authorizeExactReferenceTx(ctx, tx, fileID, profileID, ref, delegated)
}

func (s *FileGRPC) authorizeExactReferenceTx(ctx context.Context, tx pgx.Tx, fileID, profileID uuid.UUID, ref *filev1.FileReferenceKey, delegated bool) (store.FileRow, error) {
	ids, err := parseReference(ref)
	if err != nil || ids.file != fileID {
		return store.FileRow{}, status.Error(codes.PermissionDenied, "reference does not match file")
	}
	// Canonical lock order is fence, exact reference, file row, blob row.
	if ids.scope != nil {
		var state string
		if err := tx.QueryRow(ctx, `SELECT state FROM file_space_lifecycle_fences WHERE space_id=$1 FOR SHARE`, *ids.scope).Scan(&state); err != nil || state != "LIVE" {
			return store.FileRow{}, status.Error(codes.FailedPrecondition, "Space is not LIVE")
		}
	}
	var lockedFile uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT file_id FROM file_references WHERE file_id=$1 AND owner_type=$2 AND owner_id=$3 AND subresource_id IS NOT DISTINCT FROM $4 AND scope_space_id IS NOT DISTINCT FROM $5 AND released_at IS NULL FOR SHARE`, ids.file, int32(ref.GetOwnerType()), ids.owner, ids.subresource, ids.scope).Scan(&lockedFile); err != nil {
		return store.FileRow{}, status.Error(codes.FailedPrecondition, "reference is not live")
	}
	row, err := s.files.GetFileByIDTx(ctx, tx, fileID, false)
	if err != nil {
		return store.FileRow{}, status.Error(codes.NotFound, "file not found")
	}
	var blobState string
	if err := tx.QueryRow(ctx, `SELECT b.state FROM files f JOIN file_blobs b ON b.blob_id=f.blob_id WHERE f.id=$1 FOR SHARE OF b`, fileID).Scan(&blobState); err != nil || blobState != "LIVE" || row.Status == "deleted" {
		return store.FileRow{}, status.Error(codes.FailedPrecondition, "file is not live")
	}
	if !delegated {
		if ref.GetOwnerType() == filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_PROFILE_AVATAR && ids.owner == profileID {
			return row, nil
		}
		if err := s.ensureFileAccess(ctx, row, profileID); err != nil {
			return store.FileRow{}, err
		}
	}
	return row, nil
}

func requireLifecycleStore(s *FileGRPC) error {
	if s == nil || s.files == nil || s.files.Pool == nil {
		return status.Error(codes.FailedPrecondition, "file persistence not configured")
	}
	return nil
}
func requireFileService(ctx context.Context, want, rpc string, req proto.Message) (string, error) {
	p, ok := principal.FromContext(ctx)
	if !ok {
		return "", status.Error(codes.PermissionDenied, "verified service principal required")
	}
	hash, err := principal.RequestHash(req)
	if err != nil {
		return "", status.Error(codes.PermissionDenied, "request binding invalid")
	}
	if want == "" || p.Kind != "service" || p.Issuer != want || p.Subject != "service:"+want || p.Audience != "file" || p.RPC != rpc || p.RequestHash != hash {
		return "", status.Error(codes.PermissionDenied, "service principal binding mismatch")
	}
	return p.Issuer, nil
}
func producerService(id filev1.FileReferenceProducerId) string {
	switch id {
	case filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_SPACE:
		return "space"
	case filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_CHAT:
		return "chat"
	case filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_MESSAGING:
		return "messaging"
	}
	return ""
}
func producerOwnsReference(producer filev1.FileReferenceProducerId, owner filev1.FileReferenceOwnerType) bool {
	switch producer {
	case filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_SPACE:
		return owner == filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_SPACE_AVATAR
	case filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_CHAT:
		return owner == filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_CHAT_AVATAR
	case filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_MESSAGING:
		return owner == filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_MESSAGE ||
			owner == filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_STICKER ||
			owner == filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_GIF_ASSET
	default:
		return false
	}
}
func ownerService(id filev1.FileReferenceOwnerType) string {
	switch id {
	case filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_MESSAGE, filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_STICKER, filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_GIF_ASSET:
		return "messaging"
	case filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_STORY, filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_PROFILE_AVATAR:
		return "user"
	case filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_CHAT_AVATAR:
		return "chat"
	case filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_SPACE_AVATAR:
		return "space"
	}
	return ""
}

type referenceIDs struct {
	file, owner        uuid.UUID
	subresource, scope *uuid.UUID
}

func parseReference(ref *filev1.FileReferenceKey) (referenceIDs, error) {
	if ref == nil || ref.GetOwnerType() == filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_UNSPECIFIED {
		return referenceIDs{}, status.Error(codes.InvalidArgument, "invalid reference")
	}
	file, err := parseUUID("file_id", ref.GetFileId())
	if err != nil {
		return referenceIDs{}, err
	}
	owner, err := parseUUID("owner_id", ref.GetOwnerId())
	if err != nil {
		return referenceIDs{}, err
	}
	out := referenceIDs{file: file, owner: owner}
	if ref.SubresourceId != nil {
		id, e := parseUUID("subresource_id", ref.GetSubresourceId())
		if e != nil {
			return out, e
		}
		out.subresource = &id
	}
	if ref.ScopeSpaceId != nil {
		id, e := parseUUID("scope_space_id", ref.GetScopeSpaceId())
		if e != nil {
			return out, e
		}
		out.scope = &id
	}
	return out, nil
}
func lockLiveFence(ctx context.Context, tx pgx.Tx, space uuid.UUID) (string, error) {
	if _, err := tx.Exec(ctx, `INSERT INTO file_space_lifecycle_fences(space_id,generation,state) VALUES($1,0,'LIVE') ON CONFLICT DO NOTHING`, space); err != nil {
		return "", status.Error(codes.Internal, err.Error())
	}
	var state string
	if err := tx.QueryRow(ctx, `SELECT state FROM file_space_lifecycle_fences WHERE space_id=$1 FOR UPDATE`, space).Scan(&state); err != nil {
		return "", status.Error(codes.Internal, err.Error())
	}
	return state, nil
}
func lifecycleRequest(m proto.Message) ([]byte, []byte, error) {
	wire, err := proto.MarshalOptions{Deterministic: true}.Marshal(m)
	if err != nil {
		return nil, nil, err
	}
	return wire, domainHash(m), nil
}
func domainHash(m proto.Message) []byte {
	wire, _ := proto.MarshalOptions{Deterministic: true}.Marshal(m)
	name := string(m.ProtoReflect().Descriptor().FullName())
	sum := sha256.Sum256(append(append([]byte(name), 0), wire...))
	return sum[:]
}
func loadReferenceOperation(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, caller string, op uuid.UUID, hash []byte, out proto.Message) (bool, error) {
	var oldHash, receipt []byte
	err := q.QueryRow(ctx, `SELECT request_sha256,receipt_bytes FROM file_reference_operations WHERE caller_service=$1 AND operation_id=$2`, caller, op).Scan(&oldHash, &receipt)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, status.Error(codes.Internal, err.Error())
	}
	if !bytes.Equal(oldHash, hash) {
		return false, status.Error(codes.AlreadyExists, "operation changed")
	}
	if err := proto.Unmarshal(receipt, out); err != nil {
		return false, status.Error(codes.Internal, err.Error())
	}
	return true, nil
}
func saveReferenceOperation(ctx context.Context, tx pgx.Tx, caller string, op uuid.UUID, request, hash []byte, response proto.Message) error {
	receipt, err := proto.MarshalOptions{Deterministic: true}.Marshal(response)
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}
	receiptHash := sha256.Sum256(receipt)
	_, err = tx.Exec(ctx, `INSERT INTO file_reference_operations(caller_service,operation_id,request_bytes,request_sha256,state,receipt_bytes,receipt_sha256,completed_at) VALUES($1,$2,$3,$4,'COMPLETED',$5,$6,clock_timestamp())`, caller, op, request, hash, receipt, receiptHash[:])
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}
	return nil
}
func validateSortedReferences(refs []*filev1.FileReferenceKey) error {
	for i, r := range refs {
		if _, err := parseReference(r); err != nil {
			return err
		}
		if i > 0 {
			cmp := compareReferences(refs[i-1], r)
			if cmp > 0 {
				return status.Error(codes.InvalidArgument, "references must be sorted")
			}
			if cmp == 0 {
				return status.Error(codes.FailedPrecondition, "duplicate reference")
			}
		}
	}
	return nil
}
func compareReferences(a, b *filev1.FileReferenceKey) int {
	aa, _ := uuid.Parse(a.GetFileId())
	bb, _ := uuid.Parse(b.GetFileId())
	if c := bytes.Compare(aa[:], bb[:]); c != 0 {
		return c
	}
	if a.GetOwnerType() < b.GetOwnerType() {
		return -1
	}
	if a.GetOwnerType() > b.GetOwnerType() {
		return 1
	}
	return strings.Compare(referenceTail(a), referenceTail(b))
}
func referenceTail(r *filev1.FileReferenceKey) string {
	var b []byte
	for _, raw := range []string{r.GetOwnerId(), r.GetSubresourceId(), r.GetScopeSpaceId()} {
		if id, err := uuid.Parse(raw); err == nil {
			b = append(b, id[:]...)
		} else {
			b = append(b, make([]byte, 16)...)
		}
	}
	return string(b)
}
func referenceManifestHash(deletion, space uuid.UUID, generation uint64, producer filev1.FileReferenceProducerId, refs []*filev1.FileReferenceKey) []byte {
	refs = append([]*filev1.FileReferenceKey(nil), refs...)
	sort.Slice(refs, func(i, j int) bool { return compareReferences(refs[i], refs[j]) < 0 })
	h := sha256.New()
	h.Write([]byte("voice.file.v1.SpaceDeletionReferenceProducer"))
	h.Write([]byte{0})
	h.Write(deletion[:])
	h.Write(space[:])
	var g [8]byte
	binary.BigEndian.PutUint64(g[:], generation)
	h.Write(g[:])
	var p [4]byte
	binary.BigEndian.PutUint32(p[:], uint32(producer))
	h.Write(p[:])
	for _, r := range refs {
		wire, _ := proto.MarshalOptions{Deterministic: true}.Marshal(r)
		var l [4]byte
		binary.BigEndian.PutUint32(l[:], uint32(len(wire)))
		h.Write(l[:])
		h.Write(wire)
	}
	return h.Sum(nil)
}
func loadManifestReferences(ctx context.Context, tx pgx.Tx, space, deletion uuid.UUID, generation uint64, producer filev1.FileReferenceProducerId) ([]*filev1.FileReferenceKey, error) {
	rows, err := tx.Query(ctx, `SELECT references_bytes FROM file_space_deletion_manifest_chunks WHERE space_id=$1 AND deletion_operation_id=$2 AND schedule_generation=$3 AND producer_id=$4 ORDER BY chunk_index`, space, deletion, generation, int32(producer))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*filev1.FileReferenceKey
	for rows.Next() {
		var wire []byte
		if err := rows.Scan(&wire); err != nil {
			return nil, err
		}
		var request filev1.RegisterSpaceDeletionReferenceChunkRequest
		if err := proto.Unmarshal(wire, &request); err != nil {
			return nil, err
		}
		out = append(out, request.GetReferences()...)
	}
	return out, rows.Err()
}
func loadDeclarations(ctx context.Context, tx pgx.Tx, space, deletion uuid.UUID, generation uint64) ([]*filev1.FileReferenceProducerDeclaration, error) {
	rows, err := tx.Query(ctx, `SELECT producer_id,expected_total_count,expected_references_sha256,sealed_at FROM file_space_deletion_manifests WHERE space_id=$1 AND deletion_operation_id=$2 AND schedule_generation=$3 ORDER BY producer_id`, space, deletion, generation)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	defer rows.Close()
	var out []*filev1.FileReferenceProducerDeclaration
	for rows.Next() {
		var p int32
		var c uint64
		var h []byte
		var sealed *time.Time
		if err := rows.Scan(&p, &c, &h, &sealed); err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		if sealed == nil {
			return nil, status.Error(codes.FailedPrecondition, "all producers must seal")
		}
		out = append(out, &filev1.FileReferenceProducerDeclaration{ProducerId: filev1.FileReferenceProducerId(p), ExpectedTotalCount: c, ExpectedReferencesSha256: h})
	}
	if len(out) != 3 {
		return nil, status.Error(codes.FailedPrecondition, "three producers required")
	}
	return out, nil
}
func loadSpaceManifestRoot(ctx context.Context, tx pgx.Tx, space, deletion uuid.UUID, generation uint64, chatBytes []byte) (*commonv1.ManifestBinding, error) {
	declarations, err := loadDeclarations(ctx, tx, space, deletion, generation)
	if err != nil {
		return nil, err
	}
	var chat commonv1.ManifestBinding
	if err := proto.Unmarshal(chatBytes, &chat); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return spaceManifestRoot(deletion, space, generation, &chat, declarations), nil
}
func spaceManifestRoot(deletion, space uuid.UUID, generation uint64, chat *commonv1.ManifestBinding, declarations []*filev1.FileReferenceProducerDeclaration) *commonv1.ManifestBinding {
	var wire []byte
	wire = protowire.AppendTag(wire, 1, protowire.VarintType)
	wire = protowire.AppendVarint(wire, 1)
	wire = protowire.AppendTag(wire, 2, protowire.BytesType)
	wire = protowire.AppendString(wire, space.String())
	wire = protowire.AppendTag(wire, 3, protowire.BytesType)
	wire = protowire.AppendString(wire, deletion.String())
	wire = protowire.AppendTag(wire, 4, protowire.VarintType)
	wire = protowire.AppendVarint(wire, generation)
	cw, _ := proto.MarshalOptions{Deterministic: true}.Marshal(chat)
	wire = protowire.AppendTag(wire, 5, protowire.BytesType)
	wire = protowire.AppendBytes(wire, cw)
	count := chat.GetItemCount()
	for _, d := range declarations {
		dw, _ := proto.MarshalOptions{Deterministic: true}.Marshal(d)
		wire = protowire.AppendTag(wire, 6, protowire.BytesType)
		wire = protowire.AppendBytes(wire, dw)
		count += d.GetExpectedTotalCount()
	}
	h := sha256.New()
	h.Write([]byte("voice.space.v1.SpaceDeletionManifestSet"))
	h.Write([]byte{0})
	h.Write(wire)
	return &commonv1.ManifestBinding{ManifestId: deletion.String(), ManifestSha256: h.Sum(nil), ItemCount: count}
}
