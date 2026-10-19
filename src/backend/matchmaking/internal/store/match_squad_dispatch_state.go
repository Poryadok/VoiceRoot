package store

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// MatchSquadDispatchState is private durable coordination metadata, never a
// provider absence receipt or an authorization to activate a match.
type MatchSquadDispatchState struct {
	Disposition    string
	Chat           string
	Voice          string
	OperationState string
	MatchStatus    string
}

func (s *MatchStore) GetMatchSquadDispatchState(ctx context.Context, matchID uuid.UUID) (MatchSquadDispatchState, error) {
	if s == nil || s.Pool == nil || matchID == uuid.Nil {
		return MatchSquadDispatchState{}, ErrSourcePartyUnavailable
	}
	var state MatchSquadDispatchState
	err := s.Pool.QueryRow(ctx, `SELECT COALESCE(to_jsonb(o)->>'provision_disposition',''),
		COALESCE(to_jsonb(o)->>'chat_dispatch_state',''),COALESCE(to_jsonb(o)->>'voice_dispatch_state',''),o.state,m.status
		FROM matchmaking_match_squad_operations o JOIN matches m ON m.id=o.match_id WHERE o.match_id=$1`, matchID).
		Scan(&state.Disposition, &state.Chat, &state.Voice, &state.OperationState, &state.MatchStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return MatchSquadDispatchState{}, ErrMatchSquadPending
	}
	if err != nil {
		return MatchSquadDispatchState{}, err
	}
	if state.Disposition == "" || state.Chat == "" || state.Voice == "" {
		return MatchSquadDispatchState{}, ErrSourcePartyUnavailable
	}
	return state, nil
}
