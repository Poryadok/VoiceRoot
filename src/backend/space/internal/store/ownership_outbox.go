package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ClaimedOwnershipOutboxEvent is immutable event data paired with a temporary
// fencing token. The token changes on every lease acquisition; the event data
// does not.
type ClaimedOwnershipOutboxEvent struct {
	EventID    uuid.UUID
	SpaceID    uuid.UUID
	EventType  string
	CreatedAt  time.Time
	LeaseToken uuid.UUID
}

// ClaimReadyOwnershipOutbox atomically leases ready events in stable delivery
// order. The transaction is committed before claimed events are returned, so a
// caller cannot perform a network publish inside the claim transaction.
func (s *SpaceStore) ClaimReadyOwnershipOutbox(ctx context.Context, limit int) ([]ClaimedOwnershipOutboxEvent, error) {
	if limit < 1 || limit > 100 {
		return nil, errors.New("invalid ownership outbox claim limit")
	}
	if s == nil || s.Pool == nil || s.tx != nil {
		return nil, errors.New("space store: claim requires configured root pool")
	}

	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin ownership outbox claim: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	rows, err := tx.Query(ctx, `WITH candidates AS (
		SELECT event_id
		FROM ownership_outbox
		WHERE ready
		  AND delivered_at IS NULL
		  AND next_attempt_at <= clock_timestamp()
		  AND (lease_expires_at IS NULL OR lease_expires_at <= clock_timestamp())
		ORDER BY created_at,event_id
		FOR UPDATE SKIP LOCKED
		LIMIT $1
	), claimed AS (
		UPDATE ownership_outbox AS outbox
		SET lease_token=gen_random_uuid(),
			lease_expires_at=clock_timestamp()+interval '30 seconds'
		FROM candidates
		WHERE outbox.event_id=candidates.event_id
		RETURNING outbox.event_id,outbox.space_id,outbox.event_type,outbox.created_at,outbox.lease_token
	)
	SELECT event_id,space_id,event_type,created_at,lease_token
	FROM claimed
	ORDER BY created_at,event_id`, limit)
	if err != nil {
		return nil, fmt.Errorf("claim ownership outbox: %w", err)
	}
	defer rows.Close()

	claimed := make([]ClaimedOwnershipOutboxEvent, 0, limit)
	for rows.Next() {
		var event ClaimedOwnershipOutboxEvent
		if err := rows.Scan(&event.EventID, &event.SpaceID, &event.EventType, &event.CreatedAt, &event.LeaseToken); err != nil {
			return nil, fmt.Errorf("scan claimed ownership outbox: %w", err)
		}
		event.CreatedAt = event.CreatedAt.UTC()
		claimed = append(claimed, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read claimed ownership outbox: %w", err)
	}
	rows.Close()
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit ownership outbox claim: %w", err)
	}
	return claimed, nil
}

// MarkOwnershipOutboxFailed releases a matching lease and schedules the next
// attempt from PostgreSQL time. A stale fencing token cannot change the row.
func (s *SpaceStore) MarkOwnershipOutboxFailed(ctx context.Context, eventID, leaseToken uuid.UUID) (bool, error) {
	if eventID == uuid.Nil || leaseToken == uuid.Nil {
		return false, errors.New("invalid ownership outbox failure mark")
	}
	if s == nil || s.Pool == nil {
		return false, errors.New("space store: pool not configured")
	}
	command, err := s.Pool.Exec(ctx, `WITH failure_clock AS (
		SELECT clock_timestamp() AS failed_at
	)
	UPDATE ownership_outbox AS outbox
	SET attempt_count=outbox.attempt_count+1,
		last_failure_at=failure_clock.failed_at,
		next_attempt_at=failure_clock.failed_at+make_interval(secs => LEAST(
			300.0,
			power(2.0, LEAST(outbox.attempt_count, 9))*(0.8+random()*0.4)
		)),
		lease_token=NULL,
		lease_expires_at=NULL
	FROM failure_clock
	WHERE outbox.event_id=$1
	  AND outbox.lease_token=$2
	  AND outbox.delivered_at IS NULL`, eventID, leaseToken)
	if err != nil {
		return false, fmt.Errorf("mark ownership outbox failed: %w", err)
	}
	return command.RowsAffected() == 1, nil
}

// MarkOwnershipOutboxDelivered records delivery without deleting the row. Both
// the stable event ID and current lease token are required for the CAS update.
func (s *SpaceStore) MarkOwnershipOutboxDelivered(ctx context.Context, eventID, leaseToken uuid.UUID) (bool, error) {
	if eventID == uuid.Nil || leaseToken == uuid.Nil {
		return false, errors.New("invalid ownership outbox delivery mark")
	}
	if s == nil || s.Pool == nil {
		return false, errors.New("space store: pool not configured")
	}
	command, err := s.Pool.Exec(ctx, `UPDATE ownership_outbox
		SET ready=FALSE,delivered_at=clock_timestamp(),lease_token=NULL,lease_expires_at=NULL
		WHERE event_id=$1 AND lease_token=$2 AND delivered_at IS NULL`, eventID, leaseToken)
	if err != nil {
		return false, fmt.Errorf("mark ownership outbox delivered: %w", err)
	}
	return command.RowsAffected() == 1, nil
}

// CleanupDeliveredOwnershipOutbox removes a bounded batch of terminal delivery
// evidence after the documented retention period. transaction_timestamp is
// intentionally stable for callers that provide an existing transaction.
func (s *SpaceStore) CleanupDeliveredOwnershipOutbox(ctx context.Context, limit int) (int64, error) {
	if limit < 1 || limit > 100 {
		return 0, errors.New("invalid ownership outbox cleanup limit")
	}
	if s == nil || (s.Pool == nil && s.tx == nil) {
		return 0, errors.New("space store: database not configured")
	}
	command, err := s.db().Exec(ctx, `WITH candidates AS (
		SELECT event_id
		FROM ownership_outbox
		WHERE NOT ready
		  AND delivered_at IS NOT NULL
		  AND lease_token IS NULL
		  AND lease_expires_at IS NULL
		  AND delivered_at <= transaction_timestamp()-interval '30 days'
		ORDER BY delivered_at,event_id
		FOR UPDATE SKIP LOCKED
		LIMIT $1
	)
	DELETE FROM ownership_outbox AS outbox
	USING candidates
	WHERE outbox.event_id=candidates.event_id`, limit)
	if err != nil {
		return 0, fmt.Errorf("cleanup delivered ownership outbox: %w", err)
	}
	return command.RowsAffected(), nil
}

// ReadReadyOwnershipOutbox visits immutable ready events in stable delivery order.
// Claiming, acknowledgement, deletion and publishing belong to the dispatcher slice.
func (s *SpaceStore) ReadReadyOwnershipOutbox(ctx context.Context, limit int, visit func(eventID, operationID, spaceID, previousOwnerProfileID, newOwnerProfileID uuid.UUID, eventType string, createdAt time.Time) error) error {
	if limit < 1 || limit > 100 || visit == nil {
		return errors.New("invalid ownership outbox read")
	}
	if s == nil || s.Pool == nil {
		return errors.New("space store: pool not configured")
	}
	rows, err := s.Pool.Query(ctx, `SELECT event_id,operation_id,space_id,previous_owner_profile_id,new_owner_profile_id,event_type,created_at
		FROM ownership_outbox WHERE ready ORDER BY created_at,event_id LIMIT $1`, limit)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var eventID, operationID, spaceID, previousOwnerID, newOwnerID uuid.UUID
		var eventType string
		var createdAt time.Time
		if err := rows.Scan(&eventID, &operationID, &spaceID, &previousOwnerID, &newOwnerID, &eventType, &createdAt); err != nil {
			return err
		}
		if err := visit(eventID, operationID, spaceID, previousOwnerID, newOwnerID, eventType, createdAt.UTC()); err != nil {
			return err
		}
	}
	return rows.Err()
}
