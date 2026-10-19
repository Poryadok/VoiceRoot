package dispatch

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"voice/backend/notification/internal/delivery"
	"voice/backend/notification/internal/presence"
)

// EnrichDecisions applies presence and notification policy to raw routing decisions.
func EnrichDecisions(
	ctx context.Context,
	presenceChecker presence.Checker,
	policy delivery.DeliveryPolicyLoader,
	raw map[string]delivery.DeliveryDecision,
	senderID uuid.UUID,
	chatID string,
	typ delivery.NotificationType,
) (map[string]delivery.DeliveryDecision, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	if policy == nil {
		policy = delivery.PermissivePolicyLoader{}
	}
	out := make(map[string]delivery.DeliveryDecision, len(raw))
	for profileID := range raw {
		recipient, err := uuid.Parse(profileID)
		if err != nil {
			continue
		}
		isOnline := false
		if !delivery.SkipsPresenceCheck(typ) {
			if presenceChecker == nil {
				return nil, fmt.Errorf("notification routing presence authority is not configured")
			}
			isOnline, err = presenceChecker.IsOnline(ctx, recipient)
			if err != nil {
				return nil, fmt.Errorf("notification routing presence: %w", err)
			}
		}
		in := delivery.DeliveryInput{
			RecipientProfileID: recipient,
			SenderProfileID:    senderID,
			ChatID:             chatID,
			Type:               typ,
			IsOnline:           isOnline,
			At:                 time.Now().UTC(),
		}
		decision := delivery.DecideRouting(in)
		settings, quiet, err := policy.LoadPolicy(ctx, recipient, chatID, typ, in.At)
		if err != nil {
			return nil, err
		}
		out[profileID] = delivery.FinalizeDecision(decision, in, settings, quiet)
	}
	return out, nil
}
