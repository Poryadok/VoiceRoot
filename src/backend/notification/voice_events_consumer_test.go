package main

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	"voice/backend/notification/internal/consumer"
	"voice/backend/notification/internal/delivery"
	eventsv1 "voice.app/voice/events/v1"
)

func TestRouteVoiceNotification_CallIncomingOffline(t *testing.T) {
	callee := uuid.NewString()
	initiator := uuid.NewString()
	roomID := uuid.NewString()
	env := &eventsv1.VoiceStreamEvent{
		Payload: &eventsv1.VoiceStreamEvent_CallIncoming{
			CallIncoming: &eventsv1.CallIncoming{
				RoomId:              roomID,
				ChatId:              uuid.NewString(),
				InitiatorProfileId:  initiator,
				CalleeProfileId:     callee,
				MediaKind:           "audio",
				LivekitRoomName:     "lk-" + roomID,
				ExpiresAt:           timestamppb.New(time.Now().UTC().Add(30 * time.Second)),
			},
		},
	}
	handler := &consumer.VoiceEventHandler{Router: delivery.DecideRouting}
	decisions, payload, ok := routeVoiceNotification(handler, env, false)
	require.True(t, ok)
	require.True(t, decisions[callee].Push)
	require.Equal(t, string(delivery.TypeIncomingCall), payload.Data["type"])
	require.Equal(t, roomID, payload.Data["room_id"])
	require.Equal(t, initiator, payload.Data["initiator_profile_id"])
	require.Equal(t, "audio", payload.Data["media_kind"])
}

func TestRouteVoiceNotification_CallIncomingOnlineNoPush(t *testing.T) {
	callee := uuid.NewString()
	env := &eventsv1.VoiceStreamEvent{
		Payload: &eventsv1.VoiceStreamEvent_CallIncoming{
			CallIncoming: &eventsv1.CallIncoming{
				RoomId:             uuid.NewString(),
				InitiatorProfileId: uuid.NewString(),
				CalleeProfileId:    callee,
			},
		},
	}
	handler := &consumer.VoiceEventHandler{Router: delivery.DecideRouting}
	decisions, _, ok := routeVoiceNotification(handler, env, true)
	require.True(t, ok)
	require.False(t, decisions[callee].Push)
}

func TestRouteVoiceNotification_IgnoresOtherEvents(t *testing.T) {
	env := &eventsv1.VoiceStreamEvent{
		Payload: &eventsv1.VoiceStreamEvent_CallEnded{
			CallEnded: &eventsv1.CallEnded{RoomId: uuid.NewString()},
		},
	}
	handler := &consumer.VoiceEventHandler{Router: delivery.DecideRouting}
	_, _, ok := routeVoiceNotification(handler, env, false)
	require.False(t, ok)
}
