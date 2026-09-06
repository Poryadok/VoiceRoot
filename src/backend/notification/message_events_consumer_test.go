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
	"voice/backend/notification/internal/fcm"
	"voice/backend/notification/internal/grouping"
	"voice/backend/notification/internal/push"
	"voice/backend/notification/internal/pushenrich"
	"voice/backend/notification/internal/store"
)

type stubChatMembers struct {
	ids  []string
	rows []chatmembers.Member
}

func (s stubChatMembers) ListMemberProfileIDs(context.Context, string) ([]string, error) {
	return s.ids, nil
}

func (s stubChatMembers) ListMembers(_ context.Context, _ string) ([]chatmembers.Member, error) {
	if s.rows != nil {
		return s.rows, nil
	}
	out := make([]chatmembers.Member, 0, len(s.ids))
	for _, id := range s.ids {
		out = append(out, chatmembers.Member{ProfileID: id, InboxBucket: "main"})
	}
	return out, nil
}

type recordingMessageFCM struct {
	sent []push.Payload
}

func (r *recordingMessageFCM) Send(_ context.Context, _ uuid.UUID, _ store.DeviceToken, payload fcm.PushPayload) error {
	r.sent = append(r.sent, push.Payload(payload))
	return nil
}

type messageTokenRepo struct {
	byProfile map[uuid.UUID][]store.DeviceToken
}

func (r messageTokenRepo) ListByProfile(_ context.Context, profileID uuid.UUID) ([]store.DeviceToken, error) {
	return r.byProfile[profileID], nil
}

func (messageTokenRepo) DeleteByToken(context.Context, string) error { return nil }

type parentAuthorResolver struct {
	pushenrich.NoopResolver
	author string
}

func (r parentAuthorResolver) MessageAuthorProfileID(context.Context, string) (string, error) {
	return r.author, nil
}

func stringPtr(value string) *string { return &value }

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

			require.False(t, decision.InApp, "archiving must suppress notification-center delivery")
			require.False(t, decision.Push, "archived %s recipient must not receive push", chatKind)
		})
	}
}

func TestRouteMessageNotification_ArchivedRecipientGetsNoPush(t *testing.T) {
	for _, chatKind := range []string{"dm", "group", "channel"} {
		t.Run(chatKind, func(t *testing.T) {
			senderID := uuid.New()
			recipientID := uuid.New()
			members := []chatmembers.Member{
				{ProfileID: senderID.String(), InboxBucket: "main"},
				{ProfileID: recipientID.String(), InboxBucket: "main", IsArchived: true},
			}
			handler := &consumer.MessageEventHandler{Router: delivery.DecideRouting}
			recorder := &recordingMessageFCM{}
			env := &eventsv1.MessageStreamEvent{Payload: &eventsv1.MessageStreamEvent_MessageSent{
				MessageSent: &eventsv1.MessageSent{
					MessageId:       uuid.NewString(),
					ChatId:          uuid.NewString(),
					SenderProfileId: senderID.String(),
				},
			}}
			err := routeMessageNotification(context.Background(), handler, stubChatMembers{rows: members}, &dispatch.MessagePusher{
				Tokens: messageTokenRepo{byProfile: map[uuid.UUID][]store.DeviceToken{
					recipientID: {{Token: "recipient-token", PushService: "fcm"}},
				}},
				Pusher:   &dispatch.PushDispatcher{FCM: recorder},
				Grouping: grouping.NewMemoryStore(),
			}, pushenrich.NoopResolver{}, env)

			require.NoError(t, err)
			require.Empty(t, recorder.sent, "archived %s recipient must not receive push", chatKind)
		})
	}
}

func TestRouteMessageNotification_ArchivedReplyAndMentionSuppressNotificationDelivery(t *testing.T) {
	for _, tc := range []struct {
		name  string
		route func(t *testing.T, handler *consumer.MessageEventHandler, members chatmembers.Lister, pusher *dispatch.MessagePusher, resolver pushenrich.Resolver, senderID, recipientID string) error
	}{
		{
			name: "thread reply",
			route: func(_ *testing.T, handler *consumer.MessageEventHandler, members chatmembers.Lister, pusher *dispatch.MessagePusher, resolver pushenrich.Resolver, senderID, recipientID string) error {
				return routeMessageNotification(context.Background(), handler, members, pusher, parentAuthorResolver{author: recipientID}, &eventsv1.MessageStreamEvent{Payload: &eventsv1.MessageStreamEvent_MessageSent{
					MessageSent: &eventsv1.MessageSent{MessageId: uuid.NewString(), ChatId: "group-chat", SenderProfileId: senderID, ThreadParentId: stringPtr("parent-message")},
				}})
			},
		},
		{
			name: "mention",
			route: func(_ *testing.T, handler *consumer.MessageEventHandler, members chatmembers.Lister, pusher *dispatch.MessagePusher, resolver pushenrich.Resolver, senderID, recipientID string) error {
				return routeMessageNotification(context.Background(), handler, members, pusher, resolver, &eventsv1.MessageStreamEvent{Payload: &eventsv1.MessageStreamEvent_MentionAdded{
					MentionAdded: &eventsv1.MentionAdded{MessageId: uuid.NewString(), ChatId: "channel-chat", SenderProfileId: senderID, MentionedProfileIds: []string{recipientID}},
				}})
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			senderID := uuid.NewString()
			recipientID := uuid.NewString()
			recorder := &recordingMessageFCM{}
			pusher := &dispatch.MessagePusher{
				Tokens: messageTokenRepo{byProfile: map[uuid.UUID][]store.DeviceToken{
					uuid.MustParse(recipientID): {{Token: "recipient-token", PushService: "fcm"}},
				}},
				Pusher:   &dispatch.PushDispatcher{FCM: recorder},
				Grouping: grouping.NewMemoryStore(),
			}
			members := stubChatMembers{rows: []chatmembers.Member{
				{ProfileID: senderID, InboxBucket: "main"},
				{ProfileID: recipientID, InboxBucket: "main", IsArchived: true},
			}}

			err := tc.route(t, &consumer.MessageEventHandler{Router: delivery.DecideRouting}, members, pusher, pushenrich.NoopResolver{}, senderID, recipientID)

			require.NoError(t, err)
			require.Empty(t, recorder.sent, "archived recipient must receive neither push nor notification-center delivery")
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
