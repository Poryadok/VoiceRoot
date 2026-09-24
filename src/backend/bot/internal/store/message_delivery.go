package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// MessageDelivery is a durable recipient intent, independent of other bots.
type MessageDelivery struct {
	ID            uuid.UUID
	BotID         uuid.UUID
	MessageID     uuid.UUID
	ChatID        uuid.UUID
	Payload       map[string]any
	IsPollingMode bool
	WebhookURL    *string
	WebhookSecret *string
	Attempts      int
}

// QueueMessageRecipients commits the complete recipient set before NATS ACK.
// A failed transaction leaves the source message eligible for redelivery.
func (s *BotStore) QueueMessageRecipients(ctx context.Context, chatID, messageID uuid.UUID, senderProfileID string, payload map[string]any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = s.Pool.Exec(ctx, `
INSERT INTO bot_message_deliveries
    (id, bot_id, message_id, chat_id, payload, is_polling_mode, webhook_url, webhook_secret)
SELECT gen_random_uuid(), b.id, $2, $1, $4::jsonb, b.is_polling_mode, b.webhook_url, b.webhook_secret
FROM bots b JOIN bot_chat_whitelist w ON w.bot_id = b.id
WHERE w.chat_id = $1 AND w.enabled AND b.status = 'live'
  AND b.actor_profile_id::text <> $3
  AND (b.is_polling_mode OR NULLIF(b.webhook_url, '') IS NOT NULL)
ON CONFLICT (bot_id, message_id) DO NOTHING`, chatID, messageID, senderProfileID, string(raw))
	return err
}

// ClaimDueMessageDelivery leases one recipient; concurrent workers skip it.
func (s *BotStore) ClaimDueMessageDelivery(ctx context.Context) (*MessageDelivery, error) {
	var d MessageDelivery
	var raw []byte
	err := s.Pool.QueryRow(ctx, `
WITH due AS (
  SELECT id FROM bot_message_deliveries
  WHERE status = 'pending' AND next_attempt_at <= now()
    AND (claimed_until IS NULL OR claimed_until < now())
  ORDER BY next_attempt_at, created_at, id
  FOR UPDATE SKIP LOCKED LIMIT 1
)
UPDATE bot_message_deliveries d
SET claimed_until = now() + interval '2 minutes', attempts = attempts + 1
FROM due WHERE d.id = due.id
RETURNING d.id, d.bot_id, d.message_id, d.chat_id, d.payload, d.is_polling_mode,
          d.webhook_url, d.webhook_secret, d.attempts`).Scan(
		&d.ID, &d.BotID, &d.MessageID, &d.ChatID, &raw, &d.IsPollingMode,
		&d.WebhookURL, &d.WebhookSecret, &d.Attempts)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(raw, &d.Payload); err != nil {
		return nil, err
	}
	return &d, nil
}

// CompletePollingMessage atomically creates the visible event and receipt.
func (s *BotStore) CompletePollingMessage(ctx context.Context, d *MessageDelivery) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	raw, err := json.Marshal(d.Payload)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
INSERT INTO bot_event_log (id, bot_id, event_type, payload, delivery_status, interaction_token, source_message_id)
VALUES ($1, $2, 'message', $3::jsonb, 'pending', '', $4)
ON CONFLICT (bot_id, source_message_id) WHERE source_message_id IS NOT NULL DO NOTHING`,
		uuid.New(), d.BotID, string(raw), d.MessageID)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE bot_message_deliveries SET status = 'delivered', claimed_until = NULL, delivered_at = now() WHERE id = $1`, d.ID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *BotStore) CompleteWebhookMessage(ctx context.Context, id uuid.UUID) error {
	_, err := s.Pool.Exec(ctx, `UPDATE bot_message_deliveries SET status = 'delivered', claimed_until = NULL, delivered_at = now() WHERE id = $1`, id)
	return err
}

func (s *BotStore) RetryMessageDelivery(ctx context.Context, id uuid.UUID, attempts int) error {
	delay := time.Second << min(attempts-1, 6)
	_, err := s.Pool.Exec(ctx, `UPDATE bot_message_deliveries SET claimed_until = NULL, next_attempt_at = now() + $2::interval WHERE id = $1`, id, delay.String())
	return err
}
