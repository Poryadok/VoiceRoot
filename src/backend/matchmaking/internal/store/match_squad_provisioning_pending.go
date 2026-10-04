package store

import (
	"context"
	"errors"
)

// ListPendingMatchSquadProvisioningOperations returns committed provider
// operations whose exact requests still need a receipt or match activation.
func (s *MatchStore) ListPendingMatchSquadProvisioningOperations(ctx context.Context, limit int) ([]MatchSquadProvisioningOperation, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("match store unavailable")
	}
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT match_id, operation_id, participant_manifest_sha256, participant_manifest_bytes, state,
		       chat_operation_id, chat_request_sha256, chat_request_bytes, chat_receipt_id, chat_receipt_bytes, chat_id,
		       voice_operation_id, voice_request_sha256, voice_request_bytes, voice_receipt_id, voice_receipt_bytes, voice_room_id
		FROM matchmaking_match_squad_operations
		WHERE state='provisioning'
		ORDER BY created_at,match_id LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var operations []MatchSquadProvisioningOperation
	for rows.Next() {
		operation, err := scanMatchSquadProvisioningOperation(rows)
		if err != nil {
			return nil, err
		}
		operations = append(operations, operation)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return operations, nil
}
