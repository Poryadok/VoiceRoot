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

func (h *VoiceEventHandler) route(
	recipientID, senderID string,
	typ delivery.NotificationType,
	isOnline bool,
) delivery.DeliveryDecision {
	router := h.Router
	if router == nil {
		router = delivery.DecideRouting
	}
	recipientUUID, _ := uuid.Parse(recipientID)
	senderUUID, _ := uuid.Parse(senderID)
	return router(delivery.DeliveryInput{
		RecipientProfileID: recipientUUID,
		SenderProfileID:    senderUUID,
		Type:               typ,
		IsOnline:           isOnline,
		At:                 time.Now().UTC(),
	})
}

func (h *VoiceEventHandler) routeCallIncoming(
	recipientID, initiatorID string,
	isOnline bool,
) delivery.DeliveryDecision {
	return h.route(recipientID, initiatorID, delivery.TypeIncomingCall, isOnline)
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

// HandleVoiceMemberJoined returns delivery decisions for profiles that should be notified.
func (h *VoiceEventHandler) HandleVoiceMemberJoined(
	ctx context.Context,
	ev *eventsv1.VoiceMemberJoined,
	isOnlineByProfile func(profileID string) bool,
) map[string]delivery.DeliveryDecision {
	_ = ctx
	if ev == nil || ev.GetJoinedProfileId() == "" {
		return nil
	}
	out := make(map[string]delivery.DeliveryDecision)
	for _, profileID := range ev.GetNotifyProfileIds() {
		if profileID == "" || profileID == ev.GetJoinedProfileId() {
			continue
		}
		isOnline := false
		if isOnlineByProfile != nil && !delivery.SkipsPresenceCheck(delivery.TypeVoiceMemberJoined) {
			isOnline = isOnlineByProfile(profileID)
		}
		out[profileID] = h.route(profileID, ev.GetJoinedProfileId(), delivery.TypeVoiceMemberJoined, isOnline)
	}
	return out
}
