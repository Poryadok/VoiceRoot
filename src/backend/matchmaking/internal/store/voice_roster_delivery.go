package store

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var ErrRosterCancellationLease = errors.New("roster cancellation outbox lease changed")

type VoiceRosterCancellation struct {
	EventID, LeaseToken uuid.UUID
	Payload             []byte
}

func (s *MatchStore) ClaimVoiceRosterCancellation(ctx context.Context) (*VoiceRosterCancellation, error) {
	var item VoiceRosterCancellation
	err := s.Pool.QueryRow(ctx, `UPDATE matchmaking_voice_roster_cancellation_outbox SET lease_token=$1,lease_until=clock_timestamp()+interval '15 seconds'
WHERE event_id=(SELECT event_id FROM matchmaking_voice_roster_cancellation_outbox WHERE delivered_at IS NULL AND next_attempt_at<=clock_timestamp() AND (lease_until IS NULL OR lease_until<=clock_timestamp()) ORDER BY occurred_at,event_id FOR UPDATE SKIP LOCKED LIMIT 1)
RETURNING event_id,lease_token,payload`, uuid.New()).Scan(&item.EventID, &item.LeaseToken, &item.Payload)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &item, err
}
func (s *MatchStore) FinishVoiceRosterCancellation(ctx context.Context, item VoiceRosterCancellation, delivered bool) error {
	tag, err := s.Pool.Exec(ctx, `UPDATE matchmaking_voice_roster_cancellation_outbox SET delivered_at=CASE WHEN $3 THEN clock_timestamp() ELSE NULL END,lease_token=NULL,lease_until=NULL,next_attempt_at=clock_timestamp()+interval '5 seconds' WHERE event_id=$1 AND lease_token=$2 AND delivered_at IS NULL AND lease_until>clock_timestamp()`, item.EventID, item.LeaseToken, delivered)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrRosterCancellationLease
	}
	return nil
}
func (s *MatchStore) VoiceRosterContractMismatchCount(ctx context.Context) (int64, error) {
	var count int64
	err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM matchmaking_voice_roster_contract_conflicts`).Scan(&count)
	return count, err
}
