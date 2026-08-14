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
	route, err := routeVoiceNotification(
		t.Context(),
		handler,
		nil,
		delivery.PermissivePolicyLoader{},
		env,
		func(string) bool { return false },
	)
	require.NoError(t, err)
	require.NotNil(t, route)
	require.True(t, route.decisions[callee].Push)
	require.Equal(t, string(delivery.TypeIncomingCall), route.payload.Data["type"])
	require.Equal(t, roomID, route.payload.Data["room_id"])
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
	route, err := routeVoiceNotification(
		t.Context(),
		handler,
		nil,
		delivery.PermissivePolicyLoader{},
		env,
		func(profileID string) bool { return profileID == callee },
	)
	require.NoError(t, err)
	require.NotNil(t, route)
	require.False(t, route.decisions[callee].Push)
}

func TestRouteVoiceNotification_VoiceMemberJoined(t *testing.T) {
	notifyID := uuid.NewString()
	joinedID := uuid.NewString()
	env := &eventsv1.VoiceStreamEvent{
		Payload: &eventsv1.VoiceStreamEvent_VoiceMemberJoined{
			VoiceMemberJoined: &eventsv1.VoiceMemberJoined{
				RoomId:           uuid.NewString(),
				VoiceRoomId:      uuid.NewString(),
				SpaceId:          uuid.NewString(),
				JoinedProfileId:  joinedID,
				NotifyProfileIds: []string{notifyID},
			},
		},
	}
	handler := &consumer.VoiceEventHandler{Router: delivery.DecideRouting}
	route, err := routeVoiceNotification(
		t.Context(),
		handler,
		nil,
		delivery.PermissivePolicyLoader{},
		env,
		func(string) bool { return false },
	)
	require.NoError(t, err)
	require.NotNil(t, route)
	require.True(t, route.decisions[notifyID].Push)
	require.Equal(t, string(delivery.TypeVoiceMemberJoined), route.payload.Data["type"])
}

func TestRouteVoiceNotification_IgnoresOtherEvents(t *testing.T) {
	env := &eventsv1.VoiceStreamEvent{
		Payload: &eventsv1.VoiceStreamEvent_CallEnded{
			CallEnded: &eventsv1.CallEnded{RoomId: uuid.NewString()},
		},
	}
	handler := &consumer.VoiceEventHandler{Router: delivery.DecideRouting}
	route, err := routeVoiceNotification(
		t.Context(),
		handler,
		nil,
		delivery.PermissivePolicyLoader{},
		env,
		func(string) bool { return false },
	)
	require.NoError(t, err)
	require.Nil(t, route)
}
