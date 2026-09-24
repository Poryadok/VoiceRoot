package store

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// MessageDelivery is a durable recipient intent, independent of other bots.
type MessageDelivery struct {
	ID        uuid.UUID
	BotID     uuid.UUID
	MessageID uuid.UUID
	ChatID    uuid.UUID
	Payload   map[string]any
	Attempts  int
}

// QueueMessageRecipients commits the complete recipient set before NATS ACK.
// A failed transaction leaves the source message eligible for redelivery.
func (s *BotStore) QueueMessageRecipients(ctx context.Context, chatID, messageID uuid.UUID, senderProfileID string, payload map[string]any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	rows, err := tx.Query(ctx, `
SELECT b.id
FROM bots b JOIN bot_chat_whitelist w ON w.bot_id = b.id
WHERE w.chat_id = $1 AND w.enabled AND b.status = 'live'
  AND b.actor_profile_id::text <> $2
  AND (b.is_polling_mode OR NULLIF(b.webhook_url, '') IS NOT NULL)
ORDER BY b.id`, chatID, senderProfileID)
	if err != nil {
		return err
	}
	var botIDs []uuid.UUID
	for rows.Next() {
		var botID uuid.UUID
		if err := rows.Scan(&botID); err != nil {
			rows.Close()
			return err
		}
		botIDs = append(botIDs, botID)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, botID := range botIDs {
		// Lifecycle changes lock the bot before whitelist and delivery rows. Match
		// that order so recipient capture cannot deadlock with a reinstall/removal.
		var actorProfileID uuid.UUID
		var polling bool
		var webhookURL *string
		err := tx.QueryRow(ctx, `
SELECT actor_profile_id, is_polling_mode, webhook_url
FROM bots WHERE id = $1 AND status = 'live'
FOR SHARE`, botID).Scan(&actorProfileID, &polling, &webhookURL)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return err
		}
		if actorProfileID.String() == senderProfileID || (!polling && strings.TrimSpace(ptrValue(webhookURL)) == "") {
			continue
		}
		var enabled bool
		err = tx.QueryRow(ctx, `
SELECT enabled FROM bot_chat_whitelist WHERE bot_id = $1 AND chat_id = $2
FOR SHARE`, botID, chatID).Scan(&enabled)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && !enabled) {
			continue
		}
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
INSERT INTO bot_message_deliveries (id, bot_id, message_id, chat_id, payload)
VALUES (gen_random_uuid(), $1, $2, $3, $4::jsonb)
ON CONFLICT (bot_id, message_id) DO NOTHING`, botID, messageID, chatID, string(raw)); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
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
RETURNING d.id, d.bot_id, d.message_id, d.chat_id, d.payload, d.attempts`).Scan(
		&d.ID, &d.BotID, &d.MessageID, &d.ChatID, &raw, &d.Attempts)
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

// DeliverMessage serializes current authorization and destination changes with
// delivery. Revocation and secret/URL rotation wait until this transaction ends.
func (s *BotStore) DeliverMessage(ctx context.Context, d *MessageDelivery, post func(string, string) error) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var polling bool
	var url *string
	var secret string
	err = tx.QueryRow(ctx, `
SELECT b.is_polling_mode, b.webhook_url, b.webhook_secret
FROM bots b JOIN bot_chat_whitelist w ON w.bot_id = b.id
WHERE b.id = $1 AND w.chat_id = $2 AND w.enabled AND b.status = 'live'
FOR SHARE OF b, w`, d.BotID, d.ChatID).Scan(&polling, &url, &secret)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !polling && strings.TrimSpace(ptrValue(url)) == "") {
		_, err = tx.Exec(ctx, `UPDATE bot_message_deliveries SET status = 'canceled', claimed_until = NULL WHERE id = $1`, d.ID)
		if err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
	if err != nil {
		return err
	}
	var status string
	err = tx.QueryRow(ctx, `SELECT status FROM bot_message_deliveries WHERE id = $1 FOR UPDATE`, d.ID).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && status != "pending") {
		return tx.Commit(ctx)
	}
	if err != nil {
		return err
	}
	if polling {
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
	} else if err := post(strings.TrimSpace(ptrValue(url)), secret); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE bot_message_deliveries SET status = 'delivered', claimed_until = NULL, delivered_at = now() WHERE id = $1`, d.ID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func ptrValue(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func (s *BotStore) FailMessageDelivery(ctx context.Context, id uuid.UUID) error {
	_, err := s.Pool.Exec(ctx, `UPDATE bot_message_deliveries SET status = 'failed', claimed_until = NULL WHERE id = $1`, id)
	return err
}

func (s *BotStore) RetryMessageDelivery(ctx context.Context, id uuid.UUID, attempts int) error {
	delay := time.Second << min(attempts-1, 6)
	_, err := s.Pool.Exec(ctx, `UPDATE bot_message_deliveries SET claimed_until = NULL, next_attempt_at = now() + $2::interval WHERE id = $1`, id, delay.String())
	return err
}
