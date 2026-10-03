package grpcsvc

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	messagingv1 "voice.app/voice/messaging/v1"
	"voice/backend/messaging/internal/gameprotocol"
	"voice/backend/messaging/internal/store"
)

type GameTombstoneSigningKeySource interface {
	CurrentGameTombstoneKey(context.Context) (gameprotocol.GameMessageTombstoneKey, error)
}

type VerifiedGameTombstoneProcessor struct {
	Store *store.MessagesStore
	Keys  GameTombstoneSigningKeySource
	Clock func() time.Time
}

func (p *VerifiedGameTombstoneProcessor) ProcessTombstoneGameMessage(ctx context.Context, request *messagingv1.TombstoneGameMessageRequest) error {
	if p == nil || p.Store == nil || p.Keys == nil || request == nil {
		return errors.New("game tombstone signing and storage are unavailable")
	}
	parse := func(value string) (uuid.UUID, error) {
		id, err := uuid.Parse(value)
		if err != nil || id.String() != value {
			return uuid.Nil, errors.New("invalid canonical tombstone identity")
		}
		return id, nil
	}
	app, err := parse(request.GetApplicationId())
	if err != nil {
		return err
	}
	env, err := parse(request.GetEnvironmentId())
	if err != nil {
		return err
	}
	chat, err := parse(request.GetChatId())
	if err != nil {
		return err
	}
	message, err := parse(request.GetMessageId())
	if err != nil {
		return err
	}
	action, err := parse(request.GetActionId())
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	if p.Clock != nil {
		now = p.Clock().UTC()
	}
	input := gameprotocol.GameMessageTombstone{
		ApplicationID: app, EnvironmentID: env, ChatID: chat, MessageID: message, ActionID: action,
		ReasonClass: request.GetReasonClass(), IssuedAt: now.Truncate(time.Second),
	}
	// Exact receipt lookup precedes current key availability and validity.
	// The store repeats it under the action lock to close the race.
	if _, found, err := p.Store.LookupGameMessageTombstoneReceipt(ctx, input); err != nil {
		return err
	} else if found {
		return nil
	}
	key, err := p.Keys.CurrentGameTombstoneKey(ctx)
	if err != nil {
		return err
	}
	_, _, err = p.Store.AppendGameMessageTombstone(ctx, input, func(tombstone gameprotocol.GameMessageTombstone) (string, error) {
		return gameprotocol.SignGameMessageTombstone(tombstone, key, now)
	})
	return err
}
