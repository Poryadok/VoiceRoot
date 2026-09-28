package registry

import (
	"context"
	"crypto/sha256"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type SessionActiveEvent struct {
	EventID       uuid.UUID `json:"event_id"`
	ApplicationID uuid.UUID `json:"application_id"`
	EnvironmentID uuid.UUID `json:"environment_id"`
	SessionID     uuid.UUID `json:"session_id"`
	OperationID   uuid.UUID `json:"operation_id"`
	Kind          string    `json:"kind"`
	ActiveAt      time.Time `json:"active_at"`
}

type SessionEventClaim struct {
	EventID        uuid.UUID
	ApplicationID  uuid.UUID
	EnvironmentID  uuid.UUID
	SessionID      uuid.UUID
	PayloadBytes   []byte
	PayloadSHA256  []byte
	LeaseID        uuid.UUID
	LeaseExpiresAt time.Time
}

func (o *SessionOrchestrator) ClaimSessionEvent(ctx context.Context, principal SessionPrincipal) (*SessionEventClaim, error) {
	if o == nil || o.Store == nil {
		return nil, errors.New("session orchestrator is not configured")
	}
	return o.Store.ClaimSessionEvent(ctx, principal)
}

func (o *SessionOrchestrator) AckSessionEvent(ctx context.Context, principal SessionPrincipal, eventID, leaseID uuid.UUID, digest []byte) error {
	if o == nil || o.Store == nil {
		return errors.New("session orchestrator is not configured")
	}
	return o.Store.AckSessionEvent(ctx, principal, eventID, leaseID, digest)
}

// ClaimSessionEvent leases one undelivered event in the authenticated app/env.
// Filtering scope before row locking prevents a consumer from locking foreign work.
func (s *Store) ClaimSessionEvent(ctx context.Context, principal SessionPrincipal) (*SessionEventClaim, error) {
	if s == nil || s.Pool == nil || principal.ApplicationID == uuid.Nil || principal.EnvironmentID == uuid.Nil {
		return nil, errors.New("session event store and app/environment are required")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	claim := &SessionEventClaim{}
	err = tx.QueryRow(ctx, `SELECT event_id,application_id,environment_id,session_id,payload_bytes,payload_sha256
		FROM gis_session_outbox
		WHERE application_id=$1 AND environment_id=$2 AND delivered_at IS NULL
		  AND (claim_lease_until IS NULL OR claim_lease_until<=clock_timestamp())
		ORDER BY created_at,event_id
		FOR UPDATE SKIP LOCKED LIMIT 1`, principal.ApplicationID, principal.EnvironmentID).Scan(
		&claim.EventID, &claim.ApplicationID, &claim.EnvironmentID, &claim.SessionID, &claim.PayloadBytes, &claim.PayloadSHA256)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(claim.PayloadBytes)
	if len(claim.PayloadSHA256) != sha256.Size || !equalBytes(digest[:], claim.PayloadSHA256) {
		return nil, errors.New("session event outbox payload digest is corrupt")
	}
	claim.LeaseID = uuid.New()
	err = tx.QueryRow(ctx, `UPDATE gis_session_outbox
		SET claim_lease_id=$2,claim_lease_until=clock_timestamp()+interval '30 seconds',attempts=attempts+1
		WHERE event_id=$1 RETURNING claim_lease_until`, claim.EventID, claim.LeaseID).Scan(&claim.LeaseExpiresAt)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	claim.LeaseExpiresAt = claim.LeaseExpiresAt.UTC()
	return claim, nil
}

// AckSessionEvent records delivery only after the independently persisted game
// receiver confirms its inbox/effect commit with this live lease and digest.
func (s *Store) AckSessionEvent(ctx context.Context, principal SessionPrincipal, eventID, leaseID uuid.UUID, digest []byte) error {
	if s == nil || s.Pool == nil || principal.ApplicationID == uuid.Nil || principal.EnvironmentID == uuid.Nil ||
		eventID == uuid.Nil || leaseID == uuid.Nil || len(digest) != sha256.Size {
		return ErrIdempotencyConflict
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `UPDATE gis_session_outbox SET delivered_at=clock_timestamp(),
		consumer_ack_lease_id=$4,consumer_ack_payload_sha256=$5,claim_lease_id=NULL,claim_lease_until=NULL
		WHERE event_id=$1 AND application_id=$2 AND environment_id=$3 AND delivered_at IS NULL
		  AND claim_lease_id=$4 AND claim_lease_until>clock_timestamp() AND payload_sha256=$5`,
		eventID, principal.ApplicationID, principal.EnvironmentID, leaseID, digest)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 1 {
		return tx.Commit(ctx)
	}
	var deliveredAt *time.Time
	var ackLease uuid.UUID
	var ackDigest []byte
	err = tx.QueryRow(ctx, `SELECT delivered_at,consumer_ack_lease_id,consumer_ack_payload_sha256
		FROM gis_session_outbox WHERE event_id=$1 AND application_id=$2 AND environment_id=$3 FOR UPDATE`,
		eventID, principal.ApplicationID, principal.EnvironmentID).Scan(&deliveredAt, &ackLease, &ackDigest)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrSessionNotFound
	}
	if err != nil {
		return err
	}
	if deliveredAt != nil {
		if ackLease == leaseID && equalBytes(ackDigest, digest) {
			return tx.Commit(ctx)
		}
		return ErrIdempotencyConflict
	}
	return ErrIdempotencyConflict
}

// ConsumeActiveSessionEvent commits the inbox key and the caller's game-side
// effect in one transaction. Duplicate delivery returns applied=false.
func (s *Store) ConsumeActiveSessionEvent(ctx context.Context, event SessionActiveEvent, apply func(pgx.Tx) error) (bool, error) {
	if s == nil || s.Pool == nil || event.EventID == uuid.Nil || event.ApplicationID == uuid.Nil || event.EnvironmentID == uuid.Nil || event.SessionID == uuid.Nil || apply == nil {
		return false, errors.New("invalid active session event")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	result, err := tx.Exec(ctx, `INSERT INTO gis_session_inbox(application_id,environment_id,event_id) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, event.ApplicationID, event.EnvironmentID, event.EventID)
	if err != nil {
		return false, err
	}
	if result.RowsAffected() == 0 {
		if err = tx.Commit(ctx); err != nil {
			return false, err
		}
		return false, nil
	}
	if err = apply(tx); err != nil {
		return false, err
	}
	if err = tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}
