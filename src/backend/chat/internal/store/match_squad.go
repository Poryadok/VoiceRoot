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
