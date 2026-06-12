package dispatch

import (
	"context"

	"github.com/google/uuid"

	"voice/backend/notification/internal/delivery"
	"voice/backend/notification/internal/fcm"
	"voice/backend/notification/internal/store"
)

// MatchmakingPusher sends FCM pushes for routed matchmaking notifications.
type MatchmakingPusher struct {
	Tokens *store.DeviceTokenStore
	FCM    fcm.Sender
}

// SendPush delivers a push to all device tokens for each recipient with Push=true.
func (p *MatchmakingPusher) SendPush(ctx context.Context, decisions map[string]delivery.DeliveryDecision, payload fcm.PushPayload) error {
	if p == nil || p.Tokens == nil || p.FCM == nil || len(decisions) == 0 {
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
		if len(tokens) == 0 {
			_ = p.FCM.Send(ctx, recipient, store.DeviceToken{}, payload)
			continue
		}
		for _, tok := range tokens {
			if err := p.FCM.Send(ctx, recipient, tok, payload); err != nil {
				if err == fcm.ErrInvalidToken {
					_ = p.Tokens.DeleteByToken(ctx, tok.Token)
					continue
				}
				return err
			}
		}
	}
	return nil
}
