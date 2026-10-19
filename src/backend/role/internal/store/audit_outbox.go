package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

// RoleAuditEffect contains the committed exact request and its current lease.
type RoleAuditEffect struct {
	EventID      uuid.UUID
	LeaseToken   uuid.UUID
	RequestBytes []byte
}

// EnqueueAuditEffect never starts a separate transaction or falls back to Pool.
// The verified actor and validated immutable request are supplied by the domain
// writer after authorization, in its already fenced ordinary transaction.
func (s *RoleStore) EnqueueAuditEffect(ctx context.Context, eventID, spaceID uuid.UUID, request []byte) error {
	if s == nil || s.tx == nil || eventID == uuid.Nil || spaceID == uuid.Nil || len(request) == 0 {
		return errors.New("role audit enqueue requires a scoped transaction and immutable request")
	}
	if _, ok := s.scopeSpaces[spaceID]; !ok {
		return errors.New("role audit enqueue outside transaction Space scope")
	}
	_, err := s.tx.Exec(ctx, `INSERT INTO role_space_audit_outbox(audit_event_id,space_id,request_bytes) VALUES($1,$2,$3)`, eventID, spaceID, request)
	return err
}

func (s *RoleStore) ClaimAuditEffects(ctx context.Context, limit int, lease time.Duration) ([]RoleAuditEffect, error) {
	if s == nil || s.Pool == nil || limit < 1 || limit > 100 || lease.Microseconds() < 1 {
		return nil, errors.New("invalid role audit claim")
	}
	rows, err := s.Pool.Query(ctx, `
WITH candidates AS (
 SELECT audit_event_id FROM role_space_audit_outbox
 WHERE delivered_at IS NULL AND next_attempt_at<=clock_timestamp()
   AND (lease_expires_at IS NULL OR lease_expires_at<=clock_timestamp())
 ORDER BY created_at,audit_event_id FOR UPDATE SKIP LOCKED LIMIT $1
)
UPDATE role_space_audit_outbox AS outbox
SET lease_token=gen_random_uuid(), lease_expires_at=clock_timestamp()+interval '1 microsecond'*$2
FROM candidates WHERE outbox.audit_event_id=candidates.audit_event_id
RETURNING outbox.audit_event_id,outbox.lease_token,outbox.request_bytes`, limit, lease.Microseconds())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var effects []RoleAuditEffect
	for rows.Next() {
		var effect RoleAuditEffect
		if err := rows.Scan(&effect.EventID, &effect.LeaseToken, &effect.RequestBytes); err != nil {
			return nil, err
		}
		effects = append(effects, effect)
	}
	return effects, rows.Err()
}

func (s *RoleStore) CompleteAuditEffect(ctx context.Context, eventID, token uuid.UUID) (bool, error) {
	if s == nil || s.Pool == nil || eventID == uuid.Nil || token == uuid.Nil {
		return false, errors.New("invalid role audit completion")
	}
	tag, err := s.Pool.Exec(ctx, `UPDATE role_space_audit_outbox SET delivered_at=clock_timestamp(),lease_token=NULL,lease_expires_at=NULL WHERE audit_event_id=$1 AND lease_token=$2 AND delivered_at IS NULL`, eventID, token)
	return tag.RowsAffected() == 1, err
}

func (s *RoleStore) RetryAuditEffect(ctx context.Context, eventID, token uuid.UUID) (bool, error) {
	if s == nil || s.Pool == nil || eventID == uuid.Nil || token == uuid.Nil {
		return false, errors.New("invalid role audit retry")
	}
	tag, err := s.Pool.Exec(ctx, `UPDATE role_space_audit_outbox SET attempt_count=LEAST(attempt_count::bigint+1,2147483647)::integer,next_attempt_at=clock_timestamp()+make_interval(secs => power(2.0,LEAST(attempt_count,9))::integer),lease_token=NULL,lease_expires_at=NULL WHERE audit_event_id=$1 AND lease_token=$2 AND delivered_at IS NULL`, eventID, token)
	return tag.RowsAffected() == 1, err
}
