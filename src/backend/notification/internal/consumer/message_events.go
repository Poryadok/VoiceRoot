package consumer

import (
	"context"

	"github.com/google/uuid"

	"voice/backend/notification/internal/delivery"
	eventsv1 "voice.app/voice/events/v1"
)

// MessageEventHandler maps NATS message.events to notification delivery decisions.
type MessageEventHandler struct {
	Router func(in delivery.DeliveryInput) delivery.DeliveryDecision
}

func (h *MessageEventHandler) route(senderID, recipientID, chatID string, typ delivery.NotificationType) delivery.DeliveryDecision {
	router := h.Router
	if router == nil {
		router = delivery.DecideRouting
	}
	senderUUID, _ := uuid.Parse(senderID)
	recipientUUID, _ := uuid.Parse(recipientID)
	return router(delivery.DeliveryInput{
		RecipientProfileID: recipientUUID,
		SenderProfileID:    senderUUID,
		ChatID:             chatID,
		Type:               typ,
		IsOnline:           false,
	})
}

// HandleMessageSent returns per-recipient delivery decisions for a MessageSent event.
func (h *MessageEventHandler) HandleMessageSent(ctx context.Context, ev *eventsv1.MessageSent, memberProfileIDs []string) map[string]delivery.DeliveryDecision {
	_ = ctx
	if ev == nil {
		return nil
	}
	if ev.GetThreadParentId() != "" {
		return nil
	}
	out := make(map[string]delivery.DeliveryDecision)
	for _, profileID := range memberProfileIDs {
		if profileID == "" || profileID == ev.GetSenderProfileId() {
			continue
		}
		decision := h.route(ev.GetSenderProfileId(), profileID, ev.GetChatId(), delivery.TypeNewMessage)
		out[profileID] = decision
	}
	return out
}

// HandleMessageReply returns delivery decision for the parent-message author on thread replies.
func (h *MessageEventHandler) HandleMessageReply(ctx context.Context, ev *eventsv1.MessageSent, parentAuthorProfileID string) map[string]delivery.DeliveryDecision {
	_ = ctx
	if ev == nil || ev.GetThreadParentId() == "" || parentAuthorProfileID == "" {
		return nil
	}
	if parentAuthorProfileID == ev.GetSenderProfileId() {
		return nil
	}
	decision := h.route(ev.GetSenderProfileId(), parentAuthorProfileID, ev.GetChatId(), delivery.TypeReply)
	return map[string]delivery.DeliveryDecision{
		parentAuthorProfileID: decision,
	}
}

// HandleMentionAdded returns per-mentioned-profile delivery decisions.
func (h *MessageEventHandler) HandleMentionAdded(ctx context.Context, ev *eventsv1.MentionAdded) map[string]delivery.DeliveryDecision {
	_ = ctx
	if ev == nil {
		return nil
	}
	out := make(map[string]delivery.DeliveryDecision)
	for _, profileID := range ev.GetMentionedProfileIds() {
		if profileID == "" || profileID == ev.GetSenderProfileId() {
			continue
		}
		decision := h.route(ev.GetSenderProfileId(), profileID, ev.GetChatId(), delivery.TypeMention)
		out[profileID] = decision
	}
	return out
}
