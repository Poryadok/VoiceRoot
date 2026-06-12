package dispatch

import (
	"context"

	"github.com/google/uuid"

	"voice/backend/notification/internal/apns"
	"voice/backend/notification/internal/delivery"
	"voice/backend/notification/internal/push"
	"voice/backend/notification/internal/store"
)

type profileTokenLister interface {
	ListByProfile(ctx context.Context, profileID uuid.UUID) ([]store.DeviceToken, error)
	DeleteByToken(ctx context.Context, token string) error
}

// CallPusher sends VoIP pushes for routed incoming_call notifications.
type CallPusher struct {
	Tokens profileTokenLister
	Pusher *PushDispatcher
}

// SendPush delivers a VoIP push to voip_apns tokens for recipients with Push=true.
func (p *CallPusher) SendPush(ctx context.Context, decisions map[string]delivery.DeliveryDecision, payload push.Payload) error {
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
		sent := false
		for _, tok := range tokens {
			if tok.PushService != "voip_apns" {
				continue
			}
			sent = true
			if err := p.Pusher.Send(ctx, recipient, tok, payload); err != nil {
				if err == apns.ErrInvalidToken {
					_ = p.Tokens.DeleteByToken(ctx, tok.Token)
					continue
				}
				return err
			}
		}
		if !sent {
			_ = p.Pusher.Send(ctx, recipient, store.DeviceToken{PushService: "voip_apns"}, payload)
		}
	}
	return nil
}
