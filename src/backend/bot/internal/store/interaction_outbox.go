package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// SlashInteraction is a leased, durably accepted webhook command.
type SlashInteraction struct {
	ID       uuid.UUID
	BotID    uuid.UUID
	Token    string
	Payload  []byte
	Attempts int
	Epoch    int64
	SpaceID  *uuid.UUID
}

const slashDeliveryMaxAttempts = 8

// ClaimSlashInteraction leases one due webhook interaction. A specific ID lets
// the request path attempt prompt delivery without bypassing the outbox lease.
func (s *BotStore) ClaimSlashInteraction(ctx context.Context, id uuid.UUID) (*SlashInteraction, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	// Expire accepted intents even when a bot has since removed its webhook or
	// switched delivery mode; those rows are not eligible for a new claim.
	_, err = tx.Exec(ctx, `
WITH stale AS (
  SELECT id FROM bot_event_log
  WHERE event_type = 'interaction' AND delivery_status = 'pending'
    AND (attempts >= $2 OR created_at <= clock_timestamp() - interval '24 hours')
    AND (claimed_until IS NULL OR claimed_until < clock_timestamp())
    AND ($1::uuid = '00000000-0000-0000-0000-000000000000'::uuid OR id = $1)
  ORDER BY created_at LIMIT 32 FOR UPDATE SKIP LOCKED
)
UPDATE bot_event_log e SET delivery_status = 'failed', claimed_until = NULL
FROM stale WHERE e.id = stale.id`, id, slashDeliveryMaxAttempts)
	if err != nil {
		return nil, err
	}
	var eventID uuid.UUID
	var exhausted bool
	err = tx.QueryRow(ctx, `
SELECT e.id, e.attempts >= $2 OR e.created_at <= clock_timestamp() - interval '24 hours'
FROM bot_event_log e JOIN bots b ON b.id = e.bot_id
WHERE e.event_type = 'interaction' AND e.delivery_status = 'pending'
  AND e.session_epoch=b.session_epoch AND b.status='live'
  AND e.next_attempt_at <= clock_timestamp() AND (e.claimed_until IS NULL OR e.claimed_until < clock_timestamp())
  AND b.is_polling_mode = false AND b.webhook_url IS NOT NULL AND b.webhook_url <> ''
  AND ($1::uuid = '00000000-0000-0000-0000-000000000000'::uuid OR e.id = $1)
ORDER BY e.created_at LIMIT 1 FOR UPDATE OF e SKIP LOCKED`, id, slashDeliveryMaxAttempts).Scan(&eventID, &exhausted)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, tx.Commit(ctx)
	}
	if err != nil {
		return nil, err
	}
	if exhausted {
		if _, err := tx.Exec(ctx, `UPDATE bot_event_log SET delivery_status = 'failed', claimed_until = NULL WHERE id = $1`, eventID); err != nil {
			return nil, err
		}
		return nil, tx.Commit(ctx)
	}
	var out SlashInteraction
	err = tx.QueryRow(ctx, `
UPDATE bot_event_log SET claimed_until = clock_timestamp() + interval '30 seconds', attempts = attempts + 1
WHERE id = $1 RETURNING id, bot_id, interaction_token, payload, attempts,session_epoch,space_id`, eventID).
		Scan(&out.ID, &out.BotID, &out.Token, &out.Payload, &out.Attempts, &out.Epoch, &out.SpaceID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &out, nil
}

// CompleteSlashDelivery records the delivery result for the active claim.
// The claim generation is the attempts counter. A transition needs both that
// generation and its unexpired lease, so a paused worker cannot overwrite a
// newer claim after its lease lapses.
func (s *BotStore) CompleteSlashDelivery(ctx context.Context, id uuid.UUID, attempts int, deferred bool) (bool, error) {
	state := "delivered"
	if deferred {
		state = "deferred"
	}
	return s.withLiveSlashClaim(ctx, id, attempts, func(tx pgx.Tx) (bool, error) {
		tag, err := tx.Exec(ctx, `
UPDATE bot_event_log SET delivery_status = $2, delivered_at = clock_timestamp(), claimed_until = NULL
WHERE id = $1 AND delivery_status = 'pending' AND attempts = $3
  AND claimed_until > clock_timestamp()`, id, state, attempts)
		return tag.RowsAffected() == 1, err
	})
}

func (s *BotStore) RetrySlashDelivery(ctx context.Context, id uuid.UUID, attempts int) (bool, error) {
	shift := attempts
	if shift > 8 {
		shift = 8
	}
	if shift < 1 {
		shift = 1
	}
	backoff := time.Duration(1<<uint(shift-1)) * time.Second
	return s.withLiveSlashClaim(ctx, id, attempts, func(tx pgx.Tx) (bool, error) {
		tag, err := tx.Exec(ctx, `
UPDATE bot_event_log SET
  delivery_status = CASE WHEN attempts >= $3 OR created_at <= clock_timestamp() - interval '24 hours'
    THEN 'failed' ELSE 'pending' END,
  claimed_until = NULL, next_attempt_at = clock_timestamp() + $2::interval
WHERE id = $1 AND delivery_status = 'pending' AND attempts = $4
  AND claimed_until > clock_timestamp()`, id, backoff.String(), slashDeliveryMaxAttempts, attempts)
		return tag.RowsAffected() == 1, err
	})
}

func (s *BotStore) FailSlashDelivery(ctx context.Context, id uuid.UUID, attempts int) (bool, error) {
	return s.withLiveSlashClaim(ctx, id, attempts, func(tx pgx.Tx) (bool, error) {
		tag, err := tx.Exec(ctx, `
UPDATE bot_event_log SET delivery_status = 'failed', claimed_until = NULL
WHERE id = $1 AND delivery_status = 'pending' AND attempts = $2
  AND claimed_until > clock_timestamp()`, id, attempts)
		return tag.RowsAffected() == 1, err
	})
}

// Event-only bookkeeping never acquires an upstream bot/Space lock. Fresh wall
// clock is read in a separate statement after the event row-lock wait.
func (s *BotStore) withLiveSlashClaim(ctx context.Context, id uuid.UUID, attempts int, update func(pgx.Tx) (bool, error)) (bool, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer rollbackSession(tx)
	var until *time.Time
	err = tx.QueryRow(ctx, `SELECT claimed_until FROM bot_event_log WHERE id=$1 AND delivery_status='pending' AND attempts=$2 FOR UPDATE`, id, attempts).Scan(&until)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if until == nil {
		return false, nil
	}
	var live bool
	if err = tx.QueryRow(ctx, `SELECT $1::timestamptz>clock_timestamp()`, *until).Scan(&live); err != nil {
		return false, err
	}
	if !live {
		return false, nil
	}
	applied, err := update(tx)
	if err != nil || !applied {
		return false, err
	}
	if err = tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}
