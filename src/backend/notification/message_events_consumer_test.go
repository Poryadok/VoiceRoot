package main

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/notification/internal/chatmembers"
	"voice/backend/notification/internal/consumer"
	"voice/backend/notification/internal/delivery"
	"voice/backend/notification/internal/dispatch"
	"voice/backend/notification/internal/grouping"
	eventsv1 "voice.app/voice/events/v1"
)

type stubChatMembers struct {
	ids []string
}

func (s stubChatMembers) ListMemberProfileIDs(context.Context, string) ([]string, error) {
	return s.ids, nil
}

func TestRouteMessageNotification_MessageSent(t *testing.T) {
	senderID := uuid.NewString()
	recipientID := uuid.NewString()
	handler := &consumer.MessageEventHandler{Router: delivery.DecideRouting}
	pusher := &dispatch.MessagePusher{Grouping: grouping.NewMemoryStore()}
	env := &eventsv1.MessageStreamEvent{
		Payload: &eventsv1.MessageStreamEvent_MessageSent{
			MessageSent: &eventsv1.MessageSent{
				MessageId:       uuid.NewString(),
				ChatId:          uuid.NewString(),
				SenderProfileId: senderID,
			},
		},
	}
	err := routeMessageNotification(context.Background(), handler, stubChatMembers{ids: []string{senderID, recipientID}}, pusher, env)
	require.NoError(t, err)
}

func TestRouteMessageNotification_MentionAdded(t *testing.T) {
	senderID := uuid.NewString()
	mentionedID := uuid.NewString()
	handler := &consumer.MessageEventHandler{Router: delivery.DecideRouting}
	pusher := &dispatch.MessagePusher{Grouping: grouping.NewMemoryStore()}
	env := &eventsv1.MessageStreamEvent{
		Payload: &eventsv1.MessageStreamEvent_MentionAdded{
			MentionAdded: &eventsv1.MentionAdded{
				MessageId:           uuid.NewString(),
				ChatId:              uuid.NewString(),
				SenderProfileId:     senderID,
				MentionedProfileIds: []string{mentionedID},
			},
		},
	}
	err := routeMessageNotification(context.Background(), handler, chatmembers.NoopLister{}, pusher, env)
	require.NoError(t, err)
}

func TestRouteMessageNotification_UnknownPayload(t *testing.T) {
	err := routeMessageNotification(context.Background(), nil, nil, nil, &eventsv1.MessageStreamEvent{})
	require.NoError(t, err)
}
