package delivery

import "github.com/google/uuid"

// DecideRouting applies presence and sender exclusion rules.
// Online recipients get in-app only; offline get push. Sender never receives own notification.
func DecideRouting(in DeliveryInput) DeliveryDecision {
	if in.RecipientProfileID == in.SenderProfileID && in.SenderProfileID != uuid.Nil {
		return DeliveryDecision{}
	}
	if in.IsOnline {
		return DeliveryDecision{InApp: true, Push: false}
	}
	return DeliveryDecision{InApp: true, Push: true}
}
