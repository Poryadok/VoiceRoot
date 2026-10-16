package store

import (
	"context"
	"errors"
)

// ListAcceptedMatchesMissingProvisioningIntent finds the narrow crash window
// after the final response commits but before the provider saga saves intent.
func (s *MatchStore) ListAcceptedMatchesMissingProvisioningIntent(ctx context.Context, limit int) ([]Match, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("match store unavailable")
	}
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT m.id, m.game_id, m.mode, m.region, m.participants, m.left_profile_ids,
		       m.voice_room_id, m.chat_id, m.status, m.created_at, m.completed_at
		FROM matches m
		WHERE m.status = $1
		  AND EXISTS (SELECT 1 FROM match_proposals p WHERE p.match_id=m.id)
		  AND NOT EXISTS (SELECT 1 FROM match_proposals p WHERE p.match_id=m.id AND p.response <> $2)
		  AND NOT EXISTS (SELECT 1 FROM matchmaking_match_squad_operations o WHERE o.match_id=m.id)
		ORDER BY m.created_at, m.id LIMIT $3
	`, MatchStatusPendingAccept, ProposalResponseAccepted, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var matches []Match
	for rows.Next() {
		match, err := scanMatch(rows)
		if err != nil {
			return nil, err
		}
		matches = append(matches, match)
	}
	return matches, rows.Err()
}

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
