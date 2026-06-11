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

func TestHandleMatchFound_RoutesPushAndInAppToParticipants(t *testing.T) {
	profileA := uuid.NewString()
	profileB := uuid.NewString()
	matchID := uuid.NewString()

	h := &consumer.MatchmakingEventHandler{
		Router: delivery.DecideRouting,
	}
	ev := &eventsv1.MatchFound{
		MatchId:    matchID,
		ProfileIds: []string{profileA, profileB},
		GameId:     uuid.NewString(),
		Mode:       "Duo",
		Region:     "eu",
		SessionIds: []string{uuid.NewString(), uuid.NewString()},
	}
	decisions := h.HandleMatchFound(context.Background(), ev)
	require.Len(t, decisions, 2)

	for _, profileID := range []string{profileA, profileB} {
		d, ok := decisions[profileID]
		require.True(t, ok, "profile %s must receive routing decision", profileID)
		require.True(t, d.InApp, "match_found in-app for %s", profileID)
		require.True(t, d.Push, "match_found push for %s", profileID)
	}
}

func TestHandleMatchFound_NilEvent(t *testing.T) {
	h := &consumer.MatchmakingEventHandler{}
	require.Nil(t, h.HandleMatchFound(context.Background(), nil))
}

func TestHandleMatchFound_SkipsEmptyProfileIDs(t *testing.T) {
	h := &consumer.MatchmakingEventHandler{Router: delivery.DecideRouting}
	ev := &eventsv1.MatchFound{
		MatchId:    uuid.NewString(),
		ProfileIds: []string{"", uuid.NewString()},
		GameId:     uuid.NewString(),
		Mode:       "Duo",
		Region:     "eu",
	}
	decisions := h.HandleMatchFound(context.Background(), ev)
	require.Len(t, decisions, 1)
}
