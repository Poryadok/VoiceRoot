package store

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ErrMessageBindingChanged asks the caller to retry discovery in a new transaction.
// Never acquire another Space lock after bot/whitelist/delivery row locks.
var ErrMessageBindingChanged = errors.New("bot message Space binding changed")

// Lock order: all distinct actual Space advisory keys, ascending; bot IDs,
// ascending; whitelist (bot_id,chat_id); delivery/event rows. Message senders use
// shared Space locks through the external attempt and commit; lifecycle and
// installation mutations use exclusive locks. Per-chat enablement uses a shared
// Space guard and serializes with delivery through the whitelist row instead.
// No transaction upgrades a lock.
func lockMessageSpaces(ctx context.Context, tx pgx.Tx, spaces []uuid.UUID) error {
	ids := make([]string, 0, len(spaces))
	for _, id := range spaces {
		if id != uuid.Nil {
			ids = append(ids, id.String())
		}
	}
	if len(ids) == 0 {
		return nil
	}
	rows, err := tx.Query(ctx, `SELECT DISTINCT hashtextextended(id,71431) AS key FROM unnest($1::text[]) AS t(id) ORDER BY key`, ids)
	if err != nil {
		return err
	}
	var keys []int64
	for rows.Next() {
		var key int64
		if err := rows.Scan(&key); err != nil {
			rows.Close()
			return err
		}
		keys = append(keys, key)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, key := range keys {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock_shared($1)`, key); err != nil {
			return err
		}
	}
	return nil
}

// This statement must run AFTER waiting for the advisory lock so READ COMMITTED
// observes the fence that released that lock. No head preserves sparse semantics.
func messageSpaceLive(ctx context.Context, tx pgx.Tx, space uuid.UUID) (bool, error) {
	var state string
	err := tx.QueryRow(ctx, `SELECT desired_state FROM bot_space_lifecycle_heads WHERE space_id=$1`, space).Scan(&state)
	if errors.Is(err, pgx.ErrNoRows) {
		return true, nil
	}
	return err == nil && state == "LIVE", err
}

func messageWhitelistSpace(ctx context.Context, tx pgx.Tx, bot, chat uuid.UUID) (uuid.UUID, error) {
	var space uuid.UUID
	err := tx.QueryRow(ctx, `SELECT space_id FROM bot_chat_whitelist WHERE bot_id=$1 AND chat_id=$2`, bot, chat).Scan(&space)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, nil
	}
	return space, err
}

func lockMessageWhitelist(ctx context.Context, tx pgx.Tx, bot, chat, captured uuid.UUID) (bool, error) {
	var space uuid.UUID
	var enabled bool
	err := tx.QueryRow(ctx, `SELECT space_id,enabled FROM bot_chat_whitelist WHERE bot_id=$1 AND chat_id=$2 FOR SHARE`, bot, chat).Scan(&space, &enabled)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if space != captured {
		return false, ErrMessageBindingChanged
	}
	return enabled, nil
}

func cancelMessageDelivery(ctx context.Context, tx pgx.Tx, id uuid.UUID) error {
	_, err := tx.Exec(ctx, `UPDATE bot_message_deliveries SET status='canceled',claimed_until=NULL,payload='{}'::jsonb WHERE id=$1 AND status='pending'`, id)
	return err
}
