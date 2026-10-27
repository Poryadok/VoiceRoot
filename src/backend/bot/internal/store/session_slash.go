package store

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"time"
)

type SlashResult struct{ Deferred, Retry, Failed bool }

func (s *BotStore) CheckSlashAttempt(ctx context.Context, event *SlashInteraction) error {
	if err := s.CheckGuardedSession(ctx, Session{BotID: event.BotID, Epoch: event.Epoch}); err != nil {
		return err
	}
	held := ctx.Value(guardedSessionKey{}).(*guardedSession)
	var live bool
	err := held.tx.QueryRow(ctx, `SELECT claimed_until>clock_timestamp() FROM bot_event_log WHERE id=$1 AND bot_id=$2 AND delivery_status='pending' AND attempts=$3 AND session_epoch=$4`, event.ID, event.BotID, event.Attempts, event.Epoch).Scan(&live)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrSessionRevoked
	}
	if err != nil {
		return err
	}
	if !live {
		return ErrSessionRevoked
	}
	return nil
}

// DeliverSlashInteraction reloads authority and claim under ordered locks. A
// claim is a scheduling hint, never permission to issue a webhook request.
func (s *BotStore) DeliverSlashInteraction(ctx context.Context, event *SlashInteraction, deliver func(context.Context, *BotRow) (SlashResult, error)) (bool, error) {
	var hint struct {
		ChatID string `json:"chat_id"`
	}
	if json.Unmarshal(event.Payload, &hint) != nil {
		return false, ErrMessageBindingChanged
	}
	chat, err := uuid.Parse(hint.ChatID)
	if err != nil || chat == uuid.Nil {
		return false, ErrMessageBindingChanged
	}
	var effectErr error
	var applied bool
	if event.SpaceID == nil || *event.SpaceID == uuid.Nil {
		return false, ErrMessageBindingChanged
	}
	err = s.WithSessionSpaces(ctx, Authority{Session: Session{BotID: event.BotID, Epoch: event.Epoch}}, chat, []uuid.UUID{*event.SpaceID}, func(call context.Context, tx pgx.Tx, bot *BotRow) error {
		var payload string
		var token string
		var epoch int64
		var until *time.Time
		var space *uuid.UUID
		err := tx.QueryRow(call, `SELECT payload::text,interaction_token,session_epoch,claimed_until,space_id FROM bot_event_log
WHERE id=$1 AND bot_id=$2 AND delivery_status='pending' AND attempts=$3
FOR UPDATE`, event.ID, event.BotID, event.Attempts).Scan(&payload, &token, &epoch, &until, &space)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		var claimLive bool
		if until == nil {
			return nil
		}
		if err = tx.QueryRow(call, `SELECT $1::timestamptz>clock_timestamp()`, *until).Scan(&claimLive); err != nil {
			return err
		}
		if !claimLive {
			return nil
		}
		guardedSpace, held := GuardedChatSpace(call)
		if !held || !sameOptionalUUID(space, event.SpaceID) || space == nil || *space != guardedSpace {
			return ErrMessageBindingChanged
		}
		if epoch != event.Epoch || token != event.Token || !jsonEqual([]byte(payload), event.Payload) {
			return ErrMessageBindingChanged
		}
		if bot.IsPollingMode || bot.WebhookURL == nil || !ScopeAllows(bot.ScopesJSON, "TEXT_CHAT_SEND_MESSAGES") {
			return ErrMessageSendDenied
		}
		result, e := deliver(call, bot)
		effectErr = e
		state := "delivered"
		if result.Deferred {
			state = "deferred"
		}
		if result.Failed {
			state = "failed"
		}
		if result.Retry {
			shift := event.Attempts
			if shift > 8 {
				shift = 8
			}
			if shift < 1 {
				shift = 1
			}
			backoff := time.Duration(1<<uint(shift-1)) * time.Second
			tag, e := tx.Exec(call, `UPDATE bot_event_log SET claimed_until=NULL,next_attempt_at=clock_timestamp()+$4::interval,
delivery_status=CASE WHEN attempts >= $5 OR created_at<=clock_timestamp()-interval '24 hours' THEN 'failed' ELSE 'pending' END
WHERE id=$1 AND delivery_status='pending' AND attempts=$2 AND session_epoch=$3 AND claimed_until>clock_timestamp()`, event.ID, event.Attempts, event.Epoch, backoff.String(), slashDeliveryMaxAttempts)
			err = e
			applied = e == nil && tag.RowsAffected() == 1
		} else {
			tag, e := tx.Exec(call, `UPDATE bot_event_log SET delivery_status=$4,claimed_until=NULL,delivered_at=clock_timestamp() WHERE id=$1 AND delivery_status='pending' AND attempts=$2 AND session_epoch=$3 AND claimed_until>clock_timestamp()`, event.ID, event.Attempts, event.Epoch, state)
			err = e
			applied = e == nil && tag.RowsAffected() == 1
		}
		return err
	})
	if err != nil {
		return false, err
	}
	return applied, effectErr
}

func jsonEqual(a, b []byte) bool {
	var x, y any
	left := json.NewDecoder(bytes.NewReader(a))
	left.UseNumber()
	right := json.NewDecoder(bytes.NewReader(b))
	right.UseNumber()
	if left.Decode(&x) != nil || right.Decode(&y) != nil {
		return false
	}
	xb, _ := json.Marshal(x)
	yb, _ := json.Marshal(y)
	return string(xb) == string(yb)
}
