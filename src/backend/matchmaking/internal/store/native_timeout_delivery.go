package store

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"time"
)

var ErrNativeTimeoutLeaseLost = errors.New("native timeout delivery lease changed")

type NativeTimeoutDelivery struct {
	EventID, LeaseToken uuid.UUID
	Payload             []byte
}

func (s *MatchStore) ClaimNativeTimeouts(ctx context.Context, limit int, lease time.Duration) ([]NativeTimeoutDelivery, error) {
	if s == nil || s.Pool == nil || ctx == nil || limit < 1 || limit > 100 || lease <= 0 || lease > time.Minute {
		return nil, ErrSourcePartyUnavailable
	}
	rows, err := s.Pool.Query(ctx, `WITH candidates AS (
 SELECT event_id FROM matchmaking_native_timeout_outbox
 WHERE delivered_at IS NULL AND next_attempt_at<=clock_timestamp() AND (lease_until IS NULL OR lease_until<=clock_timestamp())
 ORDER BY next_attempt_at,event_id FOR UPDATE SKIP LOCKED LIMIT $1
 ) UPDATE matchmaking_native_timeout_outbox o SET lease_token=$2,lease_until=clock_timestamp()+make_interval(secs=>$3)
 FROM candidates c WHERE o.event_id=c.event_id RETURNING o.event_id,o.lease_token,o.payload`, limit, uuid.New(), lease.Seconds())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []NativeTimeoutDelivery
	for rows.Next() {
		var fact NativeTimeoutDelivery
		if err = rows.Scan(&fact.EventID, &fact.LeaseToken, &fact.Payload); err != nil {
			return nil, err
		}
		result = append(result, fact)
	}
	return result, rows.Err()
}

func (s *MatchStore) FinishNativeTimeout(ctx context.Context, fact NativeTimeoutDelivery, success bool) error {
	if s == nil || s.Pool == nil || ctx == nil || fact.EventID == uuid.Nil || fact.LeaseToken == uuid.Nil {
		return ErrSourcePartyUnavailable
	}
	tag, err := s.Pool.Exec(ctx, `UPDATE matchmaking_native_timeout_outbox SET
 delivered_at=CASE WHEN $3 THEN clock_timestamp() ELSE NULL END,
 lease_token=NULL,lease_until=NULL,next_attempt_at=clock_timestamp()+interval '1 second'
 WHERE event_id=$1 AND lease_token=$2 AND lease_until>clock_timestamp() AND delivered_at IS NULL`, fact.EventID, fact.LeaseToken, success)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrNativeTimeoutLeaseLost
	}
	return nil
}
