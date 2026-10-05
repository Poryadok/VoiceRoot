package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	chatv1 "voice.app/voice/chat/v1"
)

var (
	ErrMatchSquadConflict = errors.New("match squad operation conflicts with stored ownership")
	ErrMatchSquadTerminal = errors.New("match squad chat is already terminal")
	ErrMatchSquadNotFound = errors.New("match squad chat not found")
)

type MatchSquadStore struct{ Pool *pgxpool.Pool }

type MatchSquadCompaction struct {
	OperationID               uuid.UUID
	AggregateID               uuid.UUID
	MatchID                   uuid.UUID
	ChatID                    uuid.UUID
	CreationReceiptID         uuid.UUID
	CreationRequestSHA256     []byte
	TeardownOperationID       uuid.UUID
	TeardownReceiptID         uuid.UUID
	TeardownReceiptSHA256     []byte
	ParticipantManifestSHA256 []byte
	AggregateCompletedAt      time.Time
	CompactionAuthorizedAt    time.Time
	RequestBytes              []byte
}

func (s *MatchSquadStore) Create(ctx context.Context, operationID, matchID uuid.UUID, participants []uuid.UUID, manifest, requestBytes []byte) ([]byte, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("match squad store is not configured")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1, 21))", matchID.String()); err != nil {
		return nil, err
	}
	var existingBytes, receiptBytes []byte
	var compactedAt *time.Time
	err = tx.QueryRow(ctx, `SELECT creation_request_bytes, creation_receipt_bytes, compacted_at
FROM chat_match_squad_operations WHERE operation_id=$1 FOR UPDATE`, operationID).Scan(&existingBytes, &receiptBytes, &compactedAt)
	if err == nil {
		if compactedAt != nil {
			return nil, ErrMatchSquadTerminal
		}
		if !bytes.Equal(existingBytes, requestBytes) {
			return nil, ErrMatchSquadConflict
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return receiptBytes, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	var oldOperation uuid.UUID
	err = tx.QueryRow(ctx, `SELECT operation_id FROM chat_match_squad_operations WHERE match_id=$1 FOR UPDATE`, matchID).Scan(&oldOperation)
	if err == nil {
		return nil, ErrMatchSquadConflict
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	chatID, receiptID := uuid.New(), uuid.New()
	requestHash := sha256.Sum256(requestBytes)
	createdAt := time.Now().UTC()
	if err := tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&createdAt); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO chats (id, type, name, creator_profile_id, slow_mode_seconds, created_at, updated_at)
VALUES ($1, 'group', 'Match squad', $2, 0, $3, $3)`, chatID, participants[0], createdAt); err != nil {
		return nil, err
	}
	for _, profileID := range participants {
		if _, err := tx.Exec(ctx, `INSERT INTO chat_members (chat_id, profile_id, role, inbox_bucket)
VALUES ($1, $2, 'member', 'main')`, chatID, profileID); err != nil {
			return nil, err
		}
	}
	receipt := &chatv1.MatchSquadChatReceipt{ProtocolVersion: 1, ReceiptId: receiptID.String(), OperationId: operationID.String(), MatchId: matchID.String(), ChatId: chatID.String(), ParticipantManifestSha256: append([]byte(nil), manifest...), RequestSha256: requestHash[:], CreatedAt: timestamppb.New(createdAt)}
	receiptBytes, err = proto.MarshalOptions{Deterministic: true}.Marshal(receipt)
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO chat_match_squad_operations
(operation_id, match_id, chat_id, creation_receipt_id, participant_manifest_sha256, creation_request_sha256, creation_request_bytes, creation_receipt_bytes, created_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`, operationID, matchID, chatID, receiptID, manifest, requestHash[:], requestBytes, receiptBytes, createdAt); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return receiptBytes, nil
}

func (s *MatchSquadStore) Teardown(ctx context.Context, operationID, matchID, chatID, creationReceiptID uuid.UUID, manifest, creationRequestHash, requestBytes []byte) ([]byte, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("match squad store is not configured")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1, 21))", matchID.String()); err != nil {
		return nil, err
	}
	var storedChatID, storedReceiptID uuid.UUID
	var storedManifest, storedCreationHash, storedCreateBytes, storedCreateReceipt []byte
	var storedTeardownID *uuid.UUID
	var storedTeardownHash, storedTeardownBytes, storedTeardownReceipt []byte
	var storedTeardownReceiptID *uuid.UUID
	var completedAt, compactedAt *time.Time
	err = tx.QueryRow(ctx, `SELECT chat_id, creation_receipt_id, participant_manifest_sha256, creation_request_sha256,
creation_request_bytes, creation_receipt_bytes, teardown_operation_id, teardown_request_sha256, teardown_request_bytes,
teardown_receipt_id, teardown_receipt_bytes, teardown_completed_at, compacted_at
FROM chat_match_squad_operations WHERE match_id=$1 FOR UPDATE`, matchID).Scan(&storedChatID, &storedReceiptID, &storedManifest, &storedCreationHash, &storedCreateBytes, &storedCreateReceipt, &storedTeardownID, &storedTeardownHash, &storedTeardownBytes, &storedTeardownReceiptID, &storedTeardownReceipt, &completedAt, &compactedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrMatchSquadNotFound
	}
	if err != nil {
		return nil, err
	}
	if storedChatID != chatID || storedReceiptID != creationReceiptID || !bytes.Equal(storedManifest, manifest) || !bytes.Equal(storedCreationHash, creationRequestHash) {
		return nil, ErrMatchSquadConflict
	}
	if storedTeardownID != nil {
		if *storedTeardownID != operationID || !bytes.Equal(storedTeardownHash, sha256Bytes(requestBytes)) {
			return nil, ErrMatchSquadConflict
		}
		if compactedAt != nil || storedTeardownReceipt == nil {
			return nil, ErrMatchSquadTerminal
		}
		if !bytes.Equal(storedTeardownBytes, requestBytes) {
			return nil, ErrMatchSquadConflict
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return storedTeardownReceipt, nil
	}
	if storedCreateReceipt == nil || storedCreateBytes == nil {
		return nil, ErrMatchSquadTerminal
	}
	if _, err := tx.Exec(ctx, `DELETE FROM chat_members WHERE chat_id=$1`, chatID); err != nil {
		return nil, err
	}
	teardownRequestHash := sha256.Sum256(requestBytes)
	teardownReceiptID := uuid.New()
	var done time.Time
	if err := tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&done); err != nil {
		return nil, err
	}
	receipt := &chatv1.MatchSquadChatTeardownReceipt{ProtocolVersion: 1, ReceiptId: teardownReceiptID.String(), TeardownOperationId: operationID.String(), MatchId: matchID.String(), ChatId: chatID.String(), CreationReceiptId: creationReceiptID.String(), ParticipantManifestSha256: append([]byte(nil), manifest...), RequestSha256: teardownRequestHash[:], Status: chatv1.MatchSquadTeardownStatus_MATCH_SQUAD_TEARDOWN_STATUS_COMPLETED, CompletedAt: timestamppb.New(done)}
	receiptBytes, err := proto.MarshalOptions{Deterministic: true}.Marshal(receipt)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `UPDATE chat_match_squad_operations SET teardown_operation_id=$2, teardown_request_sha256=$3,
teardown_request_bytes=$4, teardown_receipt_id=$5, teardown_receipt_bytes=$6, teardown_completed_at=$7 WHERE match_id=$1`, matchID, operationID, teardownRequestHash[:], requestBytes, teardownReceiptID, receiptBytes, done)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return receiptBytes, nil
}

// Compact atomically stores the Matchmaking aggregate-completion command and
// exact compact receipt before clearing only the original full request/receipt
// payloads. The command timestamps are trusted attestations; this service never
// derives eligibility from its own clock.
func (s *MatchSquadStore) Compact(ctx context.Context, request MatchSquadCompaction) ([]byte, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("match squad store is not configured")
	}
	if request.OperationID == uuid.Nil || request.AggregateID == uuid.Nil || request.MatchID == uuid.Nil || request.ChatID == uuid.Nil || request.CreationReceiptID == uuid.Nil || request.TeardownOperationID == uuid.Nil || request.TeardownReceiptID == uuid.Nil ||
		len(request.CreationRequestSHA256) != sha256.Size || len(request.TeardownReceiptSHA256) != sha256.Size || len(request.ParticipantManifestSHA256) != sha256.Size || len(request.RequestBytes) == 0 || request.AggregateCompletedAt.IsZero() || request.CompactionAuthorizedAt.IsZero() {
		return nil, ErrMatchSquadConflict
	}
	request.AggregateCompletedAt = request.AggregateCompletedAt.UTC()
	request.CompactionAuthorizedAt = request.CompactionAuthorizedAt.UTC()
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1, 21))", request.MatchID.String()); err != nil {
		return nil, err
	}
	var storedOperationID, storedChatID, storedCreationReceiptID uuid.UUID
	var storedCreatedAt time.Time
	var storedManifest, storedCreationHash, storedCreationBytes, storedCreationReceipt []byte
	var storedTeardownOperationID *uuid.UUID
	var storedTeardownHash, storedTeardownBytes, storedTeardownReceipt []byte
	var storedTeardownReceiptID *uuid.UUID
	var storedTeardownCompletedAt *time.Time
	var storedCompactionOperationID *uuid.UUID
	var storedCompactionRequestHash, storedCompactionRequestBytes, storedCompactionReceipt []byte
	var storedCompactionReceiptID *uuid.UUID
	var storedAggregateID *uuid.UUID
	var storedAggregateCompletedAt, storedCompactionAuthorizedAt, compactedAt *time.Time
	err = tx.QueryRow(ctx, `SELECT operation_id, chat_id, creation_receipt_id, participant_manifest_sha256, creation_request_sha256, created_at,
creation_request_bytes, creation_receipt_bytes, teardown_operation_id, teardown_request_sha256, teardown_request_bytes,
teardown_receipt_id, teardown_receipt_bytes, teardown_completed_at, compaction_operation_id, teardown_aggregate_id,
compaction_request_sha256, compaction_request_bytes, compaction_receipt_id, compaction_receipt_bytes,
aggregate_completed_at, compaction_authorized_at, compacted_at
FROM chat_match_squad_operations WHERE match_id=$1 FOR UPDATE`, request.MatchID).Scan(
		&storedOperationID, &storedChatID, &storedCreationReceiptID, &storedManifest, &storedCreationHash, &storedCreatedAt, &storedCreationBytes, &storedCreationReceipt,
		&storedTeardownOperationID, &storedTeardownHash, &storedTeardownBytes, &storedTeardownReceiptID, &storedTeardownReceipt,
		&storedTeardownCompletedAt, &storedCompactionOperationID, &storedAggregateID, &storedCompactionRequestHash,
		&storedCompactionRequestBytes, &storedCompactionReceiptID, &storedCompactionReceipt, &storedAggregateCompletedAt,
		&storedCompactionAuthorizedAt, &compactedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrMatchSquadNotFound
	}
	if err != nil {
		return nil, err
	}
	if storedCompactionOperationID != nil {
		if *storedCompactionOperationID != request.OperationID || !bytes.Equal(storedCompactionRequestBytes, request.RequestBytes) {
			return nil, ErrMatchSquadConflict
		}
		requestHash := sha256.Sum256(request.RequestBytes)
		if compactedAt == nil || len(storedCompactionReceipt) == 0 || storedCompactionReceiptID == nil || storedAggregateID == nil || *storedAggregateID != request.AggregateID || storedAggregateCompletedAt == nil || storedCompactionAuthorizedAt == nil ||
			!bytes.Equal(storedCompactionRequestHash, requestHash[:]) || !storedAggregateCompletedAt.Equal(request.AggregateCompletedAt) || !storedCompactionAuthorizedAt.Equal(request.CompactionAuthorizedAt) {
			return nil, errors.New("incomplete persisted MatchSquad compaction receipt")
		}
		var savedReceipt chatv1.MatchSquadChatCompactionReceipt
		if err := proto.Unmarshal(storedCompactionReceipt, &savedReceipt); err != nil || len(savedReceipt.ProtoReflect().GetUnknown()) != 0 || savedReceipt.GetProtocolVersion() != 1 || savedReceipt.GetReceiptId() != storedCompactionReceiptID.String() || savedReceipt.GetCompactionOperationId() != request.OperationID.String() || savedReceipt.GetTeardownAggregateId() != request.AggregateID.String() || savedReceipt.GetMatchId() != request.MatchID.String() || savedReceipt.GetChatId() != request.ChatID.String() || !bytes.Equal(savedReceipt.GetRequestSha256(), requestHash[:]) || savedReceipt.GetStatus() != chatv1.MatchSquadChatCompactionStatus_MATCH_SQUAD_CHAT_COMPACTION_STATUS_COMPACTED || savedReceipt.GetAggregateCompletedAt() == nil || savedReceipt.GetAggregateCompletedAt().CheckValid() != nil || !savedReceipt.GetAggregateCompletedAt().AsTime().Equal(request.AggregateCompletedAt) || savedReceipt.GetCompactionAuthorizedAt() == nil || savedReceipt.GetCompactionAuthorizedAt().CheckValid() != nil || !savedReceipt.GetCompactionAuthorizedAt().AsTime().Equal(request.CompactionAuthorizedAt) {
			return nil, errors.New("persisted MatchSquad compaction receipt is inconsistent")
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return storedCompactionReceipt, nil
	}
	var retentionElapsed bool
	if err := tx.QueryRow(ctx, `SELECT $2::timestamptz >= $1::timestamptz + INTERVAL '30 days'`, request.AggregateCompletedAt, request.CompactionAuthorizedAt).Scan(&retentionElapsed); err != nil {
		return nil, err
	}
	if !retentionElapsed {
		return nil, ErrMatchSquadConflict
	}
	if compactedAt != nil || storedTeardownOperationID == nil || storedTeardownReceiptID == nil || storedTeardownCompletedAt == nil || len(storedCreationBytes) == 0 || len(storedCreationReceipt) == 0 || len(storedTeardownBytes) == 0 || len(storedTeardownReceipt) == 0 {
		return nil, ErrMatchSquadTerminal
	}
	if storedChatID != request.ChatID || storedCreationReceiptID != request.CreationReceiptID || storedTeardownOperationID == nil || *storedTeardownOperationID != request.TeardownOperationID || *storedTeardownReceiptID != request.TeardownReceiptID ||
		!bytes.Equal(storedManifest, request.ParticipantManifestSHA256) || !bytes.Equal(storedCreationHash, request.CreationRequestSHA256) || !bytes.Equal(storedCreationHash, sha256Bytes(storedCreationBytes)) || !bytes.Equal(storedTeardownHash, sha256Bytes(storedTeardownBytes)) || !bytes.Equal(sha256Bytes(storedTeardownReceipt), request.TeardownReceiptSHA256) {
		return nil, ErrMatchSquadConflict
	}
	var createReceipt chatv1.MatchSquadChatReceipt
	if err := proto.Unmarshal(storedCreationReceipt, &createReceipt); err != nil || len(createReceipt.ProtoReflect().GetUnknown()) != 0 || createReceipt.GetProtocolVersion() != 1 || createReceipt.GetReceiptId() != storedCreationReceiptID.String() || createReceipt.GetOperationId() != storedOperationID.String() || createReceipt.GetMatchId() != request.MatchID.String() || createReceipt.GetChatId() != request.ChatID.String() || !bytes.Equal(createReceipt.GetParticipantManifestSha256(), storedManifest) || !bytes.Equal(createReceipt.GetRequestSha256(), storedCreationHash) || createReceipt.GetCreatedAt() == nil || createReceipt.GetCreatedAt().CheckValid() != nil || !createReceipt.GetCreatedAt().AsTime().Equal(storedCreatedAt) {
		return nil, errors.New("persisted MatchSquad creation receipt does not match its ownership row")
	}
	var teardownReceipt chatv1.MatchSquadChatTeardownReceipt
	if err := proto.Unmarshal(storedTeardownReceipt, &teardownReceipt); err != nil || len(teardownReceipt.ProtoReflect().GetUnknown()) != 0 || teardownReceipt.GetProtocolVersion() != 1 || teardownReceipt.GetStatus() != chatv1.MatchSquadTeardownStatus_MATCH_SQUAD_TEARDOWN_STATUS_COMPLETED || teardownReceipt.GetReceiptId() != storedTeardownReceiptID.String() || teardownReceipt.GetTeardownOperationId() != storedTeardownOperationID.String() || teardownReceipt.GetMatchId() != request.MatchID.String() || teardownReceipt.GetChatId() != request.ChatID.String() || teardownReceipt.GetCreationReceiptId() != storedCreationReceiptID.String() || !bytes.Equal(teardownReceipt.GetParticipantManifestSha256(), storedManifest) || !bytes.Equal(teardownReceipt.GetRequestSha256(), storedTeardownHash) || teardownReceipt.GetCompletedAt() == nil || teardownReceipt.GetCompletedAt().CheckValid() != nil || !teardownReceipt.GetCompletedAt().AsTime().Equal(*storedTeardownCompletedAt) {
		return nil, errors.New("persisted MatchSquad teardown receipt does not match its ownership row")
	}
	requestHash := sha256.Sum256(request.RequestBytes)
	receiptID := uuid.New()
	aggregateAt := request.AggregateCompletedAt.UTC()
	authorizedAt := request.CompactionAuthorizedAt.UTC()
	receipt := &chatv1.MatchSquadChatCompactionReceipt{
		ProtocolVersion: 1, ReceiptId: receiptID.String(), CompactionOperationId: request.OperationID.String(),
		TeardownAggregateId: request.AggregateID.String(), MatchId: request.MatchID.String(), ChatId: request.ChatID.String(),
		RequestSha256: requestHash[:], AggregateCompletedAt: timestamppb.New(aggregateAt),
		CompactionAuthorizedAt: timestamppb.New(authorizedAt), Status: chatv1.MatchSquadChatCompactionStatus_MATCH_SQUAD_CHAT_COMPACTION_STATUS_COMPACTED,
	}
	receiptBytes, err := proto.MarshalOptions{Deterministic: true}.Marshal(receipt)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `UPDATE chat_match_squad_operations SET creation_request_bytes=NULL, creation_receipt_bytes=NULL,
teardown_request_bytes=NULL, teardown_receipt_bytes=NULL, compaction_operation_id=$2, teardown_aggregate_id=$3,
compaction_request_sha256=$4, compaction_request_bytes=$5, compaction_receipt_id=$6, compaction_receipt_bytes=$7,
aggregate_completed_at=$8, compaction_authorized_at=$9, compacted_at=clock_timestamp() WHERE match_id=$1`, request.MatchID,
		request.OperationID, request.AggregateID, requestHash[:], request.RequestBytes, receiptID, receiptBytes, aggregateAt, authorizedAt)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return receiptBytes, nil
}

func (s *MatchSquadStore) IsMatchSquadChat(ctx context.Context, chatID uuid.UUID) (bool, error) {
	if s == nil || s.Pool == nil {
		return false, errors.New("match squad store is not configured")
	}
	var exists bool
	err := s.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM chat_match_squad_operations WHERE chat_id=$1)`, chatID).Scan(&exists)
	return exists, err
}

func (s *DMStore) IsMatchSquadChat(ctx context.Context, chatID uuid.UUID) (bool, error) {
	if s == nil || s.Pool == nil {
		return false, errors.New("dm store is not configured")
	}
	var exists bool
	err := s.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM chat_match_squad_operations WHERE chat_id=$1)`, chatID).Scan(&exists)
	return exists, err
}

func sha256Bytes(value []byte) []byte { sum := sha256.Sum256(value); return sum[:] }
