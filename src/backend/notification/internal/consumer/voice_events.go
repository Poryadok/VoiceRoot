package consumer

import (
	"context"
	"time"

	"github.com/google/uuid"

	"voice/backend/notification/internal/delivery"
	eventsv1 "voice.app/voice/events/v1"
)

// VoiceEventHandler maps voice.events to notification delivery decisions.
type VoiceEventHandler struct {
	Router func(in delivery.DeliveryInput) delivery.DeliveryDecision
}

func (h *VoiceEventHandler) routeCallIncoming(
	recipientID, initiatorID string,
	isOnline bool,
) delivery.DeliveryDecision {
	router := h.Router
	if router == nil {
		router = delivery.DecideRouting
	}
	recipientUUID, _ := uuid.Parse(recipientID)
	initiatorUUID, _ := uuid.Parse(initiatorID)
	return router(delivery.DeliveryInput{
		RecipientProfileID: recipientUUID,
		SenderProfileID:    initiatorUUID,
		Type:               delivery.TypeIncomingCall,
		IsOnline:           isOnline,
		At:                 time.Now().UTC(),
	})
}

// HandleCallIncoming returns delivery decision for the callee of an incoming DM call.
func (h *VoiceEventHandler) HandleCallIncoming(
	ctx context.Context,
	ev *eventsv1.CallIncoming,
	isOnline bool,
) map[string]delivery.DeliveryDecision {
	_ = ctx
	if ev == nil || ev.GetCalleeProfileId() == "" {
		return nil
	}
	decision := h.routeCallIncoming(ev.GetCalleeProfileId(), ev.GetInitiatorProfileId(), isOnline)
	return map[string]delivery.DeliveryDecision{
		ev.GetCalleeProfileId(): decision,
	}
}
