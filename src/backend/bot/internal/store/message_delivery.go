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
SELECT b.id,w.space_id
FROM bots b JOIN bot_chat_whitelist w ON w.bot_id = b.id
WHERE w.chat_id = $1 AND w.enabled AND b.status = 'live'
  AND b.actor_profile_id::text <> $2
  AND b.scopes @> '["TEXT_CHAT_READ_HISTORY"]'::jsonb
  AND ((b.is_polling_mode AND $3::bool) OR (NOT b.is_polling_mode AND NULLIF(b.webhook_url, '') IS NOT NULL))
ORDER BY b.id`, chatID, senderProfileID, DevPollingEnabled())
	if err != nil {
		return err
	}
	type candidate struct {
		bot, space uuid.UUID
		allowed    bool
	}
	var candidates []candidate
	var spaces []uuid.UUID
	for rows.Next() {
		var c candidate
		if err := rows.Scan(&c.bot, &c.space); err != nil {
			rows.Close()
			return err
		}
		candidates = append(candidates, c)
		spaces = append(spaces, c.space)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	if err := lockMessageSpaces(ctx, tx, spaces); err != nil {
		return err
	}
	for i := range candidates {
		c := &candidates[i]
		live, err := messageSpaceLive(ctx, tx, c.space)
		if err != nil {
			return err
		}
		if !live {
			continue
		}
		var actorProfileID uuid.UUID
		var polling bool
		var webhookURL *string
		var scopes string
		err = tx.QueryRow(ctx, `
SELECT actor_profile_id, is_polling_mode, webhook_url, scopes::text
FROM bots WHERE id = $1 AND status = 'live'
FOR SHARE`, c.bot).Scan(&actorProfileID, &polling, &webhookURL, &scopes)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return err
		}
		if actorProfileID.String() == senderProfileID || !ScopeAllows(scopes, PrivilegedScopeReadHistory) || (polling && !DevPollingEnabled()) || (!polling && strings.TrimSpace(ptrValue(webhookURL)) == "") {
			continue
		}
		c.allowed = true
	}
	// Acquire every bot lock before any whitelist/delivery row lock.
	for i := range candidates {
		c := &candidates[i]
		if !c.allowed {
			continue
		}
		enabled, err := lockMessageWhitelist(ctx, tx, c.bot, chatID, c.space)
		if err != nil {
			return err
		}
		if !enabled {
			c.allowed = false
			continue
		}
	}
	// Complete whitelist authorization before inserting any delivery rows.
	for _, c := range candidates {
		if !c.allowed {
			continue
		}
		if _, err := tx.Exec(ctx, `
INSERT INTO bot_message_deliveries (id, bot_id, message_id, chat_id, payload)
VALUES (gen_random_uuid(), $1, $2, $3, $4::jsonb)
ON CONFLICT (bot_id, message_id) DO NOTHING`, c.bot, messageID, chatID, string(raw)); err != nil {
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
	space, err := messageWhitelistSpace(ctx, tx, d.BotID, d.ChatID)
	if err != nil {
		return err
	}
	if err = lockMessageSpaces(ctx, tx, []uuid.UUID{space}); err != nil {
		return err
	}
	live := false
	if space != uuid.Nil {
		live, err = messageSpaceLive(ctx, tx, space)
		if err != nil {
			return err
		}
	}
	var polling bool
	var url *string
	var secret string
	var scopes string
	err = tx.QueryRow(ctx, `
SELECT is_polling_mode, webhook_url, webhook_secret, scopes::text
FROM bots WHERE id = $1 AND status = 'live' FOR SHARE`, d.BotID).Scan(&polling, &url, &secret, &scopes)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	authorized := err == nil && live && ScopeAllows(scopes, PrivilegedScopeReadHistory) && ((!polling && strings.TrimSpace(ptrValue(url)) != "") || (polling && DevPollingEnabled()))
	if authorized {
		authorized, err = lockMessageWhitelist(ctx, tx, d.BotID, d.ChatID, space)
		if err != nil {
			return err
		}
	}
	if !authorized {
		err = cancelMessageDelivery(ctx, tx, d.ID)
		if err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
	if err != nil {
		return err
	}
	var status string
	var raw []byte
	err = tx.QueryRow(ctx, `SELECT status,payload FROM bot_message_deliveries WHERE id = $1 AND bot_id=$2 AND chat_id=$3 AND message_id=$4 FOR UPDATE`, d.ID, d.BotID, d.ChatID, d.MessageID).Scan(&status, &raw)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && status != "pending") {
		return tx.Commit(ctx)
	}
	if err != nil {
		return err
	}
	if err = json.Unmarshal(raw, &d.Payload); err != nil {
		return err
	}
	if polling {
		raw, err := json.Marshal(d.Payload)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `
INSERT INTO bot_event_log (id, bot_id, event_type, payload, delivery_status, interaction_token, source_message_id,space_id)
VALUES ($1, $2, 'message', $3::jsonb, 'pending', '', $4,$5)
ON CONFLICT (bot_id, source_message_id) WHERE source_message_id IS NOT NULL DO NOTHING`,
			uuid.New(), d.BotID, string(raw), d.MessageID, space)
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
	_, err := s.Pool.Exec(ctx, `UPDATE bot_message_deliveries SET status = 'failed', claimed_until = NULL WHERE id = $1 AND status='pending'`, id)
	return err
}

func (s *BotStore) RetryMessageDelivery(ctx context.Context, id uuid.UUID, attempts int) error {
	delay := time.Second << min(attempts-1, 6)
	_, err := s.Pool.Exec(ctx, `UPDATE bot_message_deliveries SET claimed_until = NULL, next_attempt_at = now() + $2::interval WHERE id = $1 AND status='pending'`, id, delay.String())
	return err
}
