package main

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"voice/backend/notification/internal/consumer"
	"voice/backend/notification/internal/delivery"
	eventsv1 "voice.app/voice/events/v1"
)

func TestRouteMatchmakingNotification_SearchTimeout(t *testing.T) {
	profileID := uuid.NewString()
	env := &eventsv1.MatchmakingStreamEvent{
		Payload: &eventsv1.MatchmakingStreamEvent_MatchTimeout{
			MatchTimeout: &eventsv1.MatchTimeout{
				SessionId: uuid.NewString(),
				ProfileId: profileID,
				GameId:    uuid.NewString(),
				Mode:      "Duo",
			},
		},
	}
	handler := &consumer.MatchmakingEventHandler{Router: delivery.DecideRouting}
	decisions, payload, ok := routeMatchmakingNotification(handler, env)
	require.True(t, ok)
	require.Len(t, decisions, 1)
	require.True(t, decisions[profileID].Push)
	require.Equal(t, string(delivery.TypeSearchTimeout), payload.Data["type"])
}

func TestRouteMatchmakingNotification_SearchNudgeProto(t *testing.T) {
	profileID := uuid.NewString()
	b, err := proto.Marshal(&eventsv1.MatchmakingStreamEvent{
		Payload: &eventsv1.MatchmakingStreamEvent_SearchNudge{
			SearchNudge: &eventsv1.SearchNudge{
				SessionId: uuid.NewString(),
				ProfileId: profileID,
				GameId:    uuid.NewString(),
				Mode:      "Duo",
			},
		},
	})
	require.NoError(t, err)

	var env eventsv1.MatchmakingStreamEvent
	require.NoError(t, proto.Unmarshal(b, &env))
	handler := &consumer.MatchmakingEventHandler{Router: delivery.DecideRouting}
	decisions, payload, ok := routeMatchmakingNotification(handler, &env)
	require.True(t, ok)
	require.Len(t, decisions, 1)
	require.Equal(t, string(delivery.TypeSearchNudge), payload.Data["type"])
}
