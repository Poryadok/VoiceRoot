package delivery_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/notification/internal/delivery"
)

func TestFinalizeDecision_MutedChatBlocksNewMessage(t *testing.T) {
	base := delivery.DeliveryDecision{InApp: true, Push: true}
	in := delivery.DeliveryInput{
		RecipientProfileID: uuid.New(),
		Type:               delivery.TypeNewMessage,
	}
	settings := delivery.SettingsSnapshot{ChatMuted: true}
	out := delivery.FinalizeDecision(base, in, settings, delivery.QuietHoursSnapshot{})
	require.False(t, out.Push)
	require.False(t, out.InApp)
}

func TestFinalizeDecision_MentionOverridesMute(t *testing.T) {
	base := delivery.DeliveryDecision{InApp: true, Push: true}
	in := delivery.DeliveryInput{
		RecipientProfileID: uuid.New(),
		Type:               delivery.TypeMention,
	}
	settings := delivery.SettingsSnapshot{
		ChatMuted:            true,
		MentionOverridesMute: true,
	}
	out := delivery.FinalizeDecision(base, in, settings, delivery.QuietHoursSnapshot{})
	require.True(t, out.Push)
}

func TestFinalizeDecision_QuietHoursSuppressesPush(t *testing.T) {
	base := delivery.DeliveryDecision{InApp: true, Push: true}
	at := time.Date(2026, 6, 12, 23, 30, 0, 0, time.UTC)
	in := delivery.DeliveryInput{
		RecipientProfileID: uuid.New(),
		Type:               delivery.TypeNewMessage,
		At:                 at,
	}
	quiet := delivery.QuietHoursSnapshot{
		Enabled:   true,
		StartTime: "23:00",
		EndTime:   "08:00",
		Timezone:  "UTC",
	}
	out := delivery.FinalizeDecision(base, in, delivery.SettingsSnapshot{}, quiet)
	require.False(t, out.Push)
	require.True(t, out.InApp)
}
