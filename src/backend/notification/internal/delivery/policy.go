package delivery

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// DeliveryPolicyLoader loads effective settings for push routing.
type DeliveryPolicyLoader interface {
	LoadPolicy(ctx context.Context, profileID uuid.UUID, chatID string, typ NotificationType, at time.Time) (SettingsSnapshot, QuietHoursSnapshot, error)
}

// PermissivePolicyLoader returns default-open settings until notification_settings store is wired.
type PermissivePolicyLoader struct{}

func (PermissivePolicyLoader) LoadPolicy(
	context.Context,
	uuid.UUID,
	string,
	NotificationType,
	time.Time,
) (SettingsSnapshot, QuietHoursSnapshot, error) {
	return SettingsSnapshot{MentionOverridesMute: true}, QuietHoursSnapshot{}, nil
}

// FinalizeDecision applies settings and quiet hours to a base routing decision.
func FinalizeDecision(base DeliveryDecision, in DeliveryInput, settings SettingsSnapshot, quiet QuietHoursSnapshot) DeliveryDecision {
	out := ApplySettings(base, in, settings)
	return ApplyQuietHours(out, in, quiet)
}
