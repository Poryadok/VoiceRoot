package main

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"voice/backend/notification/internal/consumer"
	"voice/backend/notification/internal/delivery"
	"voice/backend/notification/internal/dispatch"
	eventsv1 "voice.app/voice/events/v1"
)

func TestRouteStoryNotification_StoryCreatedMention(t *testing.T) {
	mentionedID := uuid.NewString()
	authorID := uuid.NewString()
	env := &eventsv1.StoryStreamEvent{
		Payload: &eventsv1.StoryStreamEvent_StoryCreated{
			StoryCreated: &eventsv1.StoryCreated{
				StoryId:           uuid.NewString(),
				AuthorProfileId:   authorID,
				MentionProfileIds: []string{mentionedID},
			},
		},
	}
	handler := &consumer.StoryEventHandler{Router: delivery.DecideRouting}
	pusher := &dispatch.StoryPusher{}
	err := routeStoryNotification(handler, pusher, env)
	require.NoError(t, err)
	decisions := handler.HandleStoryCreated(context.Background(), env.GetStoryCreated(), nil)
	require.Contains(t, decisions, mentionedID)
}

func TestRouteStoryNotification_unknownPayload(t *testing.T) {
	err := routeStoryNotification(nil, nil, &eventsv1.StoryStreamEvent{})
	require.NoError(t, err)
}

func TestStoryStreamEvent_roundTrip(t *testing.T) {
	ev := &eventsv1.StoryStreamEvent{
		Payload: &eventsv1.StoryStreamEvent_StoryCreated{
			StoryCreated: &eventsv1.StoryCreated{StoryId: uuid.NewString()},
		},
	}
	b, err := proto.Marshal(ev)
	require.NoError(t, err)
	var out eventsv1.StoryStreamEvent
	require.NoError(t, proto.Unmarshal(b, &out))
	require.NotNil(t, out.GetStoryCreated())
}
