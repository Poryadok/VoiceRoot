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

func TestHandleStoryCreated_mentionProfilesProduceNotificationDecisions(t *testing.T) {
	mentionedID := uuid.NewString()
	authorID := uuid.NewString()
	ev := &eventsv1.StoryCreated{
		StoryId:         uuid.NewString(),
		AuthorProfileId: authorID,
	}

	handler := &consumer.StoryEventHandler{}
	decisions := handler.HandleStoryCreated(context.Background(), ev, []string{mentionedID})
	require.NotNil(t, decisions)
	require.Contains(t, decisions, mentionedID)

	mention, ok := decisions[mentionedID]
	require.True(t, ok)
	require.True(t, mention.Push, "mentioned profile receives push for story mention")
	require.True(t, mention.InApp)
}

func TestHandleStoryCreated_usesEventMentionIDs(t *testing.T) {
	mentionedID := uuid.NewString()
	authorID := uuid.NewString()
	ev := &eventsv1.StoryCreated{
		StoryId:           uuid.NewString(),
		AuthorProfileId:   authorID,
		MentionProfileIds: []string{mentionedID},
	}

	handler := &consumer.StoryEventHandler{
		Router: func(in delivery.DeliveryInput) delivery.DeliveryDecision {
			return delivery.DeliveryDecision{InApp: true, Push: true}
		},
	}
	decisions := handler.HandleStoryCreated(context.Background(), ev, nil)
	require.Contains(t, decisions, mentionedID)
}
