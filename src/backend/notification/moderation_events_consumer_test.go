package main

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/notification/internal/consumer"
	eventsv1 "voice.app/voice/events/v1"
)

func TestRouteModerationNotification_SanctionApplied(t *testing.T) {
	handler := &consumer.ModerationEventHandler{}
	env := &eventsv1.ModerationStreamEvent{
		Payload: &eventsv1.ModerationStreamEvent_SanctionApplied{
			SanctionApplied: &eventsv1.SanctionApplied{
				SanctionId: uuid.NewString(),
				TargetAccountId:  uuid.NewString(),
			},
		},
	}
	require.True(t, routeModerationNotification(handler, env))
}

func TestRouteModerationNotification_UnknownPayload(t *testing.T) {
	handler := &consumer.ModerationEventHandler{}
	env := &eventsv1.ModerationStreamEvent{}
	require.False(t, routeModerationNotification(handler, env))
}
