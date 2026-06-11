package delivery_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/notification/internal/delivery"
)

func TestApplyQuietHours_BlocksPush(t *testing.T) {
	in := delivery.DeliveryInput{
		RecipientProfileID: uuid.New(),
		SenderProfileID:    uuid.New(),
		ChatID:             uuid.NewString(),
		Type:               delivery.TypeNewMessage,
		IsOnline:           false,
		At:                 time.Date(2026, 6, 11, 23, 30, 0, 0, time.UTC),
	}
	quiet := delivery.QuietHoursSnapshot{
		Enabled:          true,
		StartTime:        "23:00",
		EndTime:          "08:00",
		Timezone:         "UTC",
		OverrideMentions: true,
		At:               in.At,
	}
	base := delivery.DeliveryDecision{InApp: true, Push: true}
	out := delivery.ApplyQuietHours(base, in, quiet)
	require.True(t, out.InApp, "in-app may still deliver during DND")
	require.False(t, out.Push, "DND blocks push for new_message")
}

func TestApplyQuietHours_SameDayWindow(t *testing.T) {
	in := delivery.DeliveryInput{
		Type: delivery.TypeNewMessage,
		At:   time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC),
	}
	quiet := delivery.QuietHoursSnapshot{
		Enabled:   true,
		StartTime: "09:00",
		EndTime:   "17:00",
		Timezone:  "UTC",
	}
	base := delivery.DeliveryDecision{InApp: true, Push: true}
	out := delivery.ApplyQuietHours(base, in, quiet)
	require.False(t, out.Push)
}

func TestApplyQuietHours_NonUTCTimezone(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	require.NoError(t, err)
	in := delivery.DeliveryInput{
		Type: delivery.TypeNewMessage,
		At:   time.Date(2026, 6, 11, 4, 30, 0, 0, time.UTC), // 00:30 EDT
	}
	quiet := delivery.QuietHoursSnapshot{
		Enabled:   true,
		StartTime: "23:00",
		EndTime:   "08:00",
		Timezone:  loc.String(),
	}
	base := delivery.DeliveryDecision{InApp: true, Push: true}
	out := delivery.ApplyQuietHours(base, in, quiet)
	require.False(t, out.Push)
}

func TestApplyQuietHours_DisabledPassesThrough(t *testing.T) {
	base := delivery.DeliveryDecision{InApp: true, Push: true}
	in := delivery.DeliveryInput{Type: delivery.TypeNewMessage, At: time.Now().UTC()}
	quiet := delivery.QuietHoursSnapshot{Enabled: false}
	out := delivery.ApplyQuietHours(base, in, quiet)
	require.True(t, out.Push)
}

func TestApplyQuietHours_OutsideWindowAllowsPush(t *testing.T) {
	in := delivery.DeliveryInput{
		Type: delivery.TypeNewMessage,
		At:   time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC),
	}
	quiet := delivery.QuietHoursSnapshot{
		Enabled:   true,
		StartTime: "23:00",
		EndTime:   "08:00",
		Timezone:  "UTC",
	}
	base := delivery.DeliveryDecision{InApp: true, Push: true}
	out := delivery.ApplyQuietHours(base, in, quiet)
	require.True(t, out.Push)
}

func TestApplyQuietHours_MentionWithoutOverrideBlocked(t *testing.T) {
	in := delivery.DeliveryInput{
		Type: delivery.TypeMention,
		At:   time.Date(2026, 6, 11, 23, 30, 0, 0, time.UTC),
	}
	quiet := delivery.QuietHoursSnapshot{
		Enabled:          true,
		StartTime:        "23:00",
		EndTime:          "08:00",
		Timezone:         "UTC",
		OverrideMentions: false,
	}
	base := delivery.DeliveryDecision{InApp: true, Push: true}
	out := delivery.ApplyQuietHours(base, in, quiet)
	require.False(t, out.Push)
}

func TestApplyQuietHours_UsesQuietAtOverride(t *testing.T) {
	in := delivery.DeliveryInput{
		Type: delivery.TypeNewMessage,
		At:   time.Date(2026, 6, 11, 10, 0, 0, 0, time.UTC),
	}
	quietAt := time.Date(2026, 6, 11, 23, 30, 0, 0, time.UTC)
	quiet := delivery.QuietHoursSnapshot{
		Enabled:   true,
		StartTime: "23:00",
		EndTime:   "08:00",
		Timezone:  "UTC",
		At:        quietAt,
	}
	base := delivery.DeliveryDecision{InApp: true, Push: true}
	out := delivery.ApplyQuietHours(base, in, quiet)
	require.False(t, out.Push)
}

func TestApplyQuietHours_InvalidScheduleAllowsPush(t *testing.T) {
	in := delivery.DeliveryInput{
		Type: delivery.TypeNewMessage,
		At:   time.Date(2026, 6, 11, 23, 30, 0, 0, time.UTC),
	}
	quiet := delivery.QuietHoursSnapshot{
		Enabled:   true,
		StartTime: "bad",
		EndTime:   "08:00",
		Timezone:  "UTC",
	}
	base := delivery.DeliveryDecision{InApp: true, Push: true}
	out := delivery.ApplyQuietHours(base, in, quiet)
	require.True(t, out.Push)
}

func TestApplyQuietHours_MalformedMinuteAllowsPush(t *testing.T) {
	in := delivery.DeliveryInput{
		Type: delivery.TypeNewMessage,
		At:   time.Date(2026, 6, 11, 23, 30, 0, 0, time.UTC),
	}
	quiet := delivery.QuietHoursSnapshot{
		Enabled:   true,
		StartTime: "23:xx",
		EndTime:   "08:00",
		Timezone:  "UTC",
	}
	base := delivery.DeliveryDecision{InApp: true, Push: true}
	out := delivery.ApplyQuietHours(base, in, quiet)
	require.True(t, out.Push)
}

func TestApplyQuietHours_OverrideMentionsAllowsMention(t *testing.T) {
	in := delivery.DeliveryInput{
		RecipientProfileID: uuid.New(),
		SenderProfileID:    uuid.New(),
		ChatID:             uuid.NewString(),
		Type:               delivery.TypeMention,
		IsOnline:           false,
		At:                 time.Date(2026, 6, 11, 23, 30, 0, 0, time.UTC),
	}
	quiet := delivery.QuietHoursSnapshot{
		Enabled:          true,
		StartTime:        "23:00",
		EndTime:          "08:00",
		Timezone:         "UTC",
		OverrideMentions: true,
		At:               in.At,
	}
	base := delivery.DeliveryDecision{InApp: true, Push: true}
	out := delivery.ApplyQuietHours(base, in, quiet)
	require.True(t, out.Push, "override_mentions allows mention push during DND")
}
