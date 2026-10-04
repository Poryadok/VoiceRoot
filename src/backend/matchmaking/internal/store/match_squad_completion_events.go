package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// MatchSquadCompletionEvent is the immutable MatchCompleted payload waiting
// for post-commit JetStream delivery.
type MatchSquadCompletionEvent struct {
	AggregateID     uuid.UUID
	EventID         uuid.UUID
	OccurredAt      time.Time
	MatchID         uuid.UUID
	DurationSeconds int64
	ProfileIDs      []string
}

func ensureMatchSquadCompletionEventTx(ctx context.Context, tx pgx.Tx, aggregateID uuid.UUID, matchID uuid.UUID) error {
	match, err := scanMatch(tx.QueryRow(ctx, `
		SELECT id, game_id, mode, region, participants, left_profile_ids, voice_room_id, chat_id, status, created_at, completed_at
		FROM matches WHERE id=$1
	`, matchID))
	if err != nil {
		return err
	}
	if match.CompletedAt == nil || match.Status != MatchStatusCompleted {
		return ErrMatchSquadConflict
	}
	occurredAt := *match.CompletedAt
	profileIDs := make([]string, 0, len(match.ProfileIDs()))
	for _, id := range match.ProfileIDs() {
		profileIDs = append(profileIDs, id.String())
	}
	encoded, err := json.Marshal(profileIDs)
	if err != nil {
		return err
	}
	eventID := uuid.New()
	duration := int64(occurredAt.Sub(match.CreatedAt).Seconds())
	if duration < 0 {
		return ErrMatchSquadConflict
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO matchmaking_match_squad_completion_events
		(aggregate_id,event_id,occurred_at,duration_seconds,profile_ids)
		VALUES ($1,$2,$3,$4,$5::jsonb) ON CONFLICT (aggregate_id) DO NOTHING
	`, aggregateID, eventID, occurredAt, duration, encoded)
	if err != nil {
		return err
	}
	var storedID uuid.UUID
	var storedAt time.Time
	var storedDuration int64
	var storedProfiles []byte
	if err := tx.QueryRow(ctx, `SELECT event_id,occurred_at,duration_seconds,profile_ids FROM matchmaking_match_squad_completion_events WHERE aggregate_id=$1`, aggregateID).Scan(&storedID, &storedAt, &storedDuration, &storedProfiles); err != nil {
		return err
	}
	if storedID != eventID {
		// A pre-existing outbox row is valid only when this transaction is an
		// idempotent retry with the exact original completion facts. Its stable
		// event ID is intentionally retained for broker deduplication.
		if !storedAt.Equal(occurredAt) || storedDuration != duration || string(storedProfiles) != string(encoded) {
			return ErrMatchSquadConflict
		}
	}
	return nil
}

func (s *MatchStore) ListPendingMatchSquadCompletionEvents(ctx context.Context, limit int) ([]MatchSquadCompletionEvent, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("match store unavailable")
	}
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT aggregate_id,event_id,occurred_at,duration_seconds,profile_ids
		FROM matchmaking_match_squad_completion_events WHERE published_at IS NULL
		ORDER BY created_at,aggregate_id LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MatchSquadCompletionEvent
	for rows.Next() {
		var item MatchSquadCompletionEvent
		var profiles []byte
		if err := rows.Scan(&item.AggregateID, &item.EventID, &item.OccurredAt, &item.DurationSeconds, &profiles); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(profiles, &item.ProfileIDs); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *MatchStore) MarkMatchSquadCompletionEventPublished(ctx context.Context, aggregateID, eventID uuid.UUID) error {
	if s == nil || s.Pool == nil {
		return errors.New("match store unavailable")
	}
	if aggregateID == uuid.Nil || eventID == uuid.Nil {
		return ErrMatchSquadConflict
	}
	tag, err := s.Pool.Exec(ctx, `
		UPDATE matchmaking_match_squad_completion_events SET published_at=clock_timestamp()
		WHERE aggregate_id=$1 AND event_id=$2 AND published_at IS NULL
	`, aggregateID, eventID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		var existing uuid.UUID
		if err := s.Pool.QueryRow(ctx, `SELECT event_id FROM matchmaking_match_squad_completion_events WHERE aggregate_id=$1`, aggregateID).Scan(&existing); err != nil {
			return err
		}
		if existing != eventID {
			return ErrMatchSquadConflict
		}
	}
	return nil
}
