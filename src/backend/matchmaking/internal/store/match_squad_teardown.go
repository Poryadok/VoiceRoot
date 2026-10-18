package store

import (
	"bytes"
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	callsv1 "voice.app/voice/calls/v1"
	chatv1 "voice.app/voice/chat/v1"
)

// MatchSquadTeardownAggregate is durable provider work initiated only after
// Matchmaking records every participant's confirmed departure.
type MatchSquadTeardownAggregate struct {
	AggregateID          uuid.UUID
	MatchID              uuid.UUID
	State                string
	Purpose              string
	RequiredProviders    []string
	ChatOperationID      uuid.UUID
	ChatRequestHash      []byte
	ChatRequestBytes     []byte
	ChatReceiptID        *uuid.UUID
	ChatReceiptBytes     []byte
	VoiceOperationID     uuid.UUID
	VoiceRequestHash     []byte
	VoiceRequestBytes    []byte
	VoiceReceiptID       *uuid.UUID
	VoiceReceiptBytes    []byte
	AggregateCompletedAt *time.Time
}

type MatchSquadCompactionIntent struct {
	AggregateID  uuid.UUID
	Provider     string
	OperationID  uuid.UUID
	NotBefore    time.Time
	RequestHash  []byte
	RequestBytes []byte
	ReceiptID    *uuid.UUID
	ReceiptBytes []byte
	CompletedAt  *time.Time
}

func (s *MatchStore) ListPendingMatchSquadTeardowns(ctx context.Context, limit int) ([]MatchSquadTeardownAggregate, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("match store unavailable")
	}
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT aggregate_id, match_id, state,
		       chat_teardown_operation_id, chat_teardown_request_sha256, chat_teardown_request_bytes,
		       chat_teardown_receipt_id, chat_teardown_receipt_bytes,
		       voice_teardown_operation_id, voice_teardown_request_sha256, voice_teardown_request_bytes,
		       voice_teardown_receipt_id, voice_teardown_receipt_bytes, aggregate_completed_at, purpose, required_providers
		FROM matchmaking_match_squad_teardowns
		WHERE state='pending'
		ORDER BY created_at, aggregate_id
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]MatchSquadTeardownAggregate, 0, limit)
	for rows.Next() {
		item, err := scanMatchSquadTeardownAggregate(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *MatchStore) GetMatchSquadTeardown(ctx context.Context, aggregateID uuid.UUID) (MatchSquadTeardownAggregate, error) {
	if s == nil || s.Pool == nil {
		return MatchSquadTeardownAggregate{}, errors.New("match store unavailable")
	}
	item, err := scanMatchSquadTeardownAggregate(s.Pool.QueryRow(ctx, `
		SELECT aggregate_id, match_id, state,
		       chat_teardown_operation_id, chat_teardown_request_sha256, chat_teardown_request_bytes,
		       chat_teardown_receipt_id, chat_teardown_receipt_bytes,
		       voice_teardown_operation_id, voice_teardown_request_sha256, voice_teardown_request_bytes,
		       voice_teardown_receipt_id, voice_teardown_receipt_bytes, aggregate_completed_at, purpose, required_providers
		FROM matchmaking_match_squad_teardowns WHERE aggregate_id=$1
	`, aggregateID))
	if errors.Is(err, pgx.ErrNoRows) {
		return MatchSquadTeardownAggregate{}, ErrMatchSquadPending
	}
	return item, err
}

func (s *MatchStore) ListDueMatchSquadCompactions(ctx context.Context, limit int) ([]MatchSquadCompactionIntent, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("match store unavailable")
	}
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT aggregate_id, provider, operation_id, not_before, request_sha256, request_bytes,
		       receipt_id, receipt_bytes, completed_at
		FROM matchmaking_match_squad_compaction_intents
		WHERE completed_at IS NULL AND not_before <= clock_timestamp()
		ORDER BY not_before, aggregate_id, provider
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]MatchSquadCompactionIntent, 0, limit)
	for rows.Next() {
		item, err := scanMatchSquadCompactionIntent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

// PrepareMatchSquadCompaction persists the exact request after the shared
// 30-day deadline and before a provider RPC. A crash can only retry these bytes.
func (s *MatchStore) PrepareMatchSquadCompaction(ctx context.Context, aggregateID uuid.UUID, provider string) (MatchSquadCompactionIntent, error) {
	if s == nil || s.Pool == nil {
		return MatchSquadCompactionIntent{}, errors.New("match store unavailable")
	}
	if aggregateID == uuid.Nil || (provider != "chat" && provider != "voice") {
		return MatchSquadCompactionIntent{}, ErrMatchSquadConflict
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return MatchSquadCompactionIntent{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	intent, err := scanMatchSquadCompactionIntent(tx.QueryRow(ctx, `
		SELECT aggregate_id, provider, operation_id, not_before, request_sha256, request_bytes,
		       receipt_id, receipt_bytes, completed_at
		FROM matchmaking_match_squad_compaction_intents
		WHERE aggregate_id=$1 AND provider=$2 FOR UPDATE
	`, aggregateID, provider))
	if errors.Is(err, pgx.ErrNoRows) {
		return MatchSquadCompactionIntent{}, ErrMatchSquadPending
	}
	if err != nil {
		return MatchSquadCompactionIntent{}, err
	}
	if intent.CompletedAt != nil {
		if err := tx.Commit(ctx); err != nil {
			return MatchSquadCompactionIntent{}, err
		}
		return intent, nil
	}
	var now time.Time
	if err := tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&now); err != nil {
		return MatchSquadCompactionIntent{}, err
	}
	if now.Before(intent.NotBefore) {
		return MatchSquadCompactionIntent{}, ErrMatchSquadPending
	}
	if len(intent.RequestBytes) != 0 {
		if !bytes.Equal(sumSHA256(intent.RequestBytes), intent.RequestHash) {
			return MatchSquadCompactionIntent{}, ErrMatchSquadConflict
		}
		if err := tx.Commit(ctx); err != nil {
			return MatchSquadCompactionIntent{}, err
		}
		return intent, nil
	}
	aggregate, err := scanMatchSquadTeardownAggregate(tx.QueryRow(ctx, `
		SELECT aggregate_id, match_id, state,
		       chat_teardown_operation_id, chat_teardown_request_sha256, chat_teardown_request_bytes,
		       chat_teardown_receipt_id, chat_teardown_receipt_bytes,
		       voice_teardown_operation_id, voice_teardown_request_sha256, voice_teardown_request_bytes,
		       voice_teardown_receipt_id, voice_teardown_receipt_bytes, aggregate_completed_at, purpose, required_providers
		FROM matchmaking_match_squad_teardowns WHERE aggregate_id=$1 FOR UPDATE
	`, aggregateID))
	if err != nil {
		return MatchSquadCompactionIntent{}, err
	}
	if aggregate.State != "complete" || aggregate.AggregateCompletedAt == nil || !aggregate.requiresProvider(provider) ||
		(provider == "chat" && aggregate.ChatReceiptID == nil) || (provider == "voice" && aggregate.VoiceReceiptID == nil) {
		return MatchSquadCompactionIntent{}, ErrMatchSquadPending
	}
	operation, err := scanMatchSquadProvisioningOperation(tx.QueryRow(ctx, `
		SELECT match_id, operation_id, participant_manifest_sha256, participant_manifest_bytes, state,
		       chat_operation_id, chat_request_sha256, chat_request_bytes, chat_receipt_id, chat_receipt_bytes, chat_id,
		       voice_operation_id, voice_request_sha256, voice_request_bytes, voice_receipt_id, voice_receipt_bytes, voice_room_id
		FROM matchmaking_match_squad_operations WHERE match_id=$1 FOR UPDATE
	`, aggregate.MatchID))
	if err != nil {
		return MatchSquadCompactionIntent{}, err
	}
	if operation.ChatID == nil || operation.ChatReceiptID == nil || len(operation.ChatRequestHash) != 32 || len(aggregate.ChatReceiptBytes) == 0 ||
		(provider == "voice" && (operation.VoiceRoomID == nil || operation.VoiceReceiptID == nil || len(operation.VoiceRequestHash) != 32 || len(aggregate.VoiceReceiptBytes) == 0)) {
		return MatchSquadCompactionIntent{}, ErrMatchSquadConflict
	}
	var request proto.Message
	if provider == "chat" {
		request = &chatv1.CompactMatchSquadChatRequest{
			ProtocolVersion: 1, OperationId: intent.OperationID.String(), TeardownAggregateId: aggregate.AggregateID.String(),
			MatchId: aggregate.MatchID.String(), ChatId: operation.ChatID.String(),
			CreationReceiptId: operation.ChatReceiptID.String(), CreationRequestSha256: append([]byte(nil), operation.ChatRequestHash...),
			TeardownOperationId: aggregate.ChatOperationID.String(), TeardownReceiptId: aggregate.ChatReceiptID.String(),
			TeardownReceiptSha256: sumSHA256(aggregate.ChatReceiptBytes), ParticipantManifestSha256: append([]byte(nil), operation.ParticipantManifestHash...),
			AggregateCompletedAt: timestamppb.New(*aggregate.AggregateCompletedAt), CompactionAuthorizedAt: timestamppb.New(now),
		}
	} else {
		request = &callsv1.CompactMatchSquadRoomRequest{
			ProtocolVersion: 1, OperationId: intent.OperationID.String(), TeardownAggregateId: aggregate.AggregateID.String(),
			MatchId: aggregate.MatchID.String(), RoomId: operation.VoiceRoomID.String(),
			CreationReceiptId: operation.VoiceReceiptID.String(), CreationRequestSha256: append([]byte(nil), operation.VoiceRequestHash...),
			TeardownOperationId: aggregate.VoiceOperationID.String(), TeardownReceiptId: aggregate.VoiceReceiptID.String(),
			TeardownReceiptSha256: sumSHA256(aggregate.VoiceReceiptBytes), ParticipantManifestSha256: append([]byte(nil), operation.ParticipantManifestHash...),
			AggregateCompletedAt: timestamppb.New(*aggregate.AggregateCompletedAt), CompactionAuthorizedAt: timestamppb.New(now),
		}
	}
	requestBytes, err := (proto.MarshalOptions{Deterministic: true}).Marshal(request)
	if err != nil {
		return MatchSquadCompactionIntent{}, err
	}
	intent.RequestBytes, intent.RequestHash = requestBytes, sumSHA256(requestBytes)
	_, err = tx.Exec(ctx, `UPDATE matchmaking_match_squad_compaction_intents SET request_sha256=$3, request_bytes=$4, updated_at=clock_timestamp() WHERE aggregate_id=$1 AND provider=$2 AND request_bytes IS NULL AND completed_at IS NULL`, aggregateID, provider, intent.RequestHash, intent.RequestBytes)
	if err != nil {
		return MatchSquadCompactionIntent{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return MatchSquadCompactionIntent{}, err
	}
	return intent, nil
}

// RecordMatchSquadCompactionReceipt preserves a provider's committed compact
// receipt and makes only that provider's due intent terminal.
func (s *MatchStore) RecordMatchSquadCompactionReceipt(ctx context.Context, aggregateID uuid.UUID, provider string, receiptID uuid.UUID, receiptBytes []byte) (MatchSquadCompactionIntent, error) {
	if s == nil || s.Pool == nil {
		return MatchSquadCompactionIntent{}, errors.New("match store unavailable")
	}
	if aggregateID == uuid.Nil || receiptID == uuid.Nil || len(receiptBytes) == 0 || (provider != "chat" && provider != "voice") {
		return MatchSquadCompactionIntent{}, ErrMatchSquadConflict
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return MatchSquadCompactionIntent{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	intent, err := scanMatchSquadCompactionIntent(tx.QueryRow(ctx, `
		SELECT aggregate_id, provider, operation_id, not_before, request_sha256, request_bytes,
		       receipt_id, receipt_bytes, completed_at
		FROM matchmaking_match_squad_compaction_intents
		WHERE aggregate_id=$1 AND provider=$2 FOR UPDATE
	`, aggregateID, provider))
	if errors.Is(err, pgx.ErrNoRows) {
		return MatchSquadCompactionIntent{}, ErrMatchSquadPending
	}
	if err != nil {
		return MatchSquadCompactionIntent{}, err
	}
	if len(intent.RequestBytes) == 0 || !bytes.Equal(sumSHA256(intent.RequestBytes), intent.RequestHash) {
		return MatchSquadCompactionIntent{}, ErrMatchSquadPending
	}
	if err := validateCompactionReceipt(intent, receiptID, receiptBytes); err != nil {
		return MatchSquadCompactionIntent{}, err
	}
	if intent.CompletedAt != nil {
		if intent.ReceiptID == nil || *intent.ReceiptID != receiptID || !bytes.Equal(intent.ReceiptBytes, receiptBytes) {
			return MatchSquadCompactionIntent{}, ErrMatchSquadConflict
		}
		if err := tx.Commit(ctx); err != nil {
			return MatchSquadCompactionIntent{}, err
		}
		return intent, nil
	}
	_, err = tx.Exec(ctx, `UPDATE matchmaking_match_squad_compaction_intents SET receipt_id=$3, receipt_bytes=$4, completed_at=clock_timestamp(), updated_at=clock_timestamp() WHERE aggregate_id=$1 AND provider=$2 AND completed_at IS NULL`, aggregateID, provider, receiptID, receiptBytes)
	if err != nil {
		return MatchSquadCompactionIntent{}, err
	}
	var completedAt time.Time
	if err := tx.QueryRow(ctx, `SELECT completed_at FROM matchmaking_match_squad_compaction_intents WHERE aggregate_id=$1 AND provider=$2`, aggregateID, provider).Scan(&completedAt); err != nil {
		return MatchSquadCompactionIntent{}, err
	}
	intent.ReceiptID, intent.ReceiptBytes, intent.CompletedAt = &receiptID, append([]byte(nil), receiptBytes...), &completedAt
	if err := tx.Commit(ctx); err != nil {
		return MatchSquadCompactionIntent{}, err
	}
	return intent, nil
}

func validateCompactionReceipt(intent MatchSquadCompactionIntent, receiptID uuid.UUID, raw []byte) error {
	if intent.Provider == "chat" {
		request := new(chatv1.CompactMatchSquadChatRequest)
		receipt := new(chatv1.MatchSquadChatCompactionReceipt)
		if proto.Unmarshal(intent.RequestBytes, request) != nil || len(request.ProtoReflect().GetUnknown()) != 0 ||
			proto.Unmarshal(raw, receipt) != nil || receipt.GetProtocolVersion() != 1 || receipt.GetReceiptId() != receiptID.String() ||
			receipt.GetCompactionOperationId() != intent.OperationID.String() || receipt.GetTeardownAggregateId() != intent.AggregateID.String() ||
			receipt.GetMatchId() != request.GetMatchId() || receipt.GetChatId() != request.GetChatId() ||
			!bytes.Equal(receipt.GetRequestSha256(), sumSHA256(intent.RequestBytes)) ||
			receipt.GetStatus() != chatv1.MatchSquadChatCompactionStatus_MATCH_SQUAD_CHAT_COMPACTION_STATUS_COMPACTED ||
			!sameTimestamp(receipt.GetAggregateCompletedAt(), request.GetAggregateCompletedAt()) ||
			!sameTimestamp(receipt.GetCompactionAuthorizedAt(), request.GetCompactionAuthorizedAt()) {
			return ErrMatchSquadConflict
		}
		return nil
	}
	request := new(callsv1.CompactMatchSquadRoomRequest)
	receipt := new(callsv1.MatchSquadRoomCompactionReceipt)
	if proto.Unmarshal(intent.RequestBytes, request) != nil || len(request.ProtoReflect().GetUnknown()) != 0 ||
		proto.Unmarshal(raw, receipt) != nil || receipt.GetProtocolVersion() != 1 || receipt.GetReceiptId() != receiptID.String() ||
		receipt.GetCompactionOperationId() != intent.OperationID.String() || receipt.GetTeardownAggregateId() != intent.AggregateID.String() ||
		receipt.GetMatchId() != request.GetMatchId() || receipt.GetRoomId() != request.GetRoomId() ||
		!bytes.Equal(receipt.GetRequestSha256(), sumSHA256(intent.RequestBytes)) ||
		receipt.GetStatus() != callsv1.MatchSquadRoomCompactionStatus_MATCH_SQUAD_ROOM_COMPACTION_STATUS_COMPACTED ||
		!sameTimestamp(receipt.GetAggregateCompletedAt(), request.GetAggregateCompletedAt()) ||
		!sameTimestamp(receipt.GetCompactionAuthorizedAt(), request.GetCompactionAuthorizedAt()) {
		return ErrMatchSquadConflict
	}
	return nil
}

func sameTimestamp(left, right *timestamppb.Timestamp) bool {
	return left != nil && right != nil && left.CheckValid() == nil && right.CheckValid() == nil && left.AsTime().Equal(right.AsTime())
}

// RecordMatchSquadTeardownReceipt commits exact typed provider evidence. The
// aggregate clock begins only after both providers confirm terminal absence.
func (s *MatchStore) RecordMatchSquadTeardownReceipt(ctx context.Context, aggregateID uuid.UUID, provider string, receiptID uuid.UUID, receiptBytes []byte) (MatchSquadTeardownAggregate, error) {
	if s == nil || s.Pool == nil {
		return MatchSquadTeardownAggregate{}, errors.New("match store unavailable")
	}
	if aggregateID == uuid.Nil || receiptID == uuid.Nil || len(receiptBytes) == 0 || (provider != "chat" && provider != "voice") {
		return MatchSquadTeardownAggregate{}, ErrMatchSquadConflict
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return MatchSquadTeardownAggregate{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	item, err := scanMatchSquadTeardownAggregate(tx.QueryRow(ctx, `
		SELECT aggregate_id, match_id, state,
		       chat_teardown_operation_id, chat_teardown_request_sha256, chat_teardown_request_bytes,
		       chat_teardown_receipt_id, chat_teardown_receipt_bytes,
		       voice_teardown_operation_id, voice_teardown_request_sha256, voice_teardown_request_bytes,
		       voice_teardown_receipt_id, voice_teardown_receipt_bytes, aggregate_completed_at, purpose, required_providers
		FROM matchmaking_match_squad_teardowns WHERE aggregate_id=$1 FOR UPDATE
	`, aggregateID))
	if errors.Is(err, pgx.ErrNoRows) {
		return MatchSquadTeardownAggregate{}, ErrMatchSquadPending
	}
	if err != nil {
		return MatchSquadTeardownAggregate{}, err
	}
	if !item.requiresProvider(provider) {
		return MatchSquadTeardownAggregate{}, ErrMatchSquadConflict
	}
	if item.State == "complete" {
		if provider == "chat" {
			if err := validateChatTeardownReceipt(item, receiptID, receiptBytes); err != nil || item.ChatReceiptID == nil || *item.ChatReceiptID != receiptID || !bytes.Equal(item.ChatReceiptBytes, receiptBytes) {
				return MatchSquadTeardownAggregate{}, ErrMatchSquadConflict
			}
		} else if err := validateVoiceTeardownReceipt(item, receiptID, receiptBytes); err != nil || item.VoiceReceiptID == nil || *item.VoiceReceiptID != receiptID || !bytes.Equal(item.VoiceReceiptBytes, receiptBytes) {
			return MatchSquadTeardownAggregate{}, ErrMatchSquadConflict
		}
		if err := recordMatchSquadTeardownParticipantReceiptTx(ctx, tx, item, provider, receiptID, receiptBytes); err != nil {
			return MatchSquadTeardownAggregate{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return MatchSquadTeardownAggregate{}, err
		}
		return item, nil
	}
	if item.State != "pending" {
		return MatchSquadTeardownAggregate{}, ErrMatchSquadPending
	}
	if provider == "chat" {
		if err := validateChatTeardownReceipt(item, receiptID, receiptBytes); err != nil {
			return MatchSquadTeardownAggregate{}, err
		}
		if item.ChatReceiptID != nil {
			if *item.ChatReceiptID != receiptID || !bytes.Equal(item.ChatReceiptBytes, receiptBytes) {
				return MatchSquadTeardownAggregate{}, ErrMatchSquadConflict
			}
		} else {
			_, err = tx.Exec(ctx, `UPDATE matchmaking_match_squad_teardowns SET chat_teardown_receipt_id=$2, chat_teardown_receipt_bytes=$3, updated_at=clock_timestamp() WHERE aggregate_id=$1 AND state='pending' AND chat_teardown_receipt_id IS NULL`, aggregateID, receiptID, receiptBytes)
			if err != nil {
				return MatchSquadTeardownAggregate{}, err
			}
			item.ChatReceiptID, item.ChatReceiptBytes = &receiptID, append([]byte(nil), receiptBytes...)
		}
	} else {
		if err := validateVoiceTeardownReceipt(item, receiptID, receiptBytes); err != nil {
			return MatchSquadTeardownAggregate{}, err
		}
		if item.VoiceReceiptID != nil {
			if *item.VoiceReceiptID != receiptID || !bytes.Equal(item.VoiceReceiptBytes, receiptBytes) {
				return MatchSquadTeardownAggregate{}, ErrMatchSquadConflict
			}
		} else {
			_, err = tx.Exec(ctx, `UPDATE matchmaking_match_squad_teardowns SET voice_teardown_receipt_id=$2, voice_teardown_receipt_bytes=$3, updated_at=clock_timestamp() WHERE aggregate_id=$1 AND state='pending' AND voice_teardown_receipt_id IS NULL`, aggregateID, receiptID, receiptBytes)
			if err != nil {
				return MatchSquadTeardownAggregate{}, err
			}
			item.VoiceReceiptID, item.VoiceReceiptBytes = &receiptID, append([]byte(nil), receiptBytes...)
		}
	}
	if err := recordMatchSquadTeardownParticipantReceiptTx(ctx, tx, item, provider, receiptID, receiptBytes); err != nil {
		return MatchSquadTeardownAggregate{}, err
	}
	complete := (!item.requiresProvider("chat") || item.ChatReceiptID != nil) && (!item.requiresProvider("voice") || item.VoiceReceiptID != nil)
	if complete {
		var completedAt time.Time
		if err := tx.QueryRow(ctx, `UPDATE matchmaking_match_squad_teardowns SET state='complete', aggregate_completed_at=clock_timestamp(), updated_at=clock_timestamp() WHERE aggregate_id=$1 AND state='pending' RETURNING aggregate_completed_at`, aggregateID).Scan(&completedAt); err != nil {
			return MatchSquadTeardownAggregate{}, err
		}
		for _, provider := range item.RequiredProviders {
			_, err := tx.Exec(ctx, `INSERT INTO matchmaking_match_squad_compaction_intents (aggregate_id, provider, operation_id, not_before) VALUES ($1,$2,$3,$4)`, aggregateID, provider, uuid.New(), completedAt.Add(30*24*time.Hour))
			if err != nil {
				return MatchSquadTeardownAggregate{}, err
			}
		}
		fromState := "active"
		if item.Purpose == "PROVISION_COMPENSATION" {
			fromState = "compensating"
		}
		closed, err := tx.Exec(ctx, `UPDATE matchmaking_match_squad_operations SET state='closed', updated_at=clock_timestamp() WHERE match_id=$1 AND state=$2`, item.MatchID, fromState)
		if err != nil {
			return MatchSquadTeardownAggregate{}, err
		}
		if closed.RowsAffected() != 1 {
			return MatchSquadTeardownAggregate{}, ErrMatchSquadConflict
		}
		if item.Purpose == "FINAL_LEAVE" {
			if err := ensureMatchSquadCompletionEventTx(ctx, tx, aggregateID, item.MatchID); err != nil {
				return MatchSquadTeardownAggregate{}, err
			}
		}
		item.State, item.AggregateCompletedAt = "complete", &completedAt
	}
	if err := tx.Commit(ctx); err != nil {
		return MatchSquadTeardownAggregate{}, err
	}
	return item, nil
}

func validateChatTeardownReceipt(item MatchSquadTeardownAggregate, receiptID uuid.UUID, raw []byte) error {
	request := new(chatv1.TeardownMatchSquadChatRequest)
	receipt := new(chatv1.MatchSquadChatTeardownReceipt)
	if proto.Unmarshal(item.ChatRequestBytes, request) != nil || len(request.ProtoReflect().GetUnknown()) != 0 ||
		!bytes.Equal(sumSHA256(item.ChatRequestBytes), item.ChatRequestHash) ||
		proto.Unmarshal(raw, receipt) != nil || receipt.GetProtocolVersion() != 1 || receipt.GetReceiptId() != receiptID.String() ||
		receipt.GetTeardownOperationId() != item.ChatOperationID.String() || receipt.GetMatchId() != item.MatchID.String() ||
		receipt.GetChatId() != request.GetChatId() || receipt.GetCreationReceiptId() != request.GetCreationReceiptId() ||
		!bytes.Equal(receipt.GetParticipantManifestSha256(), request.GetParticipantManifestSha256()) ||
		!bytes.Equal(receipt.GetRequestSha256(), sumSHA256(item.ChatRequestBytes)) ||
		receipt.GetStatus() != chatv1.MatchSquadTeardownStatus_MATCH_SQUAD_TEARDOWN_STATUS_COMPLETED ||
		!validCompletedAt(receipt.GetCompletedAt()) {
		return ErrMatchSquadConflict
	}
	return nil
}

func validateVoiceTeardownReceipt(item MatchSquadTeardownAggregate, receiptID uuid.UUID, raw []byte) error {
	request := new(callsv1.TeardownMatchSquadRoomRequest)
	receipt := new(callsv1.MatchSquadRoomTeardownReceipt)
	if proto.Unmarshal(item.VoiceRequestBytes, request) != nil || len(request.ProtoReflect().GetUnknown()) != 0 ||
		!bytes.Equal(sumSHA256(item.VoiceRequestBytes), item.VoiceRequestHash) ||
		proto.Unmarshal(raw, receipt) != nil || receipt.GetProtocolVersion() != 1 || receipt.GetReceiptId() != receiptID.String() ||
		receipt.GetTeardownOperationId() != item.VoiceOperationID.String() || receipt.GetMatchId() != item.MatchID.String() ||
		receipt.GetRoomId() != request.GetRoomId() || receipt.GetCreationReceiptId() != request.GetCreationReceiptId() ||
		!bytes.Equal(receipt.GetParticipantManifestSha256(), request.GetParticipantManifestSha256()) ||
		!bytes.Equal(receipt.GetRequestSha256(), sumSHA256(item.VoiceRequestBytes)) ||
		receipt.GetStatus() != callsv1.MatchSquadTeardownStatus_MATCH_SQUAD_TEARDOWN_STATUS_COMPLETED ||
		!validCompletedAt(receipt.GetCompletedAt()) {
		return ErrMatchSquadConflict
	}
	return nil
}

func validCompletedAt(value *timestamppb.Timestamp) bool {
	return value != nil && value.CheckValid() == nil && value.AsTime().Unix() > 0
}

func scanMatchSquadTeardownAggregate(row pgx.Row) (MatchSquadTeardownAggregate, error) {
	var item MatchSquadTeardownAggregate
	var chatReceiptID, voiceReceiptID *uuid.UUID
	var chatOperationID, voiceOperationID *uuid.UUID
	var completedAt *time.Time
	err := row.Scan(
		&item.AggregateID, &item.MatchID, &item.State,
		&chatOperationID, &item.ChatRequestHash, &item.ChatRequestBytes,
		&chatReceiptID, &item.ChatReceiptBytes,
		&voiceOperationID, &item.VoiceRequestHash, &item.VoiceRequestBytes,
		&voiceReceiptID, &item.VoiceReceiptBytes, &completedAt,
		&item.Purpose, &item.RequiredProviders,
	)
	if chatOperationID != nil {
		item.ChatOperationID = *chatOperationID
	}
	if voiceOperationID != nil {
		item.VoiceOperationID = *voiceOperationID
	}
	item.ChatReceiptID, item.VoiceReceiptID, item.AggregateCompletedAt = chatReceiptID, voiceReceiptID, completedAt
	return item, err
}

func (item MatchSquadTeardownAggregate) requiresProvider(provider string) bool {
	for _, required := range item.RequiredProviders {
		if required == provider {
			return true
		}
	}
	return false
}

func scanMatchSquadCompactionIntent(row pgx.Row) (MatchSquadCompactionIntent, error) {
	var item MatchSquadCompactionIntent
	var receiptID *uuid.UUID
	var completedAt *time.Time
	err := row.Scan(&item.AggregateID, &item.Provider, &item.OperationID, &item.NotBefore,
		&item.RequestHash, &item.RequestBytes, &receiptID, &item.ReceiptBytes, &completedAt)
	if err != nil {
		return MatchSquadCompactionIntent{}, err
	}
	item.ReceiptID, item.CompletedAt = receiptID, completedAt
	return item, nil
}
