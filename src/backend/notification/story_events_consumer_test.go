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

func storyMentionRestartSpec() notificationConsumerRestartSpec {
	authorID, recipientID := uuid.New(), uuid.New()
	storyID := uuid.NewString()
	return notificationConsumerRestartSpec{
		name: "story_created_mention", service: "story", stream: jsStreamStoryEvents, filter: "story.>",
		deliverSubject: "_INBOX.voice.notification.story", subject: "story.created",
		recipientID: recipientID,
		event: &eventsv1.StoryStreamEvent{Payload: &eventsv1.StoryStreamEvent_StoryCreated{StoryCreated: &eventsv1.StoryCreated{
			StoryId: storyID, AuthorProfileId: authorID.String(), MentionProfileIds: []string{recipientID.String()},
		}}},
		expected: push.Payload{Title: "Story mention", Body: "You were mentioned in a story", Data: map[string]string{
			"type": "mention", "story_id": storyID, "sender_profile_id": authorID.String(),
		}},
		run: func(ctx context.Context, natsURL string, tokens *store.DeviceTokenStore, pusher *dispatch.PushDispatcher) error {
			return runStoryEventsConsumer(ctx, natsURL, tokens, &dispatch.StoryPusher{Tokens: tokens, Pusher: pusher}, nil)
		},
	}
}

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

func TestRouteStoryNotification_StoryLfpResponseJoin(t *testing.T) {
	authorID := uuid.NewString()
	responderID := uuid.NewString()
	storyID := uuid.NewString()
	env := &eventsv1.StoryStreamEvent{
		Payload: &eventsv1.StoryStreamEvent_StoryLfpResponse{
			StoryLfpResponse: &eventsv1.StoryLfpResponse{
				StoryId:            storyID,
				AuthorProfileId:    authorID,
				ResponderProfileId: responderID,
				ResponseType:       "JOIN",
			},
		},
	}
	handler := &consumer.StoryEventHandler{Router: delivery.DecideRouting}
	pusher := &dispatch.StoryPusher{}
	err := routeStoryNotification(handler, pusher, env)
	require.NoError(t, err)
	decisions := handler.HandleStoryLfpResponse(context.Background(), env.GetStoryLfpResponse())
	require.Contains(t, decisions, authorID)
	require.NotContains(t, decisions, responderID)

	title, body, data := lfpPushCopy(env.GetStoryLfpResponse())
	require.Equal(t, "LFP join request", title)
	require.NotEmpty(t, body)
	require.Equal(t, "lfp_accept", data["action_accept"])
	require.Equal(t, "lfp_decline", data["action_decline"])
	require.Equal(t, storyID, data["story_id"])
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
