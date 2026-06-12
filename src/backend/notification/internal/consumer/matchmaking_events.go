package consumer

import (
	"context"

	"github.com/google/uuid"

	"voice/backend/notification/internal/delivery"
	eventsv1 "voice.app/voice/events/v1"
)

// MatchmakingEventHandler maps matchmaking.events to notification delivery decisions.
type MatchmakingEventHandler struct {
	Router func(in delivery.DeliveryInput) delivery.DeliveryDecision
}

func (h *MatchmakingEventHandler) route(recipientID string, typ delivery.NotificationType) delivery.DeliveryDecision {
	router := h.Router
	if router == nil {
		router = delivery.DecideRouting
	}
	recipientUUID, _ := uuid.Parse(recipientID)
	return router(delivery.DeliveryInput{
		RecipientProfileID: recipientUUID,
		Type:               typ,
		IsOnline:           false,
	})
}

// HandleMatchFound returns per-participant delivery decisions for a MatchFound event.
func (h *MatchmakingEventHandler) HandleMatchFound(ctx context.Context, ev *eventsv1.MatchFound) map[string]delivery.DeliveryDecision {
	_ = ctx
	if ev == nil {
		return nil
	}
	out := make(map[string]delivery.DeliveryDecision)
	for _, profileID := range ev.GetProfileIds() {
		if profileID == "" {
			continue
		}
		decision := h.route(profileID, delivery.TypeMatchFound)
		out[profileID] = decision
	}
	return out
}

// HandleSearchNudge returns delivery decision for a search nudge event.
func (h *MatchmakingEventHandler) HandleSearchNudge(ctx context.Context, ev *eventsv1.SearchNudge) map[string]delivery.DeliveryDecision {
	_ = ctx
	if ev == nil || ev.GetProfileId() == "" {
		return nil
	}
	return map[string]delivery.DeliveryDecision{
		ev.GetProfileId(): h.route(ev.GetProfileId(), delivery.TypeSearchNudge),
	}
}

// HandleSearchTimeout returns delivery decision for a search timeout event.
func (h *MatchmakingEventHandler) HandleSearchTimeout(ctx context.Context, ev *eventsv1.MatchTimeout) map[string]delivery.DeliveryDecision {
	_ = ctx
	if ev == nil || ev.GetProfileId() == "" {
		return nil
	}
	return map[string]delivery.DeliveryDecision{
		ev.GetProfileId(): h.route(ev.GetProfileId(), delivery.TypeSearchTimeout),
	}
}
