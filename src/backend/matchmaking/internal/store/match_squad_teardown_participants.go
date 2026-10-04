package store

import (
	"bytes"
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// MatchSquadTeardownParticipant is one independently retriable, immutable
// provider request within the completion aggregate.
type MatchSquadTeardownParticipant struct {
	AggregateID uuid.UUID
	Provider    string
	OperationID uuid.UUID
	State       string
	RequestHash []byte
	Request     []byte
	ReceiptID   *uuid.UUID
	Receipt     []byte
}

func (s *MatchStore) ListPendingMatchSquadTeardownParticipants(ctx context.Context, limit int) ([]MatchSquadTeardownParticipant, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("match store unavailable")
	}
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT p.aggregate_id,p.provider,p.operation_id,p.state,p.request_sha256,p.request_bytes,p.receipt_id,p.receipt_bytes
		FROM matchmaking_match_squad_teardown_participants p
		JOIN matchmaking_match_squad_teardowns a USING (aggregate_id)
		WHERE a.state='pending' AND p.state IN ('NOT_STARTED','IN_FLIGHT','RETRYABLE_FAILURE')
		ORDER BY p.created_at,p.aggregate_id,p.provider LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MatchSquadTeardownParticipant
	for rows.Next() {
		var item MatchSquadTeardownParticipant
		if err := rows.Scan(&item.AggregateID, &item.Provider, &item.OperationID, &item.State, &item.RequestHash, &item.Request, &item.ReceiptID, &item.Receipt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

// SetMatchSquadTeardownParticipantState records retry progress without changing
// the immutable operation or request. Only known retry states are accepted.
func (s *MatchStore) SetMatchSquadTeardownParticipantState(ctx context.Context, aggregateID uuid.UUID, provider, state string) error {
	if s == nil || s.Pool == nil {
		return errors.New("match store unavailable")
	}
	if aggregateID == uuid.Nil || (provider != "chat" && provider != "voice") ||
		(state != "IN_FLIGHT" && state != "RETRYABLE_FAILURE" && state != "CONTRACT_MISMATCH") {
		return ErrMatchSquadConflict
	}
	tag, err := s.Pool.Exec(ctx, `
		UPDATE matchmaking_match_squad_teardown_participants SET state=$3,updated_at=clock_timestamp()
		WHERE aggregate_id=$1 AND provider=$2 AND state IN ('NOT_STARTED','IN_FLIGHT','RETRYABLE_FAILURE') AND receipt_id IS NULL
	`, aggregateID, provider, state)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		var current string
		if err := s.Pool.QueryRow(ctx, `SELECT state FROM matchmaking_match_squad_teardown_participants WHERE aggregate_id=$1 AND provider=$2`, aggregateID, provider).Scan(&current); err != nil {
			return err
		}
		if current != state {
			return ErrMatchSquadConflict
		}
	}
	return nil
}

func recordMatchSquadTeardownParticipantReceiptTx(ctx context.Context, tx pgx.Tx, item MatchSquadTeardownAggregate, provider string, receiptID uuid.UUID, receipt []byte) error {
	var operationID uuid.UUID
	var requestHash, request []byte
	var state string
	var existingID *uuid.UUID
	var existingReceipt []byte
	if err := tx.QueryRow(ctx, `
		SELECT operation_id,request_sha256,request_bytes,state,receipt_id,receipt_bytes
		FROM matchmaking_match_squad_teardown_participants
		WHERE aggregate_id=$1 AND provider=$2 FOR UPDATE
	`, item.AggregateID, provider).Scan(&operationID, &requestHash, &request, &state, &existingID, &existingReceipt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrMatchSquadConflict
		}
		return err
	}
	var expectedOperation uuid.UUID
	var expectedHash, expectedRequest []byte
	if provider == "chat" {
		expectedOperation, expectedHash, expectedRequest = item.ChatOperationID, item.ChatRequestHash, item.ChatRequestBytes
	} else if provider == "voice" {
		expectedOperation, expectedHash, expectedRequest = item.VoiceOperationID, item.VoiceRequestHash, item.VoiceRequestBytes
	} else {
		return ErrMatchSquadConflict
	}
	if operationID != expectedOperation || !bytes.Equal(requestHash, expectedHash) || !bytes.Equal(request, expectedRequest) {
		return ErrMatchSquadConflict
	}
	if state == "COMPLETE" {
		if existingID == nil || *existingID != receiptID || !bytes.Equal(existingReceipt, receipt) {
			return ErrMatchSquadConflict
		}
		return nil
	}
	if state == "CONTRACT_MISMATCH" || existingID != nil || (state != "NOT_STARTED" && state != "IN_FLIGHT" && state != "RETRYABLE_FAILURE") {
		return ErrMatchSquadConflict
	}
	tag, err := tx.Exec(ctx, `
		UPDATE matchmaking_match_squad_teardown_participants
		SET state='COMPLETE',receipt_id=$3,receipt_bytes=$4,updated_at=clock_timestamp()
		WHERE aggregate_id=$1 AND provider=$2 AND state IN ('NOT_STARTED','IN_FLIGHT','RETRYABLE_FAILURE') AND receipt_id IS NULL
	`, item.AggregateID, provider, receiptID, receipt)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrMatchSquadConflict
	}
	return nil
}
