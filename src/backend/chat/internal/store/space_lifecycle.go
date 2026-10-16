package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	chatv1 "voice.app/voice/chat/v1"
	commonv1 "voice.app/voice/common/v1"
	eventsv1 "voice.app/voice/events/v1"
	filev1 "voice.app/voice/file/v1"
	messagingv1 "voice.app/voice/messaging/v1"
	"voice/backend/pkg/spacemutationlock"
)

const chatDeletionManifestPageSize = 1000

var (
	ErrSpaceLifecycleConflict = errors.New("space lifecycle request conflicts with saved Chat evidence")
	ErrSpaceLifecycleState    = errors.New("space lifecycle transition is not allowed")
	ErrSpaceLifecycleRequest  = errors.New("invalid space lifecycle request")
	ErrSpaceLifecycleEvidence = errors.New("space purge owner evidence is invalid")
)

// SpaceLifecycleStore owns the Chat participant's deletion manifest and fence.
type SpaceLifecycleStore struct {
	Pool *pgxpool.Pool
}

// SpacePurgeOwnerEvidence carries immutable owner receipts required before
// Chat deletes its local rows.
type SpacePurgeOwnerEvidence struct {
	MessagingReceipt   *commonv1.SpacePurgeReceipt
	FileReleaseReceipt *filev1.ReleaseSpaceDeletionProducerReferencesReceipt
}

// lockSpaceLifecycleMutation serializes Space-chat creation with manifest
// capture and rejects writes after a lifecycle operation freezes the Space.
func lockSpaceLifecycleMutation(ctx context.Context, tx pgx.Tx, spaceID uuid.UUID) error {
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1::bigint)`, spacemutationlock.Key(spaceID)); err != nil {
		return err
	}
	var state string
	err := tx.QueryRow(ctx, `SELECT state FROM chat_space_lifecycle_fences WHERE space_id=$1`, spaceID).Scan(&state)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if state != "LIVE" {
		return ErrSpaceLifecycleState
	}
	return nil
}

// PrepareSpaceDeletionManifest freezes Space-chat mutations and snapshots the
// exact sorted chat set in the same PostgreSQL transaction.
func (s *SpaceLifecycleStore) PrepareSpaceDeletionManifest(ctx context.Context, req *chatv1.PrepareSpaceDeletionManifestRequest) (*chatv1.PrepareSpaceDeletionManifestResponse, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("chat lifecycle database unavailable")
	}
	spaceID, operationID, requestBytes, requestHash, err := prepareRequestBinding(req)
	if err != nil {
		return nil, err
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1::bigint)`, spacemutationlock.Key(spaceID)); err != nil {
		return nil, err
	}
	var savedRequest, savedResponse []byte
	err = tx.QueryRow(ctx, `SELECT request_bytes,response_bytes FROM chat_space_lifecycle_operations
		WHERE space_id=$1 AND deletion_operation_id=$2 AND operation_kind='PREPARE'`,
		spaceID, operationID).Scan(&savedRequest, &savedResponse)
	if err == nil {
		if !bytes.Equal(savedRequest, requestBytes) {
			return nil, ErrSpaceLifecycleConflict
		}
		response := new(chatv1.PrepareSpaceDeletionManifestResponse)
		if err := proto.Unmarshal(savedResponse, response); err != nil {
			return nil, err
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return response, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	var currentGeneration int64
	var state string
	err = tx.QueryRow(ctx, `SELECT generation,state FROM chat_space_lifecycle_fences WHERE space_id=$1 FOR UPDATE`, spaceID).Scan(&currentGeneration, &state)
	if errors.Is(err, pgx.ErrNoRows) {
		currentGeneration, state = 0, "LIVE"
	} else if err != nil {
		return nil, err
	}
	if state != "LIVE" || req.GetScheduleGeneration() > math.MaxInt64 || int64(req.GetScheduleGeneration()) != currentGeneration+1 {
		return nil, ErrSpaceLifecycleState
	}

	rows, err := tx.Query(ctx, `SELECT id FROM chats WHERE space_id=$1 ORDER BY uuid_send(id)`, spaceID)
	if err != nil {
		return nil, err
	}
	chatIDs := make([]uuid.UUID, 0)
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		chatIDs = append(chatIDs, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	manifestID := uuid.NewSHA1(operationID, []byte(fmt.Sprintf("voice.chat.v1.SpaceDeletionManifest/%s/%d", spaceID, req.GetScheduleGeneration())))
	manifestHash := chatDeletionManifestHash(spaceID, operationID, req.GetScheduleGeneration(), chatIDs)
	manifest := &commonv1.ManifestBinding{ManifestId: manifestID.String(), ManifestSha256: manifestHash[:], ItemCount: uint64(len(chatIDs))}
	var appliedAt time.Time
	if err := tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&appliedAt); err != nil {
		return nil, err
	}
	appliedAt = appliedAt.UTC()
	response := &chatv1.PrepareSpaceDeletionManifestResponse{Receipt: &chatv1.PrepareSpaceDeletionManifestReceipt{
		ProtocolVersion: 1, ReceiptId: uuid.NewString(), SpaceId: spaceID.String(), DeletionOperationId: operationID.String(),
		ScheduleGeneration: req.GetScheduleGeneration(), AppliedState: commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN,
		ChatManifest: manifest, RequestSha256: requestHash[:], AppliedAt: timestamppb.New(appliedAt),
	}}
	responseBytes, err := proto.MarshalOptions{Deterministic: true}.Marshal(response)
	if err != nil {
		return nil, err
	}
	for i, chatID := range chatIDs {
		pageIndex, itemIndex := i/chatDeletionManifestPageSize, i%chatDeletionManifestPageSize
		if _, err := tx.Exec(ctx, `INSERT INTO chat_space_deletion_manifest_items(
			space_id,deletion_operation_id,schedule_generation,page_index,item_index,chat_id)
			VALUES($1,$2,$3,$4,$5,$6)`, spaceID, operationID, req.GetScheduleGeneration(), pageIndex, itemIndex, chatID); err != nil {
			return nil, err
		}
	}
	if _, err := tx.Exec(ctx, `INSERT INTO chat_space_lifecycle_fences(
		space_id,deletion_operation_id,generation,state,schedule_generation,manifest_id,manifest_sha256,manifest_item_count,
		source_manifest_id,source_manifest_sha256,source_manifest_item_count)
		VALUES($1,$2,$3,'FROZEN',$3,$4,$5,$6,$4,$5,$6)
		ON CONFLICT(space_id) DO UPDATE SET deletion_operation_id=EXCLUDED.deletion_operation_id,
		generation=EXCLUDED.generation,state=EXCLUDED.state,schedule_generation=EXCLUDED.schedule_generation,
		manifest_id=EXCLUDED.manifest_id,manifest_sha256=EXCLUDED.manifest_sha256,manifest_item_count=EXCLUDED.manifest_item_count,
		source_manifest_id=EXCLUDED.source_manifest_id,source_manifest_sha256=EXCLUDED.source_manifest_sha256,source_manifest_item_count=EXCLUDED.source_manifest_item_count,
		updated_at=clock_timestamp()
		WHERE chat_space_lifecycle_fences.state='LIVE' AND chat_space_lifecycle_fences.generation=$3-1`,
		spaceID, operationID, req.GetScheduleGeneration(), manifestID, manifestHash[:], len(chatIDs)); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO chat_space_lifecycle_operations(
		space_id,deletion_operation_id,generation,operation_kind,request_bytes,request_sha256,response_bytes,completed_at,retain_until)
		VALUES($1,$2,$3,'PREPARE',$4,$5,$6,$7,$7::timestamptz+interval '30 days')`,
		spaceID, operationID, req.GetScheduleGeneration(), requestBytes, requestHash[:], responseBytes, appliedAt); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return response, nil
}

// ApplySpaceLifecycleFence durably acknowledges the Chat participant fence.
// Fence receipts and their exact requests are retained atomically with the
// generation transition so retries return the original receipt.
func (s *SpaceLifecycleStore) ApplySpaceLifecycleFence(ctx context.Context, req *chatv1.ApplySpaceLifecycleFenceRequest) (*chatv1.ApplySpaceLifecycleFenceResponse, error) {
	if s == nil || s.Pool == nil || req == nil || len(req.ProtoReflect().GetUnknown()) != 0 || req.GetFence() == nil || len(req.GetFence().ProtoReflect().GetUnknown()) != 0 {
		return nil, ErrSpaceLifecycleRequest
	}
	fence := req.GetFence()
	if fence.GetProtocolVersion() != 1 || fence.GetGeneration() == 0 || fence.GetGeneration() > math.MaxInt64 || fence.GetManifest() == nil || len(fence.GetManifest().ProtoReflect().GetUnknown()) != 0 || fence.GetManifest().GetItemCount() > math.MaxInt64 || len(fence.GetManifest().GetManifestSha256()) != sha256.Size {
		return nil, ErrSpaceLifecycleRequest
	}
	spaceID, err := parseLifecycleUUID(fence.GetSpaceId())
	if err != nil {
		return nil, err
	}
	operationID, err := parseLifecycleUUID(fence.GetDeletionOperationId())
	if err != nil {
		return nil, err
	}
	manifestID, err := parseLifecycleUUID(fence.GetManifest().GetManifestId())
	if err != nil {
		return nil, err
	}
	requestBytes, err := proto.MarshalOptions{Deterministic: true}.Marshal(req)
	if err != nil {
		return nil, err
	}
	requestHash := chatLifecycleDigest("voice.chat.v1.ApplySpaceLifecycleFenceRequest", requestBytes)
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1::bigint)`, spacemutationlock.Key(spaceID)); err != nil {
		return nil, err
	}
	var savedRequest, savedResponse []byte
	err = tx.QueryRow(ctx, `SELECT request_bytes,response_bytes FROM chat_space_lifecycle_operations WHERE space_id=$1 AND deletion_operation_id=$2 AND generation=$3 AND operation_kind='FENCE'`, spaceID, operationID, fence.GetGeneration()).Scan(&savedRequest, &savedResponse)
	if err == nil {
		if !bytes.Equal(savedRequest, requestBytes) {
			return nil, ErrSpaceLifecycleConflict
		}
		response := new(chatv1.ApplySpaceLifecycleFenceResponse)
		if err := proto.Unmarshal(savedResponse, response); err != nil {
			return nil, err
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return response, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	var currentOp, sourceManifest uuid.UUID
	var generation, scheduleGeneration, sourceItemCount int64
	var state string
	var sourceManifestHash []byte
	err = tx.QueryRow(ctx, `SELECT deletion_operation_id,generation,state,schedule_generation,source_manifest_id,source_manifest_sha256,source_manifest_item_count FROM chat_space_lifecycle_fences WHERE space_id=$1 FOR UPDATE`, spaceID).Scan(&currentOp, &generation, &state, &scheduleGeneration, &sourceManifest, &sourceManifestHash, &sourceItemCount)
	if err != nil {
		return nil, err
	}
	if currentOp != operationID || sourceManifest == uuid.Nil || len(sourceManifestHash) != sha256.Size || sourceItemCount < 0 || manifestID != sourceManifest || !bytes.Equal(fence.GetManifest().GetManifestSha256(), sourceManifestHash) || fence.GetManifest().GetItemCount() != uint64(sourceItemCount) {
		return nil, ErrSpaceLifecycleConflict
	}
	wantState := ""
	switch fence.GetDesiredState() {
	case commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN:
		wantState = "FROZEN"
		if state != wantState || generation != int64(fence.GetGeneration()) || scheduleGeneration != generation {
			return nil, ErrSpaceLifecycleState
		}
	case commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_LIVE:
		wantState = "LIVE"
		if state != "FROZEN" || int64(fence.GetGeneration()) != generation+1 || scheduleGeneration != generation {
			return nil, ErrSpaceLifecycleState
		}
	case commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED:
		wantState = "PURGE_DECIDED"
		if state != "FROZEN" || int64(fence.GetGeneration()) != generation+1 || scheduleGeneration != generation {
			return nil, ErrSpaceLifecycleState
		}
	default:
		return nil, ErrSpaceLifecycleRequest
	}
	var appliedAt time.Time
	if err := tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&appliedAt); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `UPDATE chat_space_lifecycle_fences SET generation=$2,state=$3,manifest_id=$5,manifest_sha256=$6,manifest_item_count=$7,updated_at=$4 WHERE space_id=$1`, spaceID, fence.GetGeneration(), wantState, appliedAt, manifestID, fence.GetManifest().GetManifestSha256(), fence.GetManifest().GetItemCount()); err != nil {
		return nil, err
	}
	response := &chatv1.ApplySpaceLifecycleFenceResponse{Receipt: &commonv1.SpaceLifecycleFenceReceipt{
		ProtocolVersion: 1, ReceiptId: uuid.NewString(), SpaceId: spaceID.String(), DeletionOperationId: operationID.String(),
		Generation: fence.GetGeneration(), ParticipantId: commonv1.ParticipantId_PARTICIPANT_ID_CHAT,
		AppliedState: fence.GetDesiredState(), RequestSha256: requestHash[:], ManifestSha256: bytes.Clone(fence.GetManifest().GetManifestSha256()), AppliedAt: timestamppb.New(appliedAt.UTC()),
	}}
	responseBytes, err := proto.MarshalOptions{Deterministic: true}.Marshal(response)
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO chat_space_lifecycle_operations(space_id,deletion_operation_id,generation,operation_kind,request_bytes,request_sha256,response_bytes,completed_at,retain_until) VALUES($1,$2,$3,'FENCE',$4,$5,$6,$7,$7::timestamptz+interval '30 days')`, spaceID, operationID, fence.GetGeneration(), requestBytes, requestHash[:], responseBytes, appliedAt); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return response, nil
}

// GetSpacePurgeManifestPage returns an immutable page saved by Chat's freeze.
func (s *SpaceLifecycleStore) GetSpacePurgeManifestPage(ctx context.Context, req *chatv1.GetSpacePurgeManifestPageRequest) (*chatv1.GetSpacePurgeManifestPageResponse, error) {
	if s == nil || s.Pool == nil || req == nil || len(req.ProtoReflect().GetUnknown()) != 0 || req.GetProtocolVersion() != 1 || req.GetGeneration() == 0 {
		return nil, ErrSpaceLifecycleRequest
	}
	spaceID, err := parseLifecycleUUID(req.GetSpaceId())
	if err != nil {
		return nil, err
	}
	operationID, err := parseLifecycleUUID(req.GetDeletionOperationId())
	if err != nil {
		return nil, err
	}
	manifestID, err := parseLifecycleUUID(req.GetManifestId())
	if err != nil {
		return nil, err
	}
	pageIndex := uint64(0)
	var tokenRoot []byte
	if req.GetPageToken() != "" {
		raw, decodeErr := base64.RawURLEncoding.DecodeString(req.GetPageToken())
		if decodeErr != nil || len(raw) != 40 {
			return nil, ErrSpaceLifecycleRequest
		}
		pageIndex = binary.BigEndian.Uint64(raw[:8])
		tokenRoot = raw[8:]
	}
	var state string
	var savedOperation, savedManifest, sourceManifest uuid.UUID
	var generation, scheduleGeneration, itemCount int64
	var rootHash []byte
	err = s.Pool.QueryRow(ctx, `SELECT state,deletion_operation_id,generation,schedule_generation,manifest_id,source_manifest_id,source_manifest_sha256,source_manifest_item_count
		FROM chat_space_lifecycle_fences WHERE space_id=$1`, spaceID).Scan(&state, &savedOperation, &generation, &scheduleGeneration, &savedManifest, &sourceManifest, &rootHash, &itemCount)
	if err != nil {
		return nil, err
	}
	if savedOperation != operationID || savedManifest != manifestID || sourceManifest == uuid.Nil || generation < int64(req.GetGeneration()) || scheduleGeneration != int64(req.GetGeneration()) || state == "PURGED" || (len(tokenRoot) > 0 && !bytes.Equal(tokenRoot, rootHash)) {
		return nil, ErrSpaceLifecycleConflict
	}
	pageCount := uint64(1)
	if itemCount > 0 {
		pageCount = uint64((itemCount + chatDeletionManifestPageSize - 1) / chatDeletionManifestPageSize)
	}
	if pageIndex >= pageCount {
		return nil, ErrSpaceLifecycleRequest
	}
	rows, err := s.Pool.Query(ctx, `SELECT chat_id FROM chat_space_deletion_manifest_items
		WHERE space_id=$1 AND deletion_operation_id=$2 AND schedule_generation=$3 AND page_index=$4 ORDER BY item_index`,
		spaceID, operationID, req.GetGeneration(), pageIndex)
	if err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, 0, chatDeletionManifestPageSize)
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	if (pageIndex+1 < pageCount && len(ids) != chatDeletionManifestPageSize) ||
		(pageIndex+1 == pageCount && len(ids) != int(itemCount)-int(pageIndex)*chatDeletionManifestPageSize) {
		return nil, errors.New("saved Chat manifest page is incomplete")
	}
	page := &chatv1.SpacePurgeManifestPage{ProtocolVersion: 1,
		Manifest:  &commonv1.ManifestBinding{ManifestId: sourceManifest.String(), ManifestSha256: bytes.Clone(rootHash), ItemCount: uint64(itemCount)},
		PageIndex: pageIndex, PageSha256: chatDeletionManifestPageHash(rootHash, pageIndex, ids), ItemIds: make([]string, len(ids))}
	for i, id := range ids {
		page.ItemIds[i] = id.String()
	}
	if pageIndex+1 < pageCount {
		token := make([]byte, 40)
		binary.BigEndian.PutUint64(token[:8], pageIndex+1)
		copy(token[8:], rootHash)
		page.NextPageToken = base64.RawURLEncoding.EncodeToString(token)
	}
	return &chatv1.GetSpacePurgeManifestPageResponse{Page: page}, nil
}

// PurgeSpace removes the exact frozen Chat set only after Messaging's durable
// participant receipt and File's idempotent CHAT producer release are proven.
func (s *SpaceLifecycleStore) PurgeSpace(ctx context.Context, req *chatv1.PurgeSpaceRequest, evidence SpacePurgeOwnerEvidence) (*chatv1.PurgeSpaceResponse, error) {
	if s == nil || s.Pool == nil || req == nil || len(req.ProtoReflect().GetUnknown()) != 0 || req.GetPurge() == nil || len(req.GetPurge().ProtoReflect().GetUnknown()) != 0 {
		return nil, ErrSpaceLifecycleRequest
	}
	purge := req.GetPurge()
	if purge.GetProtocolVersion() != 1 || purge.GetParticipantId() != commonv1.ParticipantId_PARTICIPANT_ID_CHAT || purge.GetGeneration() < 2 || purge.GetGeneration() > math.MaxInt64 || purge.GetManifest() == nil || len(purge.GetManifest().ProtoReflect().GetUnknown()) != 0 || len(purge.GetManifest().GetManifestSha256()) != sha256.Size || purge.GetPurgeDecidedAt() == nil || purge.GetPurgeDecidedAt().CheckValid() != nil {
		return nil, ErrSpaceLifecycleRequest
	}
	spaceID, err := parseLifecycleUUID(purge.GetSpaceId())
	if err != nil {
		return nil, err
	}
	operationID, err := parseLifecycleUUID(purge.GetDeletionOperationId())
	if err != nil {
		return nil, err
	}
	if _, err := parseLifecycleUUID(purge.GetManifest().GetManifestId()); err != nil {
		return nil, ErrSpaceLifecycleConflict
	}
	requestBytes, err := proto.MarshalOptions{Deterministic: true}.Marshal(req)
	if err != nil {
		return nil, err
	}
	requestHash := chatLifecycleDigest("voice.chat.v1.PurgeSpaceRequest", requestBytes)
	messagingReceiptBytes, fileReceiptBytes, err := marshalSpacePurgeOwnerEvidence(purge, evidence)
	if err != nil {
		return nil, err
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1::bigint)`, spacemutationlock.Key(spaceID)); err != nil {
		return nil, err
	}
	var savedRequest, savedResponse, savedMessagingReceipt, savedFileReceipt []byte
	err = tx.QueryRow(ctx, `SELECT request_bytes,response_bytes,messaging_receipt_bytes,file_release_receipt_bytes FROM chat_space_lifecycle_operations WHERE space_id=$1 AND deletion_operation_id=$2 AND generation=$3 AND operation_kind='PURGE'`, spaceID, operationID, purge.GetGeneration()).Scan(&savedRequest, &savedResponse, &savedMessagingReceipt, &savedFileReceipt)
	if err == nil {
		if !bytes.Equal(savedRequest, requestBytes) || !bytes.Equal(savedMessagingReceipt, messagingReceiptBytes) || !bytes.Equal(savedFileReceipt, fileReceiptBytes) {
			return nil, ErrSpaceLifecycleConflict
		}
		response := new(chatv1.PurgeSpaceResponse)
		if err := proto.Unmarshal(savedResponse, response); err != nil {
			return nil, err
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return response, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	var state string
	var deletionID, manifestID, sourceManifestID uuid.UUID
	var generation, scheduleGeneration, manifestCount, sourceCount int64
	var manifestHash, sourceHash []byte
	err = tx.QueryRow(ctx, `SELECT state,deletion_operation_id,generation,schedule_generation,manifest_id,manifest_sha256,manifest_item_count,source_manifest_id,source_manifest_sha256,source_manifest_item_count FROM chat_space_lifecycle_fences WHERE space_id=$1 FOR UPDATE`, spaceID).Scan(&state, &deletionID, &generation, &scheduleGeneration, &manifestID, &manifestHash, &manifestCount, &sourceManifestID, &sourceHash, &sourceCount)
	if err != nil {
		return nil, err
	}
	if state != "PURGE_DECIDED" || deletionID != operationID || generation != int64(purge.GetGeneration()) || scheduleGeneration != int64(purge.GetGeneration()-1) || manifestID.String() != purge.GetManifest().GetManifestId() || manifestID != sourceManifestID || !bytes.Equal(manifestHash, purge.GetManifest().GetManifestSha256()) || !bytes.Equal(manifestHash, sourceHash) || manifestCount != int64(purge.GetManifest().GetItemCount()) || manifestCount != sourceCount || sourceManifestID == uuid.Nil || sourceCount < 0 {
		return nil, ErrSpaceLifecycleState
	}
	ids := make([]uuid.UUID, 0, sourceCount)
	rows, err := tx.Query(ctx, `SELECT chat_id FROM chat_space_deletion_manifest_items WHERE space_id=$1 AND deletion_operation_id=$2 AND schedule_generation=$3 ORDER BY chat_id`, spaceID, operationID, purge.GetGeneration()-1)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	sourceDigest := chatDeletionManifestHash(spaceID, operationID, purge.GetGeneration()-1, ids)
	if int64(len(ids)) != sourceCount || !bytes.Equal(sourceDigest[:], sourceHash) {
		return nil, ErrSpaceLifecycleConflict
	}
	if _, err := tx.Exec(ctx, `DELETE FROM chat_members WHERE chat_id IN (SELECT chat_id FROM chat_space_deletion_manifest_items WHERE space_id=$1 AND deletion_operation_id=$2 AND schedule_generation=$3)`, spaceID, operationID, purge.GetGeneration()-1); err != nil {
		return nil, err
	}
	deleted, err := tx.Exec(ctx, `DELETE FROM chats WHERE space_id=$1 AND id IN (SELECT chat_id FROM chat_space_deletion_manifest_items WHERE space_id=$1 AND deletion_operation_id=$2 AND schedule_generation=$3)`, spaceID, operationID, purge.GetGeneration()-1)
	if err != nil {
		return nil, err
	}
	if deleted.RowsAffected() != sourceCount {
		return nil, ErrSpaceLifecycleConflict
	}
	var completedAt time.Time
	if err := tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&completedAt); err != nil {
		return nil, err
	}
	completedAt = completedAt.UTC()
	for _, chatID := range ids {
		eventID := uuid.NewSHA1(operationID, []byte(fmt.Sprintf("voice.chat.events.v1.ChatDeleted/%s/%d/%s", spaceID, purge.GetGeneration(), chatID)))
		envelope := &eventsv1.ChatStreamEvent{
			EventId:    eventID.String(),
			OccurredAt: timestamppb.New(completedAt),
			Payload: &eventsv1.ChatStreamEvent_ChatDeleted{ChatDeleted: &eventsv1.ChatDeleted{
				ChatId: chatID.String(), SpaceId: spaceID.String(), DeletionOperationId: operationID.String(),
				Generation: purge.GetGeneration(), ManifestId: sourceManifestID.String(), ManifestSha256: append([]byte(nil), sourceHash...),
			}},
		}
		eventBytes, err := proto.MarshalOptions{Deterministic: true}.Marshal(envelope)
		if err != nil {
			return nil, err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO chat_deleted_event_outbox(
			event_id,chat_id,space_id,deletion_operation_id,generation,manifest_id,manifest_sha256,event_bytes,occurred_at)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, eventID, chatID, spaceID, operationID, purge.GetGeneration(), sourceManifestID, sourceHash, eventBytes, completedAt); err != nil {
			return nil, err
		}
	}
	response := &chatv1.PurgeSpaceResponse{Receipt: &commonv1.SpacePurgeReceipt{ProtocolVersion: 1, ReceiptId: uuid.NewString(), SpaceId: spaceID.String(), DeletionOperationId: operationID.String(), Generation: purge.GetGeneration(), ParticipantId: commonv1.ParticipantId_PARTICIPANT_ID_CHAT, State: commonv1.PurgeReceiptState_PURGE_RECEIPT_STATE_COMPLETED, RequestSha256: requestHash[:], CompletedAt: timestamppb.New(completedAt)}}
	responseBytes, err := proto.MarshalOptions{Deterministic: true}.Marshal(response)
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `UPDATE chat_space_lifecycle_fences SET state='PURGED',updated_at=$2 WHERE space_id=$1`, spaceID, completedAt); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO chat_space_lifecycle_operations(space_id,deletion_operation_id,generation,operation_kind,request_bytes,request_sha256,response_bytes,completed_at,retain_until,messaging_receipt_bytes,file_release_receipt_bytes) VALUES($1,$2,$3,'PURGE',$4,$5,$6,$7,$7::timestamptz+interval '30 days',$8,$9)`, spaceID, operationID, purge.GetGeneration(), requestBytes, requestHash[:], responseBytes, completedAt, messagingReceiptBytes, fileReceiptBytes); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return response, nil
}

func marshalSpacePurgeOwnerEvidence(purge *commonv1.SpacePurgeRequest, evidence SpacePurgeOwnerEvidence) ([]byte, []byte, error) {
	if err := validateSpacePurgeOwnerEvidence(purge, evidence); err != nil {
		return nil, nil, err
	}
	messagingReceipt, err := proto.MarshalOptions{Deterministic: true}.Marshal(evidence.MessagingReceipt)
	if err != nil {
		return nil, nil, err
	}
	fileReceipt, err := proto.MarshalOptions{Deterministic: true}.Marshal(evidence.FileReleaseReceipt)
	if err != nil {
		return nil, nil, err
	}
	return messagingReceipt, fileReceipt, nil
}

// ValidateSpacePurgeRequest checks Chat's durable decision and exact local
// manifest before the coordinator makes either owner call. PurgeSpace repeats
// these checks in its destructive transaction after the remote calls return.
func (s *SpaceLifecycleStore) ValidateSpacePurgeRequest(ctx context.Context, req *chatv1.PurgeSpaceRequest) error {
	if s == nil || s.Pool == nil || req == nil || len(req.ProtoReflect().GetUnknown()) != 0 || req.GetPurge() == nil {
		return ErrSpaceLifecycleRequest
	}
	purge := req.GetPurge()
	if len(purge.ProtoReflect().GetUnknown()) != 0 || purge.GetProtocolVersion() != 1 || purge.GetGeneration() < 2 || purge.GetGeneration() > math.MaxInt64 || purge.GetParticipantId() != commonv1.ParticipantId_PARTICIPANT_ID_CHAT || purge.GetPurgeDecidedAt() == nil || purge.GetPurgeDecidedAt().CheckValid() != nil || purge.GetManifest() == nil || len(purge.GetManifest().ProtoReflect().GetUnknown()) != 0 || len(purge.GetManifest().GetManifestSha256()) != sha256.Size || purge.GetManifest().GetItemCount() > math.MaxInt64 {
		return ErrSpaceLifecycleRequest
	}
	spaceID, err := parseLifecycleUUID(purge.GetSpaceId())
	if err != nil {
		return err
	}
	operationID, err := parseLifecycleUUID(purge.GetDeletionOperationId())
	if err != nil {
		return ErrSpaceLifecycleRequest
	}
	if _, err := parseLifecycleUUID(purge.GetManifest().GetManifestId()); err != nil {
		return ErrSpaceLifecycleRequest
	}
	requestBytes, err := proto.MarshalOptions{Deterministic: true}.Marshal(req)
	if err != nil {
		return err
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1::bigint)`, spacemutationlock.Key(spaceID)); err != nil {
		return err
	}
	var savedRequest []byte
	err = tx.QueryRow(ctx, `SELECT request_bytes FROM chat_space_lifecycle_operations WHERE space_id=$1 AND deletion_operation_id=$2 AND generation=$3 AND operation_kind='PURGE'`, spaceID, operationID, purge.GetGeneration()).Scan(&savedRequest)
	if err == nil {
		if !bytes.Equal(savedRequest, requestBytes) {
			return ErrSpaceLifecycleConflict
		}
		return tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	var state string
	var fenceOperation, aggregateManifest, sourceManifest uuid.UUID
	var generation, sourceSchedule, aggregateCount, sourceCount int64
	var aggregateHash, sourceHash []byte
	err = tx.QueryRow(ctx, `SELECT state,deletion_operation_id,generation,schedule_generation,manifest_id,manifest_sha256,manifest_item_count,source_manifest_id,source_manifest_sha256,source_manifest_item_count FROM chat_space_lifecycle_fences WHERE space_id=$1 FOR UPDATE`, spaceID).Scan(&state, &fenceOperation, &generation, &sourceSchedule, &aggregateManifest, &aggregateHash, &aggregateCount, &sourceManifest, &sourceHash, &sourceCount)
	if err != nil {
		return err
	}
	if state != "PURGE_DECIDED" || fenceOperation != operationID || generation != int64(purge.GetGeneration()) || sourceSchedule != int64(purge.GetGeneration()-1) || aggregateManifest.String() != purge.GetManifest().GetManifestId() || aggregateManifest != sourceManifest || !bytes.Equal(aggregateHash, purge.GetManifest().GetManifestSha256()) || !bytes.Equal(aggregateHash, sourceHash) || aggregateCount != int64(purge.GetManifest().GetItemCount()) || aggregateCount != sourceCount || sourceManifest == uuid.Nil || sourceCount < 0 {
		return ErrSpaceLifecycleState
	}
	ids := make([]uuid.UUID, 0, sourceCount)
	rows, err := tx.Query(ctx, `SELECT chat_id FROM chat_space_deletion_manifest_items WHERE space_id=$1 AND deletion_operation_id=$2 AND schedule_generation=$3 ORDER BY chat_id`, spaceID, operationID, purge.GetGeneration()-1)
	if err != nil {
		return err
	}
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	manifestHash := chatDeletionManifestHash(spaceID, operationID, purge.GetGeneration()-1, ids)
	if int64(len(ids)) != sourceCount || !bytes.Equal(manifestHash[:], sourceHash) {
		return ErrSpaceLifecycleConflict
	}
	var currentChatCount, savedChatCount int64
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM chats WHERE space_id=$1`, spaceID).Scan(&currentChatCount); err != nil {
		return err
	}
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM chat_space_deletion_manifest_items m JOIN chats c ON c.id=m.chat_id AND c.space_id=m.space_id WHERE m.space_id=$1 AND m.deletion_operation_id=$2 AND m.schedule_generation=$3`, spaceID, operationID, purge.GetGeneration()-1).Scan(&savedChatCount); err != nil {
		return err
	}
	if currentChatCount != sourceCount || savedChatCount != sourceCount {
		return ErrSpaceLifecycleConflict
	}
	return tx.Commit(ctx)
}

func validateSpacePurgeOwnerEvidence(purge *commonv1.SpacePurgeRequest, evidence SpacePurgeOwnerEvidence) error {
	if evidence.MessagingReceipt == nil || evidence.FileReleaseReceipt == nil {
		return ErrSpaceLifecycleEvidence
	}
	msgPurge := proto.Clone(purge).(*commonv1.SpacePurgeRequest)
	msgPurge.ParticipantId = commonv1.ParticipantId_PARTICIPANT_ID_MESSAGING
	msgRequest := &messagingv1.PurgeSpaceRequest{Purge: msgPurge}
	msgWire, err := proto.MarshalOptions{Deterministic: true}.Marshal(msgRequest)
	if err != nil {
		return err
	}
	msgHash := chatLifecycleDigest("voice.messaging.v1.PurgeSpaceRequest", msgWire)
	m := evidence.MessagingReceipt
	if m.GetProtocolVersion() != 1 || m.GetSpaceId() != purge.GetSpaceId() || m.GetDeletionOperationId() != purge.GetDeletionOperationId() || m.GetGeneration() != purge.GetGeneration() || m.GetParticipantId() != commonv1.ParticipantId_PARTICIPANT_ID_MESSAGING || m.GetState() != commonv1.PurgeReceiptState_PURGE_RECEIPT_STATE_COMPLETED || !bytes.Equal(m.GetRequestSha256(), msgHash[:]) || m.GetCompletedAt() == nil || m.GetCompletedAt().CheckValid() != nil {
		return ErrSpaceLifecycleEvidence
	}
	fileHash := emptyChatProducerReferenceHash(purge.GetDeletionOperationId(), purge.GetSpaceId(), purge.GetGeneration()-1)
	f := evidence.FileReleaseReceipt
	if f.GetProtocolVersion() != 1 || f.GetSpaceId() != purge.GetSpaceId() || f.GetDeletionOperationId() != purge.GetDeletionOperationId() || f.GetPurgeGeneration() != purge.GetGeneration() || f.GetSourceScheduleGeneration() != purge.GetGeneration()-1 || f.GetProducerId() != filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_CHAT || f.GetReleasedCount() != 0 || !bytes.Equal(f.GetExpectedReferencesSha256(), fileHash[:]) || f.GetCompletedAt() == nil || f.GetCompletedAt().CheckValid() != nil {
		return ErrSpaceLifecycleEvidence
	}
	releaseReq := &filev1.ReleaseSpaceDeletionProducerReferencesRequest{ProtocolVersion: 1, DeletionOperationId: purge.GetDeletionOperationId(), SpaceId: purge.GetSpaceId(), PurgeGeneration: purge.GetGeneration(), SourceScheduleGeneration: purge.GetGeneration() - 1, ProducerId: filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_CHAT, ExpectedReferencesSha256: fileHash[:]}
	wire, err := proto.MarshalOptions{Deterministic: true}.Marshal(releaseReq)
	if err != nil {
		return err
	}
	requestHash := chatLifecycleDigest("voice.file.v1.ReleaseSpaceDeletionProducerReferencesRequest", wire)
	if !bytes.Equal(f.GetRequestSha256(), requestHash[:]) {
		return ErrSpaceLifecycleEvidence
	}
	return nil
}

func emptyChatProducerReferenceHash(deletionID, spaceID string, generation uint64) [sha256.Size]byte {
	deletion, _ := uuid.Parse(deletionID)
	space, _ := uuid.Parse(spaceID)
	h := sha256.New()
	_, _ = h.Write([]byte("voice.file.v1.SpaceDeletionReferenceProducer\x00"))
	_, _ = h.Write(deletion[:])
	_, _ = h.Write(space[:])
	var g [8]byte
	binary.BigEndian.PutUint64(g[:], generation)
	_, _ = h.Write(g[:])
	var p [4]byte
	binary.BigEndian.PutUint32(p[:], uint32(filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_CHAT))
	_, _ = h.Write(p[:])
	var out [sha256.Size]byte
	copy(out[:], h.Sum(nil))
	return out
}

func prepareRequestBinding(req *chatv1.PrepareSpaceDeletionManifestRequest) (uuid.UUID, uuid.UUID, []byte, [sha256.Size]byte, error) {
	if req == nil || len(req.ProtoReflect().GetUnknown()) != 0 || req.GetProtocolVersion() != 1 || req.GetScheduleGeneration() == 0 || req.GetScheduleGeneration() > math.MaxInt64 {
		return uuid.Nil, uuid.Nil, nil, [sha256.Size]byte{}, ErrSpaceLifecycleRequest
	}
	spaceID, err := parseLifecycleUUID(req.GetSpaceId())
	if err != nil {
		return uuid.Nil, uuid.Nil, nil, [sha256.Size]byte{}, err
	}
	operationID, err := parseLifecycleUUID(req.GetDeletionOperationId())
	if err != nil {
		return uuid.Nil, uuid.Nil, nil, [sha256.Size]byte{}, err
	}
	requestBytes, err := proto.MarshalOptions{Deterministic: true}.Marshal(req)
	if err != nil {
		return uuid.Nil, uuid.Nil, nil, [sha256.Size]byte{}, err
	}
	return spaceID, operationID, requestBytes, chatLifecycleDigest("voice.chat.v1.PrepareSpaceDeletionManifestRequest", requestBytes), nil
}

func parseLifecycleUUID(raw string) (uuid.UUID, error) {
	id, err := uuid.Parse(raw)
	if err != nil || id == uuid.Nil || id.String() != raw {
		return uuid.Nil, ErrSpaceLifecycleRequest
	}
	return id, nil
}

func chatDeletionManifestHash(spaceID, operationID uuid.UUID, generation uint64, ids []uuid.UUID) [sha256.Size]byte {
	h := sha256.New()
	_, _ = h.Write([]byte("voice.chat.v1.SpaceDeletionManifest\x00"))
	_, _ = h.Write(spaceID[:])
	_, _ = h.Write(operationID[:])
	var integer [8]byte
	binary.BigEndian.PutUint64(integer[:], generation)
	_, _ = h.Write(integer[:])
	binary.BigEndian.PutUint64(integer[:], uint64(len(ids)))
	_, _ = h.Write(integer[:])
	for _, id := range ids {
		_, _ = h.Write(id[:])
	}
	var digest [sha256.Size]byte
	copy(digest[:], h.Sum(nil))
	return digest
}

func chatDeletionManifestPageHash(root []byte, pageIndex uint64, ids []uuid.UUID) []byte {
	h := sha256.New()
	_, _ = h.Write([]byte("voice.chat.v1.SpaceDeletionManifestPage\x00"))
	_, _ = h.Write(root)
	var integer [8]byte
	binary.BigEndian.PutUint64(integer[:], pageIndex)
	_, _ = h.Write(integer[:])
	binary.BigEndian.PutUint64(integer[:], uint64(len(ids)))
	_, _ = h.Write(integer[:])
	for _, id := range ids {
		_, _ = h.Write(id[:])
	}
	return h.Sum(nil)
}

func chatLifecycleDigest(fqn string, request []byte) [sha256.Size]byte {
	h := sha256.New()
	_, _ = h.Write([]byte(fqn))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write(request)
	var digest [sha256.Size]byte
	copy(digest[:], h.Sum(nil))
	return digest
}
