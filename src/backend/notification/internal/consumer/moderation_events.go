package consumer

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"voice/backend/notification/internal/delivery"
	eventsv1 "voice.app/voice/events/v1"
)

// ModerationEventHandler maps moderation.events to notification delivery decisions.
type ModerationEventHandler struct {
	Router func(in delivery.DeliveryInput) delivery.DeliveryDecision
}

// NotifySanctionType reports whether the sanctioned account should be notified.
// shadow_ban must stay silent (docs/features/reports.md).
func NotifySanctionType(sanctionType string) bool {
	switch strings.ToLower(strings.TrimSpace(sanctionType)) {
	case "shadow_ban":
		return false
	case "warning", "temp_ban", "perm_ban", "mm_ban":
		return true
	default:
		// Unknown types: still notify as system so operators are not silent-fail.
		return sanctionType != ""
	}
}

// HandleSanctionApplied returns per-profile system delivery decisions for a sanction.
func (h *ModerationEventHandler) HandleSanctionApplied(
	ctx context.Context,
	ev *eventsv1.SanctionApplied,
	recipientProfileIDs []string,
) map[string]delivery.DeliveryDecision {
	_ = ctx
	if ev == nil || !NotifySanctionType(ev.GetType()) || len(recipientProfileIDs) == 0 {
		return nil
	}
	out := make(map[string]delivery.DeliveryDecision, len(recipientProfileIDs))
	for _, profileID := range recipientProfileIDs {
		profileID = strings.TrimSpace(profileID)
		if profileID == "" {
			continue
		}
		out[profileID] = h.route(profileID)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func (h *ModerationEventHandler) route(recipientID string) delivery.DeliveryDecision {
	router := h.Router
	if router == nil {
		router = delivery.DecideRouting
	}
	recipientUUID, _ := uuid.Parse(recipientID)
	return router(delivery.DeliveryInput{
		RecipientProfileID: recipientUUID,
		Type:               delivery.TypeSystem,
		IsOnline:           false,
	})
}
