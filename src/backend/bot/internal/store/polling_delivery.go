package store

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// DeliverPollingEvent treats list results as hints. Message payloads are read
// again under the same Space guard as webhook delivery; the fence receipt waits
// for an already admitted stream send to finish. Non-message event behavior is
// preserved, with conditional terminal bookkeeping.
func (s *BotStore) DeliverPollingEvent(ctx context.Context, bot, event uuid.UUID, send func(string, string) error) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var kind, payload string
	var savedSpace *uuid.UUID
	err = tx.QueryRow(ctx, `SELECT event_type,payload::text,space_id FROM bot_event_log WHERE id=$1 AND bot_id=$2 AND delivery_status='pending'`, event, bot).Scan(&kind, &payload, &savedSpace)
	if errors.Is(err, pgx.ErrNoRows) {
		return tx.Commit(ctx)
	}
	if err != nil {
		return err
	}
	authorized := true
	if kind == "message" {
		var hint struct {
			ChatID string `json:"chat_id"`
		}
		if err = json.Unmarshal([]byte(payload), &hint); err != nil {
			return err
		}
		chat, parseErr := uuid.Parse(hint.ChatID)
		if parseErr != nil || chat == uuid.Nil {
			return ErrMessageBindingChanged
		}
		space, err := messageWhitelistSpace(ctx, tx, bot, chat)
		if err != nil {
			return err
		}
		spaces := []uuid.UUID{space}
		if savedSpace != nil {
			spaces = append(spaces, *savedSpace)
		}
		if err = lockMessageSpaces(ctx, tx, spaces); err != nil {
			return err
		}
		authorized = space != uuid.Nil && (savedSpace == nil || *savedSpace == space)
		for _, id := range spaces {
			if id == uuid.Nil {
				continue
			}
			live, err := messageSpaceLive(ctx, tx, id)
			if err != nil {
				return err
			}
			authorized = authorized && live
		}
		var polling bool
		var scopes string
		err = tx.QueryRow(ctx, `SELECT is_polling_mode,scopes::text FROM bots WHERE id=$1 AND status='live' FOR SHARE`, bot).Scan(&polling, &scopes)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		authorized = authorized && err == nil && polling && DevPollingEnabled() && ScopeAllows(scopes, PrivilegedScopeReadHistory)
		if authorized {
			authorized, err = lockMessageWhitelist(ctx, tx, bot, chat, space)
			if err != nil {
				return err
			}
		}
	}
	var currentKind, currentPayload string
	var currentSpace *uuid.UUID
	err = tx.QueryRow(ctx, `SELECT event_type,payload::text,space_id FROM bot_event_log WHERE id=$1 AND bot_id=$2 AND delivery_status='pending' FOR UPDATE`, event, bot).Scan(&currentKind, &currentPayload, &currentSpace)
	if errors.Is(err, pgx.ErrNoRows) {
		return tx.Commit(ctx)
	}
	if err != nil {
		return err
	}
	if currentKind != kind || currentPayload != payload || !sameOptionalUUID(savedSpace, currentSpace) {
		return ErrMessageBindingChanged
	}
	if !authorized {
		_, err = tx.Exec(ctx, `UPDATE bot_event_log SET delivery_status='canceled',claimed_until=NULL,payload='{}'::jsonb WHERE id=$1 AND delivery_status='pending'`, event)
	} else {
		if err = send(currentKind, currentPayload); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE bot_event_log SET delivery_status='delivered',delivered_at=now(),attempts=attempts+1 WHERE id=$1 AND delivery_status='pending'`, event)
	}
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func sameOptionalUUID(a, b *uuid.UUID) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
}
