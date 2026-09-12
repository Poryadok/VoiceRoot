package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// AuditOutboxEvent is one leased, immutable audit effect. AuditEventID is the
// stable transport dedupe key and is identical to audit_log.id.
type AuditOutboxEvent struct {
	AuditEventID   uuid.UUID
	SpaceID        uuid.UUID
	ActorProfileID uuid.UUID
	Action         string
	TargetType     string
	TargetID       uuid.UUID
	DetailsJSON    string
	CreatedAt      time.Time
	LeaseToken     uuid.UUID
	LeaseExpiresAt time.Time
}

// ClaimAuditOutbox leases due rows in deterministic order. Concurrent workers
// use SKIP LOCKED and therefore cannot hold the same unexpired lease.
func (s *SpaceStore) ClaimAuditOutbox(ctx context.Context, limit int, leaseDuration time.Duration) ([]*AuditOutboxEvent, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("space store: pool not configured")
	}
	if limit < 1 || leaseDuration.Microseconds() < 1 {
		return nil, errors.New("invalid audit outbox claim")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin audit outbox claim: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	rows, err := tx.Query(ctx, `
WITH candidates AS (
	SELECT audit_event_id
	FROM audit_outbox
	WHERE delivered_at IS NULL
	  AND next_attempt_at <= clock_timestamp()
	  AND (lease_expires_at IS NULL OR lease_expires_at <= clock_timestamp())
	ORDER BY created_at,audit_event_id
	FOR UPDATE SKIP LOCKED
	LIMIT $1
), claimed AS (
	UPDATE audit_outbox AS outbox
	SET lease_token=gen_random_uuid(),
		lease_expires_at=clock_timestamp()+interval '1 microsecond' * $2
	FROM candidates
	WHERE outbox.audit_event_id=candidates.audit_event_id
	RETURNING outbox.audit_event_id,outbox.lease_token,outbox.lease_expires_at
)
SELECT audit.id,audit.space_id,audit.actor_profile_id,audit.action,audit.target_type,
	audit.target_id,audit.details::text,audit.created_at,claimed.lease_token,claimed.lease_expires_at
FROM claimed
JOIN audit_log AS audit ON audit.id=claimed.audit_event_id
ORDER BY audit.created_at,audit.id`, limit, leaseDuration.Microseconds())
	if err != nil {
		return nil, fmt.Errorf("claim audit outbox: %w", err)
	}
	defer rows.Close()
	result := make([]*AuditOutboxEvent, 0, limit)
	for rows.Next() {
		event := new(AuditOutboxEvent)
		if err := rows.Scan(&event.AuditEventID, &event.SpaceID, &event.ActorProfileID, &event.Action,
			&event.TargetType, &event.TargetID, &event.DetailsJSON, &event.CreatedAt,
			&event.LeaseToken, &event.LeaseExpiresAt); err != nil {
			return nil, fmt.Errorf("scan claimed audit outbox: %w", err)
		}
		event.CreatedAt = event.CreatedAt.UTC()
		event.LeaseExpiresAt = event.LeaseExpiresAt.UTC()
		result = append(result, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read claimed audit outbox: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit audit outbox claim: %w", err)
	}
	return result, nil
}

// MarkAuditOutboxDelivered acknowledges only the worker holding the current
// lease. Repeated or stale acknowledgements have no effect.
func (s *SpaceStore) MarkAuditOutboxDelivered(ctx context.Context, auditEventID, leaseToken uuid.UUID) (bool, error) {
	if s == nil || s.Pool == nil || auditEventID == uuid.Nil || leaseToken == uuid.Nil {
		return false, errors.New("invalid audit outbox delivery mark")
	}
	tag, err := s.Pool.Exec(ctx, `
UPDATE audit_outbox
SET delivered_at=clock_timestamp(),lease_token=NULL,lease_expires_at=NULL
WHERE audit_event_id=$1 AND lease_token=$2 AND delivered_at IS NULL`, auditEventID, leaseToken)
	if err != nil {
		return false, fmt.Errorf("mark audit outbox delivered: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// MarkAuditOutboxFailed releases the current lease and schedules a bounded
// exponential retry while retaining the same audit_event_id.
func (s *SpaceStore) MarkAuditOutboxFailed(ctx context.Context, auditEventID, leaseToken uuid.UUID) (bool, error) {
	if s == nil || s.Pool == nil || auditEventID == uuid.Nil || leaseToken == uuid.Nil {
		return false, errors.New("invalid audit outbox failure mark")
	}
	tag, err := s.Pool.Exec(ctx, `
UPDATE audit_outbox AS outbox
SET attempt_count=outbox.attempt_count+1,
	last_failure_at=clock_timestamp(),
	next_attempt_at=clock_timestamp()+make_interval(secs => power(2.0,LEAST(outbox.attempt_count,9))::integer),
	lease_token=NULL,
	lease_expires_at=NULL
WHERE outbox.audit_event_id=$1 AND outbox.lease_token=$2 AND outbox.delivered_at IS NULL`, auditEventID, leaseToken)
	if err != nil {
		return false, fmt.Errorf("mark audit outbox failed: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}
