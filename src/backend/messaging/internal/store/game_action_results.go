package store

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var ErrGameActionResultConflict = errors.New("game action result conflicts with saved projection")

type GameActionResult struct {
	OperationID  uuid.UUID
	ActionID     string
	ResultID     uuid.UUID
	StateVersion string
	Status       string
	SafeSummary  string
	RecordedAt   time.Time
}

type ProjectGameActionResultRequest struct {
	MessageID uuid.UUID
	AppID     uuid.UUID
	EnvID     uuid.UUID
	Result    GameActionResult
}

type ProjectGameActionResultResponse struct {
	Result   GameActionResult
	Replayed bool
}

// ProjectGameActionResult stores the mutable terminal projection separately
// from the immutable event card. Exact retries are inert; conflicting retries
// and second operations for one action fail closed.
func (s *MessagesStore) ProjectGameActionResult(ctx context.Context, request ProjectGameActionResultRequest) (ProjectGameActionResultResponse, error) {
	if s == nil || s.Pool == nil || request.MessageID == uuid.Nil || request.AppID == uuid.Nil || request.EnvID == uuid.Nil ||
		request.Result.OperationID == uuid.Nil || request.Result.ResultID == uuid.Nil || request.Result.ActionID == "" ||
		len(request.Result.ActionID) > 128 || request.Result.StateVersion == "" || len(request.Result.StateVersion) > 256 ||
		(request.Result.Status != "succeeded" && request.Result.Status != "failed") || len(request.Result.SafeSummary) > 512 || strings.TrimSpace(request.Result.SafeSummary) != request.Result.SafeSummary {
		return ProjectGameActionResultResponse{}, errors.New("invalid game action result projection")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return ProjectGameActionResultResponse{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var card string
	err = tx.QueryRow(ctx, `SELECT c.card_json::text FROM message_game_cards c
		JOIN messages m ON m.id=c.message_id
		WHERE c.message_id=$1 AND c.app_id=$2 AND c.environment_id=$3 AND m.deleted_at IS NULL
		AND EXISTS (SELECT 1 FROM jsonb_array_elements(c.card_json->'actions') AS action WHERE action->>'action_id'=$4)
		FOR UPDATE OF c`, request.MessageID, request.AppID, request.EnvID, request.Result.ActionID).Scan(&card)
	if errors.Is(err, pgx.ErrNoRows) {
		return ProjectGameActionResultResponse{}, ErrGameActionResultConflict
	}
	if err != nil {
		return ProjectGameActionResultResponse{}, err
	}
	var saved GameActionResult
	err = tx.QueryRow(ctx, `SELECT operation_id,action_id,result_id,state_version,status,safe_summary,recorded_at
		FROM message_game_action_results WHERE message_id=$1 AND action_id=$2 FOR UPDATE`, request.MessageID, request.Result.ActionID).
		Scan(&saved.OperationID, &saved.ActionID, &saved.ResultID, &saved.StateVersion, &saved.Status, &saved.SafeSummary, &saved.RecordedAt)
	if err == nil {
		if saved.OperationID != request.Result.OperationID || saved.ActionID != request.Result.ActionID || saved.ResultID != request.Result.ResultID || saved.StateVersion != request.Result.StateVersion || saved.Status != request.Result.Status || saved.SafeSummary != request.Result.SafeSummary {
			return ProjectGameActionResultResponse{}, ErrGameActionResultConflict
		}
		if err := tx.Commit(ctx); err != nil {
			return ProjectGameActionResultResponse{}, err
		}
		return ProjectGameActionResultResponse{Result: saved, Replayed: true}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return ProjectGameActionResultResponse{}, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO message_game_action_results(message_id,action_id,operation_id,result_id,state_version,status,safe_summary)
		VALUES($1,$2,$3,$4,$5,$6,$7)`, request.MessageID, request.Result.ActionID, request.Result.OperationID, request.Result.ResultID, request.Result.StateVersion, request.Result.Status, request.Result.SafeSummary)
	if err != nil {
		return ProjectGameActionResultResponse{}, err
	}
	if err := tx.QueryRow(ctx, `SELECT operation_id,action_id,result_id,state_version,status,safe_summary,recorded_at FROM message_game_action_results WHERE message_id=$1 AND action_id=$2`, request.MessageID, request.Result.ActionID).
		Scan(&saved.OperationID, &saved.ActionID, &saved.ResultID, &saved.StateVersion, &saved.Status, &saved.SafeSummary, &saved.RecordedAt); err != nil {
		return ProjectGameActionResultResponse{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ProjectGameActionResultResponse{}, err
	}
	return ProjectGameActionResultResponse{Result: saved}, nil
}

func (s *MessagesStore) GameActionResults(ctx context.Context, messageIDs []uuid.UUID) (map[uuid.UUID][]GameActionResult, error) {
	result := make(map[uuid.UUID][]GameActionResult, len(messageIDs))
	if len(messageIDs) == 0 {
		return result, nil
	}
	rows, err := s.Pool.Query(ctx, `SELECT message_id,operation_id,action_id,result_id,state_version,status,safe_summary,recorded_at
		FROM message_game_action_results WHERE message_id=ANY($1) ORDER BY recorded_at,action_id`, messageIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var messageID uuid.UUID
		var item GameActionResult
		if err := rows.Scan(&messageID, &item.OperationID, &item.ActionID, &item.ResultID, &item.StateVersion, &item.Status, &item.SafeSummary, &item.RecordedAt); err != nil {
			return nil, err
		}
		result[messageID] = append(result[messageID], item)
	}
	return result, rows.Err()
}
