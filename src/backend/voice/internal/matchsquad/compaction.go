package matchsquad

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	callsv1 "voice.app/voice/calls/v1"
	"voice/backend/voice/internal/matchsquadprincipal"
)

const matchSquadRetention = 30 * 24 * time.Hour

func (s *Service) Compact(ctx context.Context, req *callsv1.CompactMatchSquadRoomRequest) ([]byte, error) {
	if err := matchsquadprincipal.RequireCompact(ctx, req); err != nil {
		return nil, err
	}
	ids, completedAt, authorizedAt, requestBytes, err := validateCompaction(req)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid MatchSquad compaction binding")
	}
	operation, aggregate, match, room, creation, teardown, teardownReceipt := ids[0], ids[1], ids[2], ids[3], ids[4], ids[5], ids[6]
	requestHash, err := principalHash(req)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid MatchSquad compaction request hash")
	}
	if s == nil || s.Pool == nil {
		return nil, status.Error(codes.Unavailable, "Voice MatchSquad provider unavailable")
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, status.Error(codes.Unavailable, "Voice MatchSquad database unavailable")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, operation.String()); err != nil {
		return nil, status.Error(codes.Unavailable, "Voice MatchSquad database unavailable")
	}
	var priorRequest, priorReceipt []byte
	err = tx.QueryRow(ctx, `SELECT compaction_request_bytes,compaction_receipt_bytes FROM voice_match_squad_operations WHERE compaction_operation_id=$1 FOR UPDATE`, operation).Scan(&priorRequest, &priorReceipt)
	if err == nil {
		if !bytes.Equal(priorRequest, requestBytes) || len(priorReceipt) == 0 {
			return nil, status.Error(codes.AlreadyExists, "MatchSquad compaction operation conflicts")
		}
		if err = tx.Commit(ctx); err != nil {
			return nil, status.Error(codes.Unavailable, "Voice MatchSquad database unavailable")
		}
		return priorReceipt, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, status.Error(codes.Unavailable, "Voice MatchSquad database unavailable")
	}

	var storedOperation, storedMatch, storedRoom, owner, resourceOwner, storedCreation, storedChatReceipt uuid.UUID
	var storedChat uuid.UUID
	var createHash, manifest, chatReceiptHash, createRequest, createReceipt []byte
	var teardownHash, teardownRequest, teardownReceiptBytes []byte
	var storedTeardown, storedTeardownReceipt *uuid.UUID
	var state, resourceState, purpose, roomType string
	var effectsConfirmed, resourceClosedAt *time.Time
	var priorCompact *uuid.UUID
	err = tx.QueryRow(ctx, `SELECT o.operation_id,o.match_id,o.room_id,o.chat_id,o.owner_id,o.creation_receipt_id,
o.chat_creation_receipt_id,o.create_request_sha256,o.participant_manifest_sha256,o.chat_creation_receipt_sha256,
o.create_request_bytes,o.create_receipt_bytes,o.teardown_operation_id,o.teardown_request_sha256,
o.teardown_request_bytes,o.teardown_receipt_id,o.teardown_receipt_bytes,o.state,o.effects_confirmed_at,
o.compaction_operation_id,r.owner_id,r.state,r.purpose,r.room_type,r.closed_at
FROM voice_match_squad_operations o JOIN voice_room_instances r USING(room_id)
WHERE o.match_id=$1 AND o.room_id=$2 FOR UPDATE OF o,r`, match, room).Scan(
		&storedOperation, &storedMatch, &storedRoom, &storedChat, &owner, &storedCreation, &storedChatReceipt,
		&createHash, &manifest, &chatReceiptHash, &createRequest, &createReceipt, &storedTeardown, &teardownHash,
		&teardownRequest, &storedTeardownReceipt, &teardownReceiptBytes, &state, &effectsConfirmed, &priorCompact,
		&resourceOwner, &resourceState, &purpose, &roomType, &resourceClosedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, status.Error(codes.NotFound, "MatchSquad resource not found")
	}
	if err != nil {
		return nil, status.Error(codes.Unavailable, "Voice MatchSquad database unavailable")
	}
	if priorCompact != nil {
		return nil, status.Error(codes.AlreadyExists, "MatchSquad resource already has a different compaction fence")
	}
	if storedMatch != match || storedRoom != room || owner != match || resourceOwner != match || storedCreation != creation || state != "closed" || resourceState != "closed" || purpose != "MATCH_SQUAD" || roomType != "group_voice" || effectsConfirmed == nil || resourceClosedAt == nil ||
		storedTeardown == nil || *storedTeardown != teardown || storedTeardownReceipt == nil || *storedTeardownReceipt != teardownReceipt ||
		len(createHash) != sha256.Size || len(manifest) != sha256.Size || len(chatReceiptHash) != sha256.Size || len(teardownHash) != sha256.Size ||
		len(createRequest) == 0 || len(createReceipt) == 0 || len(teardownRequest) == 0 || len(teardownReceiptBytes) == 0 {
		return nil, status.Error(codes.FailedPrecondition, "MatchSquad resource is not eligible for compaction")
	}
	if !bytes.Equal(req.GetCreationRequestSha256(), createHash) || !bytes.Equal(req.GetParticipantManifestSha256(), manifest) {
		return nil, status.Error(codes.FailedPrecondition, "MatchSquad creation binding mismatch")
	}
	if !validateStoredCreate(storedOperation, match, room, storedChat.String(), storedCreation, storedChatReceipt, chatReceiptHash, createHash, manifest, createRequest, createReceipt) {
		return nil, status.Error(codes.FailedPrecondition, "MatchSquad creation evidence is invalid")
	}
	if !validateStoredTeardown(teardown, match, room, creation, teardownReceipt, teardownHash, manifest, createHash, teardownRequest, teardownReceiptBytes) {
		return nil, status.Error(codes.FailedPrecondition, "MatchSquad teardown evidence is invalid")
	}
	teardownDigest := sha256.Sum256(teardownReceiptBytes)
	if !bytes.Equal(req.GetTeardownReceiptSha256(), teardownDigest[:]) {
		return nil, status.Error(codes.FailedPrecondition, "MatchSquad teardown receipt digest mismatch")
	}

	receiptID, err := uuid.NewRandom()
	if err != nil {
		return nil, status.Error(codes.Unavailable, "Voice MatchSquad compaction receipt unavailable")
	}
	receipt := &callsv1.MatchSquadRoomCompactionReceipt{
		ProtocolVersion: 1, ReceiptId: receiptID.String(), CompactionOperationId: operation.String(),
		TeardownAggregateId: aggregate.String(), MatchId: match.String(), RoomId: room.String(), RequestSha256: requestHash[:],
		AggregateCompletedAt: timestamppb.New(completedAt), CompactionAuthorizedAt: timestamppb.New(authorizedAt),
		Status: callsv1.MatchSquadRoomCompactionStatus_MATCH_SQUAD_ROOM_COMPACTION_STATUS_COMPACTED,
	}
	receiptBytes, err := marshal(receipt)
	if err != nil {
		return nil, status.Error(codes.Internal, "Voice MatchSquad compaction receipt encoding failed")
	}
	command, err := tx.Exec(ctx, `UPDATE voice_match_squad_operations
SET compaction_operation_id=$2,teardown_aggregate_id=$3,compaction_request_sha256=$4,compaction_request_bytes=$5,
compaction_receipt_id=$6,compaction_receipt_bytes=$7,teardown_receipt_sha256=$8,
aggregate_completed_at=$9,compaction_authorized_at=$10,
create_request_bytes=NULL,create_receipt_bytes=NULL,teardown_request_bytes=NULL,teardown_receipt_bytes=NULL
WHERE match_id=$1 AND room_id=$11 AND state='closed' AND effects_confirmed_at IS NOT NULL AND compaction_operation_id IS NULL`,
		match, operation, aggregate, requestHash[:], requestBytes, receiptID, receiptBytes, teardownDigest[:], completedAt, authorizedAt, room)
	if err != nil || command.RowsAffected() != 1 {
		return nil, status.Error(codes.FailedPrecondition, "MatchSquad compaction fence could not be committed")
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, status.Error(codes.Unavailable, "Voice MatchSquad compaction commit pending")
	}
	return receiptBytes, nil
}

func validateCompaction(req *callsv1.CompactMatchSquadRoomRequest) ([]uuid.UUID, time.Time, time.Time, []byte, error) {
	if req == nil || req.GetProtocolVersion() != 1 || len(req.GetCreationRequestSha256()) != sha256.Size ||
		len(req.GetTeardownReceiptSha256()) != sha256.Size || len(req.GetParticipantManifestSha256()) != sha256.Size ||
		req.GetAggregateCompletedAt() == nil || req.GetCompactionAuthorizedAt() == nil ||
		req.GetAggregateCompletedAt().CheckValid() != nil || req.GetCompactionAuthorizedAt().CheckValid() != nil {
		return nil, time.Time{}, time.Time{}, nil, errors.New("invalid compaction request shape")
	}
	ids, err := parseUUIDs(req.GetOperationId(), req.GetTeardownAggregateId(), req.GetMatchId(), req.GetRoomId(), req.GetCreationReceiptId(), req.GetTeardownOperationId(), req.GetTeardownReceiptId())
	if err != nil {
		return nil, time.Time{}, time.Time{}, nil, err
	}
	completed, authorized := req.GetAggregateCompletedAt().AsTime().UTC(), req.GetCompactionAuthorizedAt().AsTime().UTC()
	if authorized.Before(completed.Add(matchSquadRetention)) {
		return nil, time.Time{}, time.Time{}, nil, errors.New("aggregate retention interval not reached")
	}
	requestBytes, err := marshal(req)
	return ids, completed, authorized, requestBytes, err
}

func validateStoredCreate(operation, match, room uuid.UUID, chat string, creation, chatReceipt uuid.UUID, chatReceiptHash, requestHash, manifest, requestBytes, receiptBytes []byte) bool {
	request := new(callsv1.CreateMatchSquadRoomRequest)
	if proto.Unmarshal(requestBytes, request) != nil || len(request.ProtoReflect().GetUnknown()) != 0 {
		return false
	}
	canonical, err := marshal(request)
	if err != nil || !bytes.Equal(canonical, requestBytes) {
		return false
	}
	op, id, _, digest, chatValue, _, err := validateCreate(request)
	if err != nil || op != operation || id != match || !bytes.Equal(digest[:], manifest) || chatValue.GetChatId() != chat || chatValue.GetReceiptId() != chatReceipt.String() {
		return false
	}
	requestDigest, err := principalHash(request)
	if err != nil || !bytes.Equal(requestDigest[:], requestHash) {
		return false
	}
	chatBytes, err := marshal(chatValue)
	if err != nil {
		return false
	}
	chatDigest := sha256.Sum256(chatBytes)
	if !bytes.Equal(chatDigest[:], chatReceiptHash) {
		return false
	}
	receipt := new(callsv1.MatchSquadRoomReceipt)
	if proto.Unmarshal(receiptBytes, receipt) != nil || len(receipt.ProtoReflect().GetUnknown()) != 0 {
		return false
	}
	canonicalReceipt, err := marshal(receipt)
	if err != nil || !bytes.Equal(canonicalReceipt, receiptBytes) || receipt.GetCreatedAt() == nil || receipt.GetCreatedAt().CheckValid() != nil {
		return false
	}
	return receipt.GetProtocolVersion() == 1 && receipt.GetReceiptId() == creation.String() && receipt.GetOperationId() == operation.String() &&
		receipt.GetMatchId() == match.String() && receipt.GetRoomId() == room.String() && receipt.GetChatId() == chat &&
		receipt.GetChatCreationReceiptId() == chatReceipt.String() && bytes.Equal(receipt.GetChatCreationReceiptSha256(), chatReceiptHash) &&
		bytes.Equal(receipt.GetParticipantManifestSha256(), manifest) && bytes.Equal(receipt.GetRequestSha256(), requestHash)
}

func validateStoredTeardown(operation, match, room, creation, receiptID uuid.UUID, requestHash, manifest, createHash, requestBytes, receiptBytes []byte) bool {
	request := new(callsv1.TeardownMatchSquadRoomRequest)
	if proto.Unmarshal(requestBytes, request) != nil || len(request.ProtoReflect().GetUnknown()) != 0 {
		return false
	}
	canonical, err := marshal(request)
	if err != nil || !bytes.Equal(canonical, requestBytes) {
		return false
	}
	requestDigest, err := principalHash(request)
	if err != nil || !bytes.Equal(requestDigest[:], requestHash) || request.GetProtocolVersion() != 1 ||
		request.GetTeardownOperationId() != operation.String() || request.GetMatchId() != match.String() || request.GetRoomId() != room.String() ||
		request.GetCreationReceiptId() != creation.String() || !bytes.Equal(request.GetParticipantManifestSha256(), manifest) ||
		!bytes.Equal(request.GetCreationRequestSha256(), createHash) {
		return false
	}
	receipt := new(callsv1.MatchSquadRoomTeardownReceipt)
	if proto.Unmarshal(receiptBytes, receipt) != nil || len(receipt.ProtoReflect().GetUnknown()) != 0 || receipt.GetCompletedAt() == nil || receipt.GetCompletedAt().CheckValid() != nil {
		return false
	}
	canonicalReceipt, err := marshal(receipt)
	if err != nil || !bytes.Equal(canonicalReceipt, receiptBytes) {
		return false
	}
	return receipt.GetProtocolVersion() == 1 && receipt.GetReceiptId() == receiptID.String() && receipt.GetTeardownOperationId() == operation.String() &&
		receipt.GetMatchId() == match.String() && receipt.GetRoomId() == room.String() && receipt.GetCreationReceiptId() == creation.String() &&
		bytes.Equal(receipt.GetParticipantManifestSha256(), manifest) && bytes.Equal(receipt.GetRequestSha256(), requestHash) &&
		receipt.GetStatus() == callsv1.MatchSquadTeardownStatus_MATCH_SQUAD_TEARDOWN_STATUS_COMPLETED
}
