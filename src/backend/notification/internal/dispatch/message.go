package dispatch

import (
	"context"
	"time"

	"github.com/google/uuid"

	"voice/backend/notification/internal/apns"
	"voice/backend/notification/internal/delivery"
	"voice/backend/notification/internal/fcm"
	"voice/backend/notification/internal/grouping"
	"voice/backend/notification/internal/presence"
	"voice/backend/notification/internal/push"
	"voice/backend/notification/internal/store"
)

// TokenRepository lists and deletes device tokens for push delivery.
type TokenRepository interface {
	ListByProfile(ctx context.Context, profileID uuid.UUID) ([]store.DeviceToken, error)
	DeleteByToken(ctx context.Context, token string) error
}

// MessagePusher sends chat-grouped message pushes with settings, grouping, and presence.
type MessagePusher struct {
	Tokens   TokenRepository
	Pusher   *PushDispatcher
	Grouping grouping.Store
	Presence presence.Checker
	Policy   delivery.DeliveryPolicyLoader
	Router   func(in delivery.DeliveryInput) delivery.DeliveryDecision
}

func (p *MessagePusher) router() func(delivery.DeliveryInput) delivery.DeliveryDecision {
	if p != nil && p.Router != nil {
		return p.Router
	}
	return delivery.DecideRouting
}

func (p *MessagePusher) policy() delivery.DeliveryPolicyLoader {
	if p != nil && p.Policy != nil {
		return p.Policy
	}
	return delivery.PermissivePolicyLoader{}
}

// SendPush delivers grouped chat notifications to offline recipients.
func (p *MessagePusher) SendPush(
	ctx context.Context,
	decisions map[string]delivery.DeliveryDecision,
	in delivery.DeliveryInput,
	payload push.Payload,
	previewBody string,
) error {
	if p == nil || p.Tokens == nil || p.Pusher == nil || len(decisions) == 0 {
		return nil
	}
	notificationType := string(in.Type)
	if notificationType == "" && payload.Data != nil {
		notificationType = payload.Data["type"]
	}
	for profileID, decision := range decisions {
		if !decision.Push {
			continue
		}
		recipient, err := uuid.Parse(profileID)
		if err != nil {
			continue
		}
		settings, quiet, err := p.policy().LoadPolicy(ctx, recipient, in.ChatID, in.Type, time.Now())
		if err != nil {
			return err
		}
		perRecipient := delivery.FinalizeDecision(decision, delivery.DeliveryInput{
			RecipientProfileID: recipient,
			SenderProfileID:    in.SenderProfileID,
			ChatID:             in.ChatID,
			Type:               in.Type,
			IsOnline:           in.IsOnline,
			At:                 in.At,
		}, settings, quiet)
		if !perRecipient.Push {
			continue
		}
		out := payload
		if in.ChatID != "" && isChatGroupedType(in.Type) {
			if err := grouping.ApplyToPayload(ctx, p.Grouping, recipient, in.ChatID, previewBody, &out); err != nil {
				// Degraded: send without grouping when Redis fails.
				out = payload
			}
		}
		tokens, err := p.Tokens.ListByProfile(ctx, recipient)
		if err != nil {
			return err
		}
		if len(tokens) == 0 {
			continue
		}
		for _, tok := range tokens {
			if !ShouldDeliverPushToToken(notificationType, tok.PushService) {
				continue
			}
			if err := p.Pusher.Send(ctx, recipient, tok, out); err != nil {
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

// EnrichDecision applies presence, settings, and quiet hours to routing.
func (p *MessagePusher) EnrichDecision(
	ctx context.Context,
	profileID string,
	senderID uuid.UUID,
	chatID string,
	typ delivery.NotificationType,
) (delivery.DeliveryDecision, error) {
	recipient, err := uuid.Parse(profileID)
	if err != nil {
		return delivery.DeliveryDecision{}, err
	}
	isOnline := false
	if p != nil && p.Presence != nil {
		isOnline, err = p.Presence.IsOnline(ctx, recipient)
		if err != nil {
			isOnline = false
		}
	}
	in := delivery.DeliveryInput{
		RecipientProfileID: recipient,
		SenderProfileID:    senderID,
		ChatID:             chatID,
		Type:               typ,
		IsOnline:           isOnline,
		At:                 time.Now(),
	}
	decision := p.router()(in)
	settings, quiet, err := p.policy().LoadPolicy(ctx, recipient, chatID, typ, in.At)
	if err != nil {
		return delivery.DeliveryDecision{}, err
	}
	return delivery.FinalizeDecision(decision, in, settings, quiet), nil
}

func isChatGroupedType(typ delivery.NotificationType) bool {
	switch typ {
	case delivery.TypeNewMessage, delivery.TypeMention, delivery.TypeReply:
		return true
	default:
		return false
	}
}
