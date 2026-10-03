package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	commonv1 "voice.app/voice/common/v1"
	messagingv1 "voice.app/voice/messaging/v1"
	"voice/backend/pkg/spacemutationlock"
)

var (
	ErrSpacePurgeReceiptNotFound = errors.New("messaging purge receipt not found")
	ErrSpacePurgeReceiptBinding  = errors.New("messaging purge receipt binding mismatch")
)

type SpacePurgeReceiptKey struct {
	SpaceID                  uuid.UUID
	DeletionOperationID      uuid.UUID
	PurgeGeneration          uint64
	SourceScheduleGeneration uint64
	MessagingRequestSHA256   []byte
}

// CompleteSpaceLifecyclePurge atomically stores the exact Messaging receipt and
// advances its durable fence to PURGED. Child chat purges and File releases must
// already have completed before this completion boundary is called.
func (s *MessagesStore) CompleteSpaceLifecyclePurge(ctx context.Context, request *messagingv1.PurgeSpaceRequest, receiptID string) ([]byte, error) {
	if s == nil || s.Pool == nil || request == nil || len(request.ProtoReflect().GetUnknown()) != 0 ||
		request.GetPurge() == nil || len(request.GetPurge().ProtoReflect().GetUnknown()) != 0 ||
		len(request.GetPurge().GetManifest().ProtoReflect().GetUnknown()) != 0 || receiptID == "" {
		return nil, errors.New("invalid Messaging Space purge completion")
	}
	purge := request.GetPurge()
	spaceID, err := uuid.Parse(purge.GetSpaceId())
	if err != nil || spaceID == uuid.Nil || spaceID.String() != purge.GetSpaceId() {
		return nil, errors.New("invalid Messaging Space purge space ID")
	}
	operationID, err := uuid.Parse(purge.GetDeletionOperationId())
	if err != nil || operationID == uuid.Nil || operationID.String() != purge.GetDeletionOperationId() {
		return nil, errors.New("invalid Messaging Space purge operation ID")
	}
	receiptUUID, err := uuid.Parse(receiptID)
	if err != nil || receiptUUID == uuid.Nil || receiptUUID.String() != receiptID {
		return nil, errors.New("invalid Messaging Space purge receipt ID")
	}
	if purge.GetProtocolVersion() != 1 || purge.GetGeneration() < 2 ||
		purge.GetParticipantId() != commonv1.ParticipantId_PARTICIPANT_ID_MESSAGING ||
		purge.GetPurgeDecidedAt() == nil || purge.GetPurgeDecidedAt().CheckValid() != nil ||
		purge.GetManifest() == nil || purge.GetManifest().GetManifestId() == "" ||
		len(purge.GetManifest().GetManifestSha256()) != sha256.Size {
		return nil, errors.New("invalid Messaging Space purge binding")
	}
	sourceGeneration := purge.GetGeneration() - 1
	requestBytes, err := proto.MarshalOptions{Deterministic: true}.Marshal(request)
	if err != nil {
		return nil, err
	}
	requestHash := lifecycleRequestDigest(request, requestBytes)
	if len(requestHash) != sha256.Size {
		return nil, errors.New("invalid Messaging Space purge request digest")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1::bigint)`, spacemutationlock.Key(spaceID)); err != nil {
		return nil, err
	}
	var generation, storedSourceGeneration uint64
	var state string
	var storedOperation uuid.UUID
	var manifestID string
	var manifestSHA []byte
	var manifestItems uint64
	err = tx.QueryRow(ctx, `SELECT generation,state,deletion_operation_id,source_schedule_generation,source_manifest_id::text,source_manifest_sha256,source_manifest_item_count
FROM messaging_space_lifecycle_fences WHERE space_id=$1 FOR UPDATE`, spaceID).
		Scan(&generation, &state, &storedOperation, &storedSourceGeneration, &manifestID, &manifestSHA, &manifestItems)
	if err != nil {
		return nil, err
	}
	if generation != purge.GetGeneration() || (state != "PURGE_DECIDED" && state != "PURGED") || storedOperation != operationID ||
		storedSourceGeneration != sourceGeneration || manifestID != purge.GetManifest().GetManifestId() ||
		!bytes.Equal(manifestSHA, purge.GetManifest().GetManifestSha256()) || manifestItems != purge.GetManifest().GetItemCount() {
		return nil, ErrSpacePurgeReceiptBinding
	}
	var savedHash, savedBytes []byte
	err = tx.QueryRow(ctx, `SELECT request_sha256,receipt_bytes FROM messaging_space_purge_receipts
WHERE space_id=$1 AND deletion_operation_id=$2 AND purge_generation=$3`, spaceID, operationID, generation).
		Scan(&savedHash, &savedBytes)
	if err == nil {
		if !bytes.Equal(savedHash, requestHash) {
			return nil, ErrSpacePurgeReceiptBinding
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return savedBytes, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if state == "PURGED" {
		return nil, ErrSpacePurgeReceiptBinding
	}
	// All frozen message payloads must have been deleted by their exact child
	// operations. Clear chat-level projections that have no message FK.
	var remaining bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM messages m JOIN messaging_space_chat_manifest_items i ON i.chat_id=m.chat_id WHERE i.space_id=$1 AND i.deletion_operation_id=$2 AND i.schedule_generation=$3)`, spaceID, operationID, sourceGeneration).Scan(&remaining); err != nil {
		return nil, err
	}
	if remaining {
		return nil, ErrSpacePurgeReceiptBinding
	}
	if _, err = tx.Exec(ctx, `SELECT set_config('voice.messaging_purge_space_id',$1,true),set_config('voice.messaging_purge_operation_id',$2,true)`, spaceID.String(), operationID.String()); err != nil {
		return nil, err
	}
	for _, table := range []string{"read_receipts", "read_positions", "scheduled_messages", "game_message_revisions", "game_message_operation_receipts", "game_message_tombstone_actions"} {
		if _, err = tx.Exec(ctx, `DELETE FROM `+table+` WHERE chat_id IN(SELECT chat_id FROM messaging_space_chat_manifest_items WHERE space_id=$1 AND deletion_operation_id=$2 AND schedule_generation=$3)`, spaceID, operationID, sourceGeneration); err != nil {
			return nil, err
		}
	}
	var completedAt time.Time
	if err = tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&completedAt); err != nil {
		return nil, err
	}
	receipt := &commonv1.SpacePurgeReceipt{
		ProtocolVersion: 1, ReceiptId: receiptID, SpaceId: spaceID.String(), DeletionOperationId: operationID.String(),
		Generation: generation, ParticipantId: commonv1.ParticipantId_PARTICIPANT_ID_MESSAGING,
		State: commonv1.PurgeReceiptState_PURGE_RECEIPT_STATE_COMPLETED, RequestSha256: requestHash,
		CompletedAt: timestamppb.New(completedAt.UTC()),
	}
	responseBytes, err := proto.MarshalOptions{Deterministic: true}.Marshal(&messagingv1.PurgeSpaceResponse{Receipt: receipt})
	if err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO messaging_space_purge_receipts(space_id,deletion_operation_id,purge_generation,source_schedule_generation,request_sha256,receipt_bytes,completed_at)
VALUES($1,$2,$3,$4,$5,$6,$7)`, spaceID, operationID, generation, sourceGeneration, requestHash, responseBytes, completedAt); err != nil {
		return nil, err
	}
	command, err := tx.Exec(ctx, `UPDATE messaging_space_lifecycle_fences
SET state='PURGED',request_bytes=$4,request_sha256=$5,receipt_bytes=$6,updated_at=$7
WHERE space_id=$1 AND deletion_operation_id=$2 AND generation=$3 AND state='PURGE_DECIDED'`,
		spaceID, operationID, generation, requestBytes, requestHash, responseBytes, completedAt)
	if err != nil {
		return nil, err
	}
	if command.RowsAffected() != 1 {
		return nil, fmt.Errorf("%w: Messaging purge fence changed during completion", ErrSpacePurgeReceiptBinding)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return responseBytes, nil
}

func lifecycleRequestDigest(message proto.Message, wire []byte) []byte {
	domain := append([]byte(message.ProtoReflect().Descriptor().FullName()), 0)
	hash := sha256.Sum256(append(domain, wire...))
	return append([]byte(nil), hash[:]...)
}

// GetSpacePurgeReceipt reads a committed completion receipt only. It never
// initiates purge and refuses receipts outside their exact request binding.
func (s *MessagesStore) GetSpacePurgeReceipt(ctx context.Context, key SpacePurgeReceiptKey) (*commonv1.SpacePurgeReceipt, error) {
	if s == nil || s.Pool == nil || key.SpaceID == uuid.Nil || key.DeletionOperationID == uuid.Nil ||
		key.PurgeGeneration == 0 || key.SourceScheduleGeneration+1 != key.PurgeGeneration || len(key.MessagingRequestSHA256) != 32 {
		return nil, errors.New("invalid Messaging purge receipt key")
	}
	var storedHash, receiptBytes []byte
	err := s.Pool.QueryRow(ctx, `
SELECT request_sha256, receipt_bytes
FROM messaging_space_purge_receipts
WHERE space_id=$1 AND deletion_operation_id=$2 AND purge_generation=$3
  AND source_schedule_generation=$4 AND completed_at > clock_timestamp() - interval '30 days'`,
		key.SpaceID, key.DeletionOperationID, key.PurgeGeneration, key.SourceScheduleGeneration).
		Scan(&storedHash, &receiptBytes)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrSpacePurgeReceiptNotFound
	}
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(storedHash, key.MessagingRequestSHA256) {
		return nil, ErrSpacePurgeReceiptBinding
	}
	response := &messagingv1.PurgeSpaceResponse{}
	if err := proto.Unmarshal(receiptBytes, response); err != nil || response.GetReceipt() == nil {
		return nil, errors.New("stored Messaging purge receipt is corrupt")
	}
	return proto.Clone(response.GetReceipt()).(*commonv1.SpacePurgeReceipt), nil
}
