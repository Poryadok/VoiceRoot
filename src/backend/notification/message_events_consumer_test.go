package main

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	eventsv1 "voice.app/voice/events/v1"
	"voice/backend/notification/internal/chatmembers"
	"voice/backend/notification/internal/consumer"
	"voice/backend/notification/internal/delivery"
	"voice/backend/notification/internal/dispatch"
	"voice/backend/notification/internal/grouping"
	"voice/backend/notification/internal/pushenrich"
)

type stubChatMembers struct {
	ids []string
}

func (s stubChatMembers) ListMemberProfileIDs(context.Context, string) ([]string, error) {
	return s.ids, nil
}

func (s stubChatMembers) ListMembers(_ context.Context, _ string) ([]chatmembers.Member, error) {
	out := make([]chatmembers.Member, 0, len(s.ids))
	for _, id := range s.ids {
		out = append(out, chatmembers.Member{ProfileID: id, InboxBucket: "main"})
	}
	return out, nil
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
	err := routeMessageNotification(context.Background(), handler, stubChatMembers{ids: []string{senderID, recipientID}}, pusher, pushenrich.NoopResolver{}, env)
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
	err := routeMessageNotification(context.Background(), handler, chatmembers.NoopLister{}, pusher, pushenrich.NoopResolver{}, env)
	require.NoError(t, err)
}

func TestRouteMessageNotification_UnknownPayload(t *testing.T) {
	err := routeMessageNotification(context.Background(), nil, nil, nil, pushenrich.NoopResolver{}, &eventsv1.MessageStreamEvent{})
	require.NoError(t, err)
}

func TestMessagePushDeepLink(t *testing.T) {
	require.Equal(t, "https://voice.gg/ch/c1/m/m1", messagePushDeepLink("c1", "m1"))
	require.Equal(t, "https://voice.gg/ch/c1", messagePushDeepLink("c1", ""))
}

func TestMessageEventsJetStreamSubjectPrefix(t *testing.T) {
	t.Parallel()
	// Messaging JetStreamPublisher uses message.sent, message.edited, … (not msg.*).
	require.Equal(t, "message.>", jsSubjectMessageEvents)
}

func TestDeliveryForMember_ArchivedRecipientSuppressesPush(t *testing.T) {
	for _, chatKind := range []string{"dm", "group", "channel"} {
		t.Run(chatKind, func(t *testing.T) {
			decision := deliveryForMember(map[string]delivery.DeliveryDecision{"recipient": {InApp: true, Push: true}}, chatmembers.Member{
				ProfileID:  uuid.NewString(),
				IsArchived: true,
			})["recipient"]

			require.True(t, decision.InApp, "archiving must not suppress unread/in-app handling")
			require.False(t, decision.Push, "archived %s recipient must not receive push", chatKind)
		})
	}
}

func TestDeliveryForMember_ActiveRecipientPreservesRouting(t *testing.T) {
	decision := deliveryForMember(map[string]delivery.DeliveryDecision{"recipient": {InApp: true, Push: true}}, chatmembers.Member{
		ProfileID: uuid.NewString(),
	})["recipient"]

	require.True(t, decision.InApp)
	require.True(t, decision.Push)
}
