package consumer_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/notification/internal/consumer"
	"voice/backend/notification/internal/delivery"
	eventsv1 "voice.app/voice/events/v1"
)

func TestNotifySanctionType(t *testing.T) {
	require.True(t, consumer.NotifySanctionType("warning"))
	require.True(t, consumer.NotifySanctionType("temp_ban"))
	require.True(t, consumer.NotifySanctionType("perm_ban"))
	require.True(t, consumer.NotifySanctionType("mm_ban"))
	require.False(t, consumer.NotifySanctionType("shadow_ban"))
	require.False(t, consumer.NotifySanctionType(""))
}

func TestHandleSanctionApplied_WithProfiles(t *testing.T) {
	profileID := uuid.NewString()
	handler := &consumer.ModerationEventHandler{}
	ev := &eventsv1.SanctionApplied{
		SanctionId:      uuid.NewString(),
		TargetAccountId: uuid.NewString(),
		Type:            "warning",
	}
	got := handler.HandleSanctionApplied(context.Background(), ev, []string{profileID})
	require.Equal(t, map[string]delivery.DeliveryDecision{
		profileID: {InApp: true, Push: true},
	}, got)
}

func TestHandleSanctionApplied_EmptyProfilesNoOp(t *testing.T) {
	handler := &consumer.ModerationEventHandler{}
	ev := &eventsv1.SanctionApplied{Type: "warning", TargetAccountId: uuid.NewString()}
	require.Nil(t, handler.HandleSanctionApplied(context.Background(), ev, nil))
	require.Nil(t, handler.HandleSanctionApplied(context.Background(), ev, []string{""}))
}

func TestHandleSanctionApplied_ShadowBanSilent(t *testing.T) {
	handler := &consumer.ModerationEventHandler{}
	ev := &eventsv1.SanctionApplied{
		Type:            "shadow_ban",
		TargetAccountId: uuid.NewString(),
	}
	require.Nil(t, handler.HandleSanctionApplied(context.Background(), ev, []string{uuid.NewString()}))
}
