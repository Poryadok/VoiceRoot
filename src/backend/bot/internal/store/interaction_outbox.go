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
}

// ClaimSlashInteraction leases one due webhook interaction. A specific ID lets
// the request path attempt prompt delivery without bypassing the outbox lease.
func (s *BotStore) ClaimSlashInteraction(ctx context.Context, id uuid.UUID) (*SlashInteraction, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var eventID uuid.UUID
	err = tx.QueryRow(ctx, `
SELECT e.id FROM bot_event_log e JOIN bots b ON b.id = e.bot_id
WHERE e.event_type = 'interaction' AND e.delivery_status = 'pending'
  AND e.next_attempt_at <= now() AND (e.claimed_until IS NULL OR e.claimed_until < now())
  AND b.is_polling_mode = false AND b.webhook_url IS NOT NULL AND b.webhook_url <> ''
  AND ($1::uuid = '00000000-0000-0000-0000-000000000000'::uuid OR e.id = $1)
ORDER BY e.created_at LIMIT 1 FOR UPDATE OF e SKIP LOCKED`, id).Scan(&eventID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out SlashInteraction
	err = tx.QueryRow(ctx, `
UPDATE bot_event_log SET claimed_until = now() + interval '30 seconds', attempts = attempts + 1
WHERE id = $1 RETURNING id, bot_id, interaction_token, payload, attempts`, eventID).
		Scan(&out.ID, &out.BotID, &out.Token, &out.Payload, &out.Attempts)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *BotStore) CompleteSlashDelivery(ctx context.Context, id uuid.UUID, deferred bool) error {
	state := "delivered"
	if deferred {
		state = "deferred"
	}
	_, err := s.Pool.Exec(ctx, `
UPDATE bot_event_log SET delivery_status = $2, delivered_at = now(), claimed_until = NULL
WHERE id = $1 AND delivery_status = 'pending'`, id, state)
	return err
}

func (s *BotStore) RetrySlashDelivery(ctx context.Context, id uuid.UUID, attempts int) error {
	shift := attempts
	if shift > 8 {
		shift = 8
	}
	if shift < 1 {
		shift = 1
	}
	backoff := time.Duration(1<<uint(shift-1)) * time.Second
	_, err := s.Pool.Exec(ctx, `
UPDATE bot_event_log SET claimed_until = NULL, next_attempt_at = now() + $2::interval
WHERE id = $1 AND delivery_status = 'pending'`, id, backoff.String())
	return err
}

func (s *BotStore) FailSlashDelivery(ctx context.Context, id uuid.UUID) error {
	_, err := s.Pool.Exec(ctx, `
UPDATE bot_event_log SET delivery_status = 'failed', claimed_until = NULL
WHERE id = $1 AND delivery_status = 'pending'`, id)
	return err
}
