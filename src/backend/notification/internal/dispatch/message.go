package dispatch

import (
	"context"
	"errors"
	"fmt"
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

var errPresenceAuthorityUnavailable = errors.New("notification presence authority unavailable")

// TokenRepository lists and deletes device tokens for push delivery.
type TokenRepository interface {
	ListByProfile(ctx context.Context, profileID uuid.UUID) ([]store.DeviceToken, error)
	DeleteByToken(ctx context.Context, token string) error
}

// MessagePusher sends grouped message pushes with settings, grouping, and presence.
type MessagePusher struct {
	LifecycleDelivery interface {
		WithChatDelivery(context.Context, string, func(context.Context) error) error
	}
	Tokens        TokenRepository
	Pusher        *PushDispatcher
	Grouping      grouping.Store
	Presence      presence.Checker
	Policy        delivery.DeliveryPolicyLoader
	Router        func(in delivery.DeliveryInput) delivery.DeliveryDecision
	GameConsent   GamePushConsentChecker
	GameBlocks    GamePushBlockChecker
	GameChatScope GamePushChatScopeResolver
}

// GamePushConsentChecker performs an execution-time GIS consent read for every device push.
type GamePushConsentChecker interface {
	AllowsGamePush(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, string) (bool, error)
}

type GamePushChatScopeResolver interface {
	ResolveGamePushChat(context.Context, uuid.UUID, uuid.UUID, string) (bool, bool, error)
}

type GamePushBlockChecker interface {
	IsGamePushBlocked(context.Context, uuid.UUID, uuid.UUID) (bool, error)
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
	if p != nil && p.LifecycleDelivery != nil {
		return p.LifecycleDelivery.WithChatDelivery(ctx, in.ChatID, func(guarded context.Context) error {
			return p.sendPush(guarded, decisions, in, payload, previewBody, false)
		})
	}
	return p.sendPush(ctx, decisions, in, payload, previewBody, false)
}

// SendPreparedPush sends decisions whose presence and policy inputs were
// resolved for every recipient before dispatch began. It avoids a second
// policy read that could fail after another recipient has already received a
// push.
func (p *MessagePusher) SendPreparedPush(
	ctx context.Context,
	decisions map[string]delivery.DeliveryDecision,
	in delivery.DeliveryInput,
	payload push.Payload,
	previewBody string,
) error {
	if p != nil && p.LifecycleDelivery != nil {
		return p.LifecycleDelivery.WithChatDelivery(ctx, in.ChatID, func(guarded context.Context) error {
			return p.sendPush(guarded, decisions, in, payload, previewBody, true)
		})
	}
	return p.sendPush(ctx, decisions, in, payload, previewBody, true)
}

func (p *MessagePusher) sendPush(
	ctx context.Context,
	decisions map[string]delivery.DeliveryDecision,
	in delivery.DeliveryInput,
	payload push.Payload,
	previewBody string,
	prepared bool,
) error {
	if p == nil || p.Tokens == nil || p.Pusher == nil || len(decisions) == 0 {
		return nil
	}
	notificationType := string(in.Type)
	if notificationType == "" && payload.Data != nil {
		notificationType = payload.Data["type"]
	}
	resolved := make(map[string]delivery.DeliveryDecision, len(decisions))
	for profileID, decision := range decisions {
		if !decision.Push {
			continue
		}
		recipient, err := uuid.Parse(profileID)
		if err != nil {
			continue
		}
		if !prepared {
			settings, quiet, err := p.policy().LoadPolicy(ctx, recipient, in.ChatID, in.Type, time.Now())
			if err != nil {
				return err
			}
			decision = delivery.FinalizeDecision(decision, delivery.DeliveryInput{
				RecipientProfileID: recipient,
				SenderProfileID:    in.SenderProfileID,
				ChatID:             in.ChatID,
				Type:               in.Type,
				IsOnline:           in.IsOnline,
				At:                 in.At,
			}, settings, quiet)
		}
		if decision.Push {
			resolved[profileID] = decision
		}
	}
	for profileID := range resolved {
		recipient, _ := uuid.Parse(profileID)
		out := payload
		groupingPreview := previewBody
		if p.GameChatScope != nil && in.GameCategory != "" && in.GameApplicationID == uuid.Nil && in.GameEnvironmentID == uuid.Nil {
			// A chat may be game-managed even when the event has no persisted app scope.
			// Keep private message text out of shared grouping state until GIS classifies it.
			groupingPreview = ""
		}
		if groupingID := pushGroupingID(in); groupingID != "" {
			if err := grouping.ApplyToPayload(ctx, p.Grouping, recipient, groupingID, groupingPreview, &out); err != nil {
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
			gameScoped := false
			if in.GameApplicationID != uuid.Nil || in.GameEnvironmentID != uuid.Nil {
				if in.GameApplicationID == uuid.Nil || in.GameEnvironmentID == uuid.Nil || in.GameCategory == "" || p.GameConsent == nil || p.GameBlocks == nil {
					continue
				}
				allowed, err := p.GameConsent.AllowsGamePush(ctx, recipient, in.GameApplicationID, in.GameEnvironmentID, in.GameCategory)
				if err != nil {
					return err
				}
				if !allowed {
					continue
				}
				gameScoped = true
			} else if in.GameCategory != "" && in.ChatID != "" && p.GameChatScope != nil {
				chatID, err := uuid.Parse(in.ChatID)
				if err != nil || chatID == uuid.Nil {
					return fmt.Errorf("game notification chat scope: invalid chat id")
				}
				resolvedGameScoped, allowed, err := p.GameChatScope.ResolveGamePushChat(ctx, recipient, chatID, in.GameCategory)
				if err != nil {
					return err
				}
				gameScoped = resolvedGameScoped
				if gameScoped && !allowed {
					continue
				}
			}
			if gameScoped {
				if p.GameBlocks == nil {
					continue
				}
				blocked, err := p.GameBlocks.IsGamePushBlocked(ctx, recipient, in.SenderProfileID)
				if err != nil {
					return err
				}
				if blocked {
					continue
				}
			}
			devicePayload := out
			if gameScoped {
				devicePayload.Title = "Game update"
				devicePayload.Body = "A game event is waiting in Voice."
			}
			if err := p.Pusher.Send(ctx, recipient, tok, devicePayload); err != nil {
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
	ctx context.Context, profileID string, senderID uuid.UUID, chatID string, typ delivery.NotificationType,
) (delivery.DeliveryDecision, error) {
	if p != nil && p.LifecycleDelivery != nil {
		var decision delivery.DeliveryDecision
		err := p.LifecycleDelivery.WithChatDelivery(ctx, chatID, func(guarded context.Context) error {
			var err error
			decision, err = p.enrichDecision(guarded, profileID, senderID, chatID, typ)
			return err
		})
		return decision, err
	}
	return p.enrichDecision(ctx, profileID, senderID, chatID, typ)
}

func (p *MessagePusher) enrichDecision(
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
	if !delivery.SkipsPresenceCheck(typ) {
		if p == nil || p.Presence == nil {
			return delivery.DeliveryDecision{}, errPresenceAuthorityUnavailable
		}
		isOnline, err = p.Presence.IsOnline(ctx, recipient)
		if err != nil {
			return delivery.DeliveryDecision{}, fmt.Errorf("check notification presence: %w", err)
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

func pushGroupingID(in delivery.DeliveryInput) string {
	switch in.Type {
	case delivery.TypeMessageRequest:
		if in.SenderProfileID == uuid.Nil {
			return ""
		}
		return "sender:" + in.SenderProfileID.String()
	case delivery.TypeNewMessage, delivery.TypeMention, delivery.TypeReply:
		return in.ChatID
	default:
		return ""
	}
}
