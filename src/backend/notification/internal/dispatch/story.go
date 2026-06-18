package dispatch

import (
	"context"

	"github.com/google/uuid"

	"voice/backend/notification/internal/apns"
	"voice/backend/notification/internal/delivery"
	"voice/backend/notification/internal/fcm"
	"voice/backend/notification/internal/push"
)

// StoryPusher sends pushes for story mention notifications.
type StoryPusher struct {
	Tokens TokenRepository
	Pusher *PushDispatcher
}

// SendPush delivers a push to all device tokens for each recipient with Push=true.
func (p *StoryPusher) SendPush(ctx context.Context, decisions map[string]delivery.DeliveryDecision, payload push.Payload) error {
	if p == nil || p.Tokens == nil || p.Pusher == nil || len(decisions) == 0 {
		return nil
	}
	for profileID, decision := range decisions {
		if !decision.Push {
			continue
		}
		recipient, err := uuid.Parse(profileID)
		if err != nil {
			continue
		}
		tokens, err := p.Tokens.ListByProfile(ctx, recipient)
		if err != nil {
			return err
		}
		for _, tok := range tokens {
			if !ShouldDeliverPushToToken(payload.Data["type"], tok.PushService) {
				continue
			}
			if err := p.Pusher.Send(ctx, recipient, tok, payload); err != nil {
				if err == fcm.ErrInvalidToken || err == apns.ErrInvalidToken {
					_ = p.Tokens.DeleteByToken(ctx, tok.Token)
					continue
				}
				return err
			}
		}
	}
	return nil
}
