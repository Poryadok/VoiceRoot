package consumer

import (
	"context"

	"github.com/google/uuid"

	"voice/backend/notification/internal/delivery"
	eventsv1 "voice.app/voice/events/v1"
)

// SocialEventHandler maps social.events to notification delivery decisions.
type SocialEventHandler struct {
	Router func(in delivery.DeliveryInput) delivery.DeliveryDecision
}

func (h *SocialEventHandler) route(senderID, recipientID string, typ delivery.NotificationType) delivery.DeliveryDecision {
	router := h.Router
	if router == nil {
		router = delivery.DecideRouting
	}
	senderUUID, _ := uuid.Parse(senderID)
	recipientUUID, _ := uuid.Parse(recipientID)
	return router(delivery.DeliveryInput{
		RecipientProfileID: recipientUUID,
		SenderProfileID:    senderUUID,
		Type:               typ,
		IsOnline:           false,
	})
}

// HandleFriendRequest returns per-recipient delivery decisions for a friend request.
func (h *SocialEventHandler) HandleFriendRequest(ctx context.Context, ev *eventsv1.FriendRequest) map[string]delivery.DeliveryDecision {
	_ = ctx
	if ev == nil || ev.GetTargetProfileId() == "" {
		return nil
	}
	return map[string]delivery.DeliveryDecision{
		ev.GetTargetProfileId(): h.route(ev.GetRequesterProfileId(), ev.GetTargetProfileId(), delivery.TypeFriendReq),
	}
}
