package main

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	eventsv1 "voice.app/voice/events/v1"
	"voice/backend/notification/internal/consumer"
	"voice/backend/notification/internal/delivery"
	"voice/backend/notification/internal/dispatch"
	"voice/backend/notification/internal/push"
	"voice/backend/notification/internal/store"
)

func matchFoundRestartSpec() notificationConsumerRestartSpec {
	recipientID := uuid.New()
	matchID, gameID := uuid.NewString(), uuid.NewString()
	return notificationConsumerRestartSpec{
		name: "matchmaking_match_found", service: "matchmaking", stream: jsStreamMatchmakingEvents, filter: "mm.>",
		deliverSubject: "_INBOX.voice.notification.matchmaking", subject: "mm.match_found",
		recipientID: recipientID,
		event: &eventsv1.MatchmakingStreamEvent{Payload: &eventsv1.MatchmakingStreamEvent_MatchFound{MatchFound: &eventsv1.MatchFound{
			MatchId: matchID, ProfileIds: []string{recipientID.String()}, GameId: gameID, Mode: "Duo",
		}}},
		expected: push.Payload{Title: "Match found", Body: "A match is ready — accept or decline", Data: map[string]string{
			"type": "match_found", "match_id": matchID, "game_id": gameID, "mode": "Duo",
		}},
		run: func(ctx context.Context, natsURL string, tokens *store.DeviceTokenStore, pusher *dispatch.PushDispatcher) error {
			return runMatchmakingEventsConsumer(ctx, natsURL, tokens, pusher, nil, nil, nil)
		},
	}
}

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
	decisions, payload, typ, ok := routeMatchmakingNotification(handler, env)
	require.True(t, ok)
	require.Equal(t, delivery.TypeSearchTimeout, typ)
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
	decisions, payload, typ, ok := routeMatchmakingNotification(handler, &env)
	require.True(t, ok)
	require.Equal(t, delivery.TypeSearchNudge, typ)
	require.Len(t, decisions, 1)
	require.Equal(t, string(delivery.TypeSearchNudge), payload.Data["type"])
}
