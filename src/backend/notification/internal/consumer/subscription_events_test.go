package consumer

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	eventsv1 "voice.app/voice/events/v1"
)

// TestSubscriptionEventHandler_GraceReminderRecordsStub documents SUB-04: grace D1/D3/D7 → notification stub.
func TestSubscriptionEventHandler_GraceReminderRecordsStub(t *testing.T) {
	h := &SubscriptionEventHandler{}
	got := h.HandleGraceReminder(context.Background(), &eventsv1.GraceReminder{
		AccountId: "11111111-1111-4111-8111-111111111111",
		Plan:      "premium",
		Day:       3,
	})
	require.True(t, got.Handled)
	require.Equal(t, "11111111-1111-4111-8111-111111111111", got.AccountID)
	require.Equal(t, "premium", got.Plan)
	require.EqualValues(t, 3, got.Day)
	require.Equal(t, "grace_reminder", got.Kind)
}

func TestSubscriptionEventHandler_GraceReminderNilIgnored(t *testing.T) {
	h := &SubscriptionEventHandler{}
	got := h.HandleGraceReminder(context.Background(), nil)
	require.False(t, got.Handled)
}
