package delivery

import "github.com/google/uuid"

// SkipsPresenceCheck reports the event types that must evaluate push policy
// without querying recipient presence.
func SkipsPresenceCheck(typ NotificationType) bool {
	return typ == TypeMatchFound || typ == TypeVoiceMemberJoined
}

// DecideRouting applies presence and sender exclusion rules.
// Online recipients get in-app only; offline get push. Sender never receives own notification.
func DecideRouting(in DeliveryInput) DeliveryDecision {
	if in.RecipientProfileID == in.SenderProfileID && in.SenderProfileID != uuid.Nil {
		return DeliveryDecision{}
	}
	if SkipsPresenceCheck(in.Type) {
		return DeliveryDecision{InApp: true, Push: true}
	}
	if in.IsOnline {
		return DeliveryDecision{InApp: true, Push: false}
	}
	return DeliveryDecision{InApp: true, Push: true}
}
