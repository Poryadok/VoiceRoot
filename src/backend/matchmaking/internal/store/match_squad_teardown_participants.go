package store

import (
	"bytes"
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// MatchSquadTeardownParticipant is one independently retriable, immutable
// provider request within the completion aggregate.
type MatchSquadTeardownParticipant struct {
	AggregateID     uuid.UUID
	Provider        string
	OperationID     uuid.UUID
	State           string
	RequestHash     []byte
	Request         []byte
	ReceiptID       *uuid.UUID
	Receipt         []byte
	AttemptCount    int
	NextAttemptAt   time.Time
	LastProgressAt  time.Time
	ClaimGeneration int64
	ClaimUntil      *time.Time
	LastErrorClass  string
}

type MatchSquadTeardownAlertState struct {
	OldestPendingProgressAgeSeconds float64
	ContractMismatchCount           int64
}

func (s *MatchStore) GetMatchSquadTeardownAlertState(ctx context.Context) (MatchSquadTeardownAlertState, error) {
	if s == nil || s.Pool == nil {
		return MatchSquadTeardownAlertState{}, errors.New("match store unavailable")
	}
	var state MatchSquadTeardownAlertState
	err := s.Pool.QueryRow(ctx, `
		SELECT COALESCE(MAX(EXTRACT(EPOCH FROM (clock_timestamp()-a.teardown_last_progress_at))),0),
		       COUNT(*) FILTER (WHERE p.state='CONTRACT_MISMATCH')
		FROM matchmaking_match_squad_teardowns a
		LEFT JOIN matchmaking_match_squad_teardown_participants p USING (aggregate_id)
		WHERE a.state='pending'
	`).Scan(&state.OldestPendingProgressAgeSeconds, &state.ContractMismatchCount)
	return state, err
}

func (s *MatchStore) ListPendingMatchSquadTeardownParticipants(ctx context.Context, limit int) ([]MatchSquadTeardownParticipant, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("match store unavailable")
	}
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT p.aggregate_id,p.provider,p.operation_id,p.state,p.request_sha256,p.request_bytes,p.receipt_id,p.receipt_bytes,
		       p.attempt_count,p.next_attempt_at,p.last_progress_at,p.claim_generation,p.claim_until,COALESCE(p.last_error_class,'')
		FROM matchmaking_match_squad_teardown_participants p
		JOIN matchmaking_match_squad_teardowns a USING (aggregate_id)
		WHERE a.state='pending' AND p.state IN ('NOT_STARTED','IN_FLIGHT','RETRYABLE_FAILURE')
		  AND p.next_attempt_at <= clock_timestamp()
		  AND (p.claim_until IS NULL OR p.claim_until <= clock_timestamp())
		ORDER BY p.created_at,p.aggregate_id,p.provider LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MatchSquadTeardownParticipant
	for rows.Next() {
		var item MatchSquadTeardownParticipant
		if err := rows.Scan(&item.AggregateID, &item.Provider, &item.OperationID, &item.State, &item.RequestHash, &item.Request, &item.ReceiptID, &item.Receipt,
			&item.AttemptCount, &item.NextAttemptAt, &item.LastProgressAt, &item.ClaimGeneration, &item.ClaimUntil, &item.LastErrorClass); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

// ClaimMatchSquadTeardownParticipant atomically takes one due provider operation.
// The claim lease is longer than the per-RPC deadline; an expired claim is safe
// to replay because the exact immutable operation ID and request are reused.
func (s *MatchStore) ClaimMatchSquadTeardownParticipant(ctx context.Context, item MatchSquadTeardownParticipant, lease time.Duration) (MatchSquadTeardownParticipant, bool, error) {
	if s == nil || s.Pool == nil || item.AggregateID == uuid.Nil ||
		(item.Provider != "chat" && item.Provider != "voice") || lease <= 0 || lease > time.Minute {
		return MatchSquadTeardownParticipant{}, false, ErrMatchSquadConflict
	}
	claimed := item
	err := s.Pool.QueryRow(ctx, `
		UPDATE matchmaking_match_squad_teardown_participants p
		SET state='IN_FLIGHT',attempt_count=p.attempt_count+1,
		    claim_generation=p.claim_generation+1,
	    claim_until=clock_timestamp()+($3 * interval '1 microsecond'),updated_at=clock_timestamp()
		FROM matchmaking_match_squad_teardowns a
		WHERE p.aggregate_id=$1 AND p.provider=$2 AND a.aggregate_id=p.aggregate_id AND a.state='pending'
		  AND p.state IN ('NOT_STARTED','IN_FLIGHT','RETRYABLE_FAILURE')
		  AND p.next_attempt_at <= clock_timestamp()
		  AND (p.claim_until IS NULL OR p.claim_until <= clock_timestamp())
		RETURNING p.aggregate_id,p.provider,p.operation_id,p.state,p.request_sha256,p.request_bytes,p.receipt_id,p.receipt_bytes,
		          p.attempt_count,p.next_attempt_at,p.last_progress_at,p.claim_generation,p.claim_until,COALESCE(p.last_error_class,'')
	`, item.AggregateID, item.Provider, lease.Microseconds()).Scan(
		&claimed.AggregateID, &claimed.Provider, &claimed.OperationID, &claimed.State, &claimed.RequestHash, &claimed.Request, &claimed.ReceiptID, &claimed.Receipt,
		&claimed.AttemptCount, &claimed.NextAttemptAt, &claimed.LastProgressAt, &claimed.ClaimGeneration, &claimed.ClaimUntil, &claimed.LastErrorClass,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return MatchSquadTeardownParticipant{}, false, nil
	}
	if err != nil {
		return MatchSquadTeardownParticipant{}, false, err
	}
	return claimed, true, nil
}

// ScheduleMatchSquadTeardownRetry records an availability failure and durable
// retry time without changing the immutable request or resetting progress time.
func (s *MatchStore) ScheduleMatchSquadTeardownRetry(ctx context.Context, item MatchSquadTeardownParticipant, delay time.Duration, errorClass string) error {
	if s == nil || s.Pool == nil || item.AggregateID == uuid.Nil || item.ClaimGeneration <= 0 ||
		(item.Provider != "chat" && item.Provider != "voice") || delay <= 0 || delay > 5*time.Minute || errorClass == "" {
		return ErrMatchSquadConflict
	}
	tag, err := s.Pool.Exec(ctx, `
		UPDATE matchmaking_match_squad_teardown_participants
		SET state='RETRYABLE_FAILURE',next_attempt_at=clock_timestamp()+($4 * interval '1 microsecond'),
		    last_error_class=$5,claim_until=NULL,updated_at=clock_timestamp()
		WHERE aggregate_id=$1 AND provider=$2 AND state='IN_FLIGHT' AND claim_generation=$3 AND receipt_id IS NULL
	`, item.AggregateID, item.Provider, item.ClaimGeneration, delay.Microseconds(), errorClass)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrMatchSquadConflict
	}
	return nil
}

func (s *MatchStore) MarkMatchSquadTeardownContractMismatch(ctx context.Context, item MatchSquadTeardownParticipant, errorClass string) error {
	if s == nil || s.Pool == nil || item.AggregateID == uuid.Nil || item.ClaimGeneration <= 0 ||
		(item.Provider != "chat" && item.Provider != "voice") || errorClass == "" {
		return ErrMatchSquadConflict
	}
	tag, err := s.Pool.Exec(ctx, `
		UPDATE matchmaking_match_squad_teardown_participants
		SET state='CONTRACT_MISMATCH',last_error_class=$4,claim_until=NULL,updated_at=clock_timestamp()
		WHERE aggregate_id=$1 AND provider=$2 AND state='IN_FLIGHT' AND claim_generation=$3 AND receipt_id IS NULL
	`, item.AggregateID, item.Provider, item.ClaimGeneration, errorClass)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrMatchSquadConflict
	}
	return nil
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
	switch provider {
	case "chat":
		expectedOperation, expectedHash, expectedRequest = item.ChatOperationID, item.ChatRequestHash, item.ChatRequestBytes
	case "voice":
		expectedOperation, expectedHash, expectedRequest = item.VoiceOperationID, item.VoiceRequestHash, item.VoiceRequestBytes
	default:
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
		SET state='COMPLETE',receipt_id=$3,receipt_bytes=$4,last_progress_at=clock_timestamp(),claim_until=NULL,updated_at=clock_timestamp()
		WHERE aggregate_id=$1 AND provider=$2 AND state IN ('NOT_STARTED','IN_FLIGHT','RETRYABLE_FAILURE') AND receipt_id IS NULL
	`, item.AggregateID, provider, receiptID, receipt)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrMatchSquadConflict
	}
	if _, err := tx.Exec(ctx, `
		UPDATE matchmaking_match_squad_teardowns SET teardown_last_progress_at=clock_timestamp(),updated_at=clock_timestamp()
		WHERE aggregate_id=$1 AND state='pending'
	`, item.AggregateID); err != nil {
		return err
	}
	return nil
}
