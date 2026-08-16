package consumer

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"voice/backend/notification/internal/delivery"
	eventsv1 "voice.app/voice/events/v1"
)

// StoryEventHandler maps NATS story.events to notification delivery decisions.
type StoryEventHandler struct {
	Router func(in delivery.DeliveryInput) delivery.DeliveryDecision
}

func (h *StoryEventHandler) route(senderID, recipientID string, typ delivery.NotificationType) delivery.DeliveryDecision {
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

// HandleStoryCreated returns per-mentioned-profile delivery decisions for story.created.
func (h *StoryEventHandler) HandleStoryCreated(ctx context.Context, ev *eventsv1.StoryCreated, mentionProfileIDs []string) map[string]delivery.DeliveryDecision {
	_ = ctx
	if ev == nil {
		return nil
	}
	ids := mentionProfileIDs
	if len(ids) == 0 {
		ids = ev.GetMentionProfileIds()
	}
	out := make(map[string]delivery.DeliveryDecision)
	for _, profileID := range ids {
		if profileID == "" || profileID == ev.GetAuthorProfileId() {
			continue
		}
		out[profileID] = h.route(ev.GetAuthorProfileId(), profileID, delivery.TypeMention)
	}
	return out
}

// HandleStoryLfpResponse notifies the LFP author about JOIN/INVITE (matchmaking.md Social Discovery).
func (h *StoryEventHandler) HandleStoryLfpResponse(ctx context.Context, ev *eventsv1.StoryLfpResponse) map[string]delivery.DeliveryDecision {
	_ = ctx
	if ev == nil {
		return nil
	}
	authorID := ev.GetAuthorProfileId()
	responderID := ev.GetResponderProfileId()
	if authorID == "" || responderID == "" || authorID == responderID {
		return nil
	}
	typ := delivery.TypeLfpJoinRequest
	if strings.EqualFold(ev.GetResponseType(), "INVITE") {
		typ = delivery.TypeLfpInviteRequest
	}
	return map[string]delivery.DeliveryDecision{
		authorID: h.route(responderID, authorID, typ),
	}
}
