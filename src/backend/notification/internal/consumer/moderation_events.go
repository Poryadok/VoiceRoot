package consumer

import (
	"context"

	"voice/backend/notification/internal/delivery"
	eventsv1 "voice.app/voice/events/v1"
)

// ModerationEventHandler maps moderation.events to notification delivery decisions.
type ModerationEventHandler struct {
	Router func(in delivery.DeliveryInput) delivery.DeliveryDecision
}

// HandleSanctionApplied returns a system notification decision for the sanctioned account.
// Recipient profile resolution is deferred to the caller when account→profile mapping is available.
func (h *ModerationEventHandler) HandleSanctionApplied(ctx context.Context, ev *eventsv1.SanctionApplied, recipientProfileID string) delivery.DeliveryDecision {
	_ = ctx
	if ev == nil || recipientProfileID == "" {
		return delivery.DeliveryDecision{}
	}
	router := h.Router
	if router == nil {
		router = delivery.DecideRouting
	}
	return router(delivery.DeliveryInput{
		Type:     delivery.TypeSystem,
		IsOnline: false,
	})
}
