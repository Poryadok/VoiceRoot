package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// SearchProjectionOutboxRecord is one immutable journal payload awaiting a
// JetStream PubAck. Payload is deliberately returned verbatim.
type SearchProjectionOutboxRecord struct {
	EventID       uuid.UUID
	JournalOffset uint64
	Payload       []byte
}

// ClaimSearchProjectionOutbox claims the first available record. A lease makes
// competing User replicas preserve journal order while a crashed owner becomes
// retryable after leaseUntil.
func (s *ProfileStore) ClaimSearchProjectionOutbox(ctx context.Context, owner string, leaseUntil time.Time) (*SearchProjectionOutboxRecord, error) {
	var record SearchProjectionOutboxRecord
	err := s.pool.QueryRow(ctx, `WITH candidate AS (
		SELECT event_id FROM user_profile_search_outbox
		WHERE delivered_at IS NULL
		-- Never skip an earlier live lease: publishing N+1 first would let
		-- Search advance past a retained-but-unpublished N permanently.
		ORDER BY journal_offset FOR UPDATE LIMIT 1
	)
	UPDATE user_profile_search_outbox o
	SET lease_owner = $1, leased_until = $2
	FROM candidate c WHERE o.event_id = c.event_id
		AND (o.leased_until IS NULL OR o.leased_until < now())
	RETURNING o.event_id, o.journal_offset, o.payload`, owner, leaseUntil).
		Scan(&record.EventID, &record.JournalOffset, &record.Payload)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &record, nil
}

func (s *ProfileStore) MarkSearchProjectionOutboxDelivered(ctx context.Context, eventID uuid.UUID, owner string) error {
	_, err := s.pool.Exec(ctx, `UPDATE user_profile_search_outbox
		SET delivered_at = now(), leased_until = NULL, lease_owner = NULL
		WHERE event_id = $1 AND lease_owner = $2`, eventID, owner)
	return err
}

func (s *ProfileStore) ReleaseSearchProjectionOutboxLease(ctx context.Context, eventID uuid.UUID, owner string) error {
	_, err := s.pool.Exec(ctx, `UPDATE user_profile_search_outbox
		SET leased_until = NULL, lease_owner = NULL
		WHERE event_id = $1 AND lease_owner = $2`, eventID, owner)
	return err
}
