package main

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"testing"
	"time"
	eventsv1 "voice.app/voice/events/v1"
	"voice/backend/notification/internal/chatmembers"
	"voice/backend/notification/internal/delivery"
	"voice/backend/notification/internal/dispatch"
	"voice/backend/notification/internal/presence"
	"voice/backend/pkg/notificationrouting"
)

type routingMembers []chatmembers.Member

func (m routingMembers) ListMembers(context.Context, string) ([]chatmembers.Member, error) {
	return m, nil
}
func (m routingMembers) ListMemberProfileIDs(context.Context, string) ([]string, error) {
	return nil, nil
}

type routingAuthority func(context.Context, string, string, string) (bool, error)

func (f routingAuthority) Authorize(c context.Context, r, s, h string) (bool, error) {
	return f(c, r, s, h)
}

type routingPolicy map[string]delivery.SettingsSnapshot

func (p routingPolicy) LoadPolicy(_ context.Context, id uuid.UUID, _ string, _ delivery.NotificationType, _ time.Time) (delivery.SettingsSnapshot, delivery.QuietHoursSnapshot, error) {
	return p[id.String()], delivery.QuietHoursSnapshot{}, nil
}

func TestMessageRoutingDecisionOnlyAuthorizesUnsuppressedCurrentRecipients(t *testing.T) {
	sender, allowed, muted, suppressed, denied, archived, declined := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	chat := uuid.NewString()
	data, err := proto.Marshal(&eventsv1.MessageStreamEvent{EventId: uuid.NewString(), Payload: &eventsv1.MessageStreamEvent_MessageSent{MessageSent: &eventsv1.MessageSent{ChatId: chat, MessageId: uuid.NewString(), SenderProfileId: sender, SendSilent: true}}})
	require.NoError(t, err)
	members := routingMembers{{ProfileID: sender}, {ProfileID: allowed}, {ProfileID: muted}, {ProfileID: suppressed}, {ProfileID: denied}, {ProfileID: archived, IsArchived: true}, {ProfileID: declined, InboxBucket: "declined"}}
	pusher := &dispatch.MessagePusher{Presence: presence.OfflineChecker{}, Policy: routingPolicy{muted: {ChatMuted: true}, suppressed: {SuppressTypes: []delivery.NotificationType{delivery.TypeNewMessage}}}}
	authority := routingAuthority(func(_ context.Context, r, s, h string) (bool, error) {
		require.Equal(t, sender, s)
		require.Equal(t, chat, h)
		return r != denied, nil
	})
	got, err := messageRoutingDecision(context.Background(), data, members, pusher, nil, authority)
	require.NoError(t, err)
	require.Equal(t, []notificationrouting.Target{{ProfileID: allowed, Type: "new_message"}}, got)
}

func TestMessageRoutingDecisionAuthorityFailureReturnsNoPartialAllowSet(t *testing.T) {
	sender, recipient, healthy := uuid.NewString(), uuid.NewString(), uuid.NewString()
	data, err := proto.Marshal(&eventsv1.MessageStreamEvent{EventId: uuid.NewString(), Payload: &eventsv1.MessageStreamEvent_MessageSent{MessageSent: &eventsv1.MessageSent{ChatId: uuid.NewString(), MessageId: uuid.NewString(), SenderProfileId: sender}}})
	require.NoError(t, err)
	got, err := messageRoutingDecision(context.Background(), data, routingMembers{{ProfileID: sender}, {ProfileID: recipient}, {ProfileID: healthy}}, &dispatch.MessagePusher{Presence: presence.OfflineChecker{}}, nil, routingAuthority(func(_ context.Context, r, _, _ string) (bool, error) {
		if r == recipient {
			return false, errors.New("authority unavailable")
		}
		return true, nil
	}))
	require.Error(t, err)
	require.Empty(t, got)
}

func TestMessageRoutingDecisionKeepsSpecialRecipientsAndTypePolicy(t *testing.T) {
	sender, target, other, chat, message := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	for _, tc := range []struct {
		name, typ  string
		suppressed bool
	}{
		{"request", "message_request", false}, {"mention", "mention", false}, {"reaction", "reaction", false},
		{"suppressed mention", "mention", true}, {"suppressed reaction", "reaction", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			event := &eventsv1.MessageStreamEvent{EventId: uuid.NewString()}
			members := routingMembers{{ProfileID: sender}, {ProfileID: target}, {ProfileID: other}}
			switch tc.typ {
			case "message_request":
				event.Payload = &eventsv1.MessageStreamEvent_MessageSent{MessageSent: &eventsv1.MessageSent{ChatId: chat, MessageId: message, SenderProfileId: sender}}
				members[1].InboxBucket = "requests"
				members[2].IsArchived = true
			case "mention":
				event.Payload = &eventsv1.MessageStreamEvent_MentionAdded{MentionAdded: &eventsv1.MentionAdded{ChatId: chat, MessageId: message, SenderProfileId: sender, MentionedProfileIds: []string{sender, target, uuid.NewString()}}}
			case "reaction":
				event.Payload = &eventsv1.MessageStreamEvent_ReactionAdded{ReactionAdded: &eventsv1.ReactionAdded{ChatId: chat, MessageId: message, ProfileId: sender, MessageAuthorProfileId: target, Emoji: "👍"}}
			}
			policy := routingPolicy{}
			if tc.suppressed {
				policy[target] = delivery.SettingsSnapshot{SuppressTypes: []delivery.NotificationType{delivery.NotificationType(tc.typ)}}
			}
			data, e := proto.Marshal(event)
			require.NoError(t, e)
			got, e := messageRoutingDecision(context.Background(), data, members, &dispatch.MessagePusher{Presence: presence.OfflineChecker{}, Policy: policy}, nil, routingAuthority(func(context.Context, string, string, string) (bool, error) { return true, nil }))
			require.NoError(t, e)
			if tc.suppressed {
				require.Empty(t, got)
			} else {
				require.Equal(t, []notificationrouting.Target{{ProfileID: target, Type: tc.typ}}, got)
			}
		})
	}
}
