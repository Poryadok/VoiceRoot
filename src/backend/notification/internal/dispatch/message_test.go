package dispatch_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/notification/internal/delivery"
	"voice/backend/notification/internal/dispatch"
	"voice/backend/notification/internal/fcm"
	"voice/backend/notification/internal/grouping"
	"voice/backend/notification/internal/push"
	"voice/backend/notification/internal/store"
)

type recordingFCM struct {
	sent []push.Payload
}

type suppressedLifecycleDelivery struct{}

func (suppressedLifecycleDelivery) WithChatDelivery(context.Context, string, func(context.Context) error) error {
	return nil
}

func TestMessagePusherFrozenChatSuppressesBeforeGroupingAndDispatch(t *testing.T) {
	profileID := uuid.New()
	rec := &recordingFCM{}
	pusher := &dispatch.MessagePusher{Tokens: &fakeTokenRepo{byProfile: map[uuid.UUID][]store.DeviceToken{profileID: {{Token: "token", PushService: "fcm"}}}}, Pusher: &dispatch.PushDispatcher{FCM: rec}, LifecycleDelivery: suppressedLifecycleDelivery{}}
	err := pusher.SendPush(context.Background(), map[string]delivery.DeliveryDecision{profileID.String(): {Push: true}}, delivery.DeliveryInput{ChatID: uuid.NewString(), Type: delivery.TypeNewMessage}, push.Payload{Body: "private text", Data: map[string]string{"type": "new_message"}}, "private text")
	require.NoError(t, err)
	require.Empty(t, rec.sent, "a frozen lifecycle must suppress delivery before entering provider dispatch")
}

func (r *recordingFCM) Send(_ context.Context, _ uuid.UUID, _ store.DeviceToken, payload fcm.PushPayload) error {
	r.sent = append(r.sent, push.Payload(payload))
	return nil
}

type fakeTokenRepo struct {
	byProfile map[uuid.UUID][]store.DeviceToken
}

type gamePushConsentSequence struct {
	answers []bool
	calls   int
	err     error
}

type gamePushChatScopeStub struct {
	scoped, allowed bool
	calls           int
	category        string
	err             error
}

func (s *gamePushChatScopeStub) ResolveGamePushChat(_ context.Context, _, _ uuid.UUID, category string) (bool, bool, error) {
	s.calls++
	s.category = category
	return s.scoped, s.allowed, s.err
}

type unblockedGamePush struct{}

func (unblockedGamePush) IsGamePushBlocked(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return false, nil
}

type blockedGamePush struct{ calls int }

func (b *blockedGamePush) IsGamePushBlocked(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	b.calls++
	return true, nil
}

func (g *gamePushConsentSequence) AllowsGamePush(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, string) (bool, error) {
	answer := g.answers[g.calls]
	g.calls++
	return answer, g.err
}

func (f *fakeTokenRepo) ListByProfile(_ context.Context, profileID uuid.UUID) ([]store.DeviceToken, error) {
	return f.byProfile[profileID], nil
}

func (f *fakeTokenRepo) DeleteByToken(context.Context, string) error { return nil }

func TestShouldDeliverPushToToken(t *testing.T) {
	require.True(t, dispatch.ShouldDeliverPushToToken("new_message", "fcm"))
	require.True(t, dispatch.ShouldDeliverPushToToken("new_message", "apns"))
	require.False(t, dispatch.ShouldDeliverPushToToken("new_message", "voip_apns"))
	require.True(t, dispatch.ShouldDeliverPushToToken("incoming_call", "voip_apns"))
	require.False(t, dispatch.ShouldDeliverPushToToken("incoming_call", "fcm"))
}

func TestMessagePusher_NoTokensNoSend(t *testing.T) {
	rec := &recordingFCM{}
	profileID := uuid.New()
	err := (&dispatch.MessagePusher{
		Tokens:   &fakeTokenRepo{byProfile: map[uuid.UUID][]store.DeviceToken{}},
		Pusher:   &dispatch.PushDispatcher{FCM: rec},
		Grouping: grouping.NewMemoryStore(),
	}).SendPush(context.Background(), map[string]delivery.DeliveryDecision{
		profileID.String(): {Push: true},
	}, delivery.DeliveryInput{
		ChatID: "chat-1",
		Type:   delivery.TypeNewMessage,
	}, push.Payload{
		Body: "Hello",
		Data: map[string]string{"type": "new_message"},
	}, "Hello")
	require.NoError(t, err)
	require.Empty(t, rec.sent)
}

func TestMessagePusherRechecksGameConsentForEachDeviceAndFailsClosed(t *testing.T) {
	profileID, appID, envID := uuid.New(), uuid.New(), uuid.New()
	rec := &recordingFCM{}
	consent := &gamePushConsentSequence{answers: []bool{true, false}}
	pusher := &dispatch.MessagePusher{
		Tokens: &fakeTokenRepo{byProfile: map[uuid.UUID][]store.DeviceToken{
			profileID: {{Token: "first", PushService: "fcm"}, {Token: "second", PushService: "fcm"}},
		}},
		Pusher: &dispatch.PushDispatcher{FCM: rec}, GameConsent: consent, GameBlocks: unblockedGamePush{},
	}
	in := delivery.DeliveryInput{Type: delivery.TypeNewMessage, GameApplicationID: appID, GameEnvironmentID: envID, GameCategory: "game_activity"}
	require.NoError(t, pusher.SendPush(context.Background(), map[string]delivery.DeliveryDecision{profileID.String(): {Push: true}}, in, push.Payload{Body: "Game event"}, "Game event"))
	require.Equal(t, 2, consent.calls, "current consent must be checked again before another device is sent")
	require.Len(t, rec.sent, 1, "a revoked consent suppresses remaining device deliveries")

	withoutRec := &recordingFCM{}
	withoutAuthority := &dispatch.MessagePusher{Tokens: pusher.Tokens, Pusher: &dispatch.PushDispatcher{FCM: withoutRec}}
	require.NoError(t, withoutAuthority.SendPush(context.Background(), map[string]delivery.DeliveryDecision{profileID.String(): {Push: true}}, in, push.Payload{}, ""))
	require.Empty(t, withoutRec.sent, "missing GIS authority must fail closed")

	deniedRec := &recordingFCM{}
	denied := &dispatch.MessagePusher{Tokens: pusher.Tokens, Pusher: &dispatch.PushDispatcher{FCM: deniedRec}, GameConsent: &gamePushConsentSequence{answers: []bool{false, false}}, GameBlocks: unblockedGamePush{}}
	require.NoError(t, denied.SendPush(context.Background(), map[string]delivery.DeliveryDecision{profileID.String(): {Push: true}}, in, push.Payload{}, ""))
	require.Empty(t, deniedRec.sent, "explicit opt-out must suppress the push")

	lookupErr := errors.New("GIS unavailable")
	errorRec := &recordingFCM{}
	errorPusher := &dispatch.MessagePusher{Tokens: pusher.Tokens, Pusher: &dispatch.PushDispatcher{FCM: errorRec}, GameConsent: &gamePushConsentSequence{answers: []bool{true}, err: lookupErr}, GameBlocks: unblockedGamePush{}}
	require.ErrorIs(t, errorPusher.SendPush(context.Background(), map[string]delivery.DeliveryDecision{profileID.String(): {Push: true}}, in, push.Payload{}, ""), lookupErr)
	require.Empty(t, errorRec.sent, "unavailable authority must retain the event for retry without sending")
}

func TestMessagePusherResolvesChatScopeBeforeOrdinaryChatPush(t *testing.T) {
	profileID, chatID := uuid.New(), uuid.New()
	tokens := &fakeTokenRepo{byProfile: map[uuid.UUID][]store.DeviceToken{
		profileID: {{Token: "first", PushService: "fcm"}, {Token: "second", PushService: "fcm"}},
	}}
	in := delivery.DeliveryInput{ChatID: chatID.String(), Type: delivery.TypeNewMessage, GameCategory: "game_activity"}
	decisions := map[string]delivery.DeliveryDecision{profileID.String(): {Push: true}}

	deniedSender := &recordingFCM{}
	deniedScope := &gamePushChatScopeStub{scoped: true, allowed: false}
	denied := &dispatch.MessagePusher{Tokens: tokens, Pusher: &dispatch.PushDispatcher{FCM: deniedSender}, GameChatScope: deniedScope}
	require.NoError(t, denied.SendPush(context.Background(), decisions, in, push.Payload{}, ""))
	require.Equal(t, 2, deniedScope.calls, "GIS consent is rechecked before each device send")
	require.Equal(t, "game_activity", deniedScope.category)
	require.Empty(t, deniedSender.sent)

	allowedSender := &recordingFCM{}
	ordinaryScope := &gamePushChatScopeStub{scoped: false}
	ordinary := &dispatch.MessagePusher{Tokens: tokens, Pusher: &dispatch.PushDispatcher{FCM: allowedSender}, GameChatScope: ordinaryScope}
	require.NoError(t, ordinary.SendPush(context.Background(), decisions, in, push.Payload{Title: "Message", Body: "private preview"}, "private preview"))
	require.Equal(t, 2, ordinaryScope.calls)
	require.Len(t, allowedSender.sent, 2, "unmapped Voice chats keep their ordinary push behavior")
	require.Equal(t, "private preview", allowedSender.sent[0].Body)

	gameSender := &recordingFCM{}
	gameScope := &gamePushChatScopeStub{scoped: true, allowed: true}
	gameGrouping := grouping.NewMemoryStore()
	game := &dispatch.MessagePusher{Tokens: tokens, Pusher: &dispatch.PushDispatcher{FCM: gameSender}, GameChatScope: gameScope, GameBlocks: unblockedGamePush{}, Grouping: gameGrouping}
	require.NoError(t, game.SendPush(context.Background(), decisions, in, push.Payload{Title: "Message", Body: "private preview"}, "private preview"))
	require.Len(t, gameSender.sent, 2)
	require.Equal(t, "Game update", gameSender.sent[0].Title)
	require.Equal(t, "A game event is waiting in Voice.", gameSender.sent[0].Body)
	state, err := gameGrouping.Get(context.Background(), delivery.GroupingKey(profileID, in.ChatID))
	require.NoError(t, err)
	require.NotNil(t, state)
	require.Empty(t, state.LastBody, "private preview must not be persisted for chats whose scope comes from GIS")

	failingSender := &recordingFCM{}
	lookupErr := errors.New("GIS unavailable")
	failing := &dispatch.MessagePusher{Tokens: tokens, Pusher: &dispatch.PushDispatcher{FCM: failingSender}, GameChatScope: &gamePushChatScopeStub{err: lookupErr}}
	require.ErrorIs(t, failing.SendPush(context.Background(), decisions, in, push.Payload{}, ""), lookupErr)
	require.Empty(t, failingSender.sent, "authority errors retry the event without a push")
}

func TestMessagePusher_SendsFCMWithGrouping(t *testing.T) {
	rec := &recordingFCM{}
	profileID := uuid.New()
	err := (&dispatch.MessagePusher{
		Tokens: &fakeTokenRepo{byProfile: map[uuid.UUID][]store.DeviceToken{
			profileID: {{Token: "tok-fcm", PushService: "fcm"}},
		}},
		Pusher:   &dispatch.PushDispatcher{FCM: rec},
		Grouping: grouping.NewMemoryStore(),
	}).SendPush(context.Background(), map[string]delivery.DeliveryDecision{
		profileID.String(): {Push: true},
	}, delivery.DeliveryInput{
		ChatID: "chat-1",
		Type:   delivery.TypeNewMessage,
	}, push.Payload{
		Body: "Hello",
		Data: map[string]string{"type": "new_message"},
	}, "Hello")
	require.NoError(t, err)
	require.Len(t, rec.sent, 1)
	require.Equal(t, 1, rec.sent[0].Counter)
	require.NotEmpty(t, rec.sent[0].CollapseTag)
}

func TestMessagePusher_MessageRequestGroupsBySenderAcrossChats(t *testing.T) {
	rec := &recordingFCM{}
	recipientID := uuid.New()
	senderID := uuid.New()
	pusher := &dispatch.MessagePusher{
		Tokens: &fakeTokenRepo{byProfile: map[uuid.UUID][]store.DeviceToken{
			recipientID: {{Token: "tok-fcm", PushService: "fcm"}},
		}},
		Pusher:   &dispatch.PushDispatcher{FCM: rec},
		Grouping: grouping.NewMemoryStore(),
	}
	decision := map[string]delivery.DeliveryDecision{
		recipientID.String(): {Push: true},
	}

	require.NoError(t, pusher.SendPush(context.Background(), decision, delivery.DeliveryInput{
		SenderProfileID: senderID,
		ChatID:          "chat-1",
		Type:            delivery.TypeMessageRequest,
	}, push.Payload{
		Body: "First request",
		Data: map[string]string{"type": "message_request"},
	}, "First request"))
	require.NoError(t, pusher.SendPush(context.Background(), decision, delivery.DeliveryInput{
		SenderProfileID: senderID,
		ChatID:          "chat-2",
		Type:            delivery.TypeMessageRequest,
	}, push.Payload{
		Body: "Second request",
		Data: map[string]string{"type": "message_request"},
	}, "Second request"))

	require.Len(t, rec.sent, 2)
	require.NotEmpty(t, rec.sent[0].CollapseTag)
	require.Equal(t, rec.sent[0].CollapseTag, rec.sent[1].CollapseTag)
	require.Equal(t, 1, rec.sent[0].Counter)
	require.Equal(t, 2, rec.sent[1].Counter)
}

func TestMessagePusher_MessageTypesShareOneChatGroupingSequence(t *testing.T) {
	rec := &recordingFCM{}
	recipientID := uuid.New()
	senderID := uuid.New()
	pusher := &dispatch.MessagePusher{
		Tokens: &fakeTokenRepo{byProfile: map[uuid.UUID][]store.DeviceToken{
			recipientID: {{Token: "tok-fcm", PushService: "fcm"}},
		}},
		Pusher:   &dispatch.PushDispatcher{FCM: rec},
		Grouping: grouping.NewMemoryStore(),
	}
	decision := map[string]delivery.DeliveryDecision{
		recipientID.String(): {Push: true},
	}
	chatID := uuid.NewString()
	types := []delivery.NotificationType{
		delivery.TypeNewMessage,
		delivery.TypeMention,
		delivery.TypeReply,
	}

	for i, typ := range types {
		body := string(typ)
		require.NoError(t, pusher.SendPush(context.Background(), decision, delivery.DeliveryInput{
			SenderProfileID: senderID,
			ChatID:          chatID,
			Type:            typ,
		}, push.Payload{
			Body: body,
			Data: map[string]string{"type": body},
		}, body))
		require.Len(t, rec.sent, i+1)
		require.Equal(t, i+1, rec.sent[i].Counter)
		if i > 0 {
			require.Equal(t, rec.sent[0].CollapseTag, rec.sent[i].CollapseTag)
		}
	}
}

func TestMessagePusher_SkipsVoIPToken(t *testing.T) {
	rec := &recordingFCM{}
	profileID := uuid.New()
	err := (&dispatch.MessagePusher{
		Tokens: &fakeTokenRepo{byProfile: map[uuid.UUID][]store.DeviceToken{
			profileID: {{Token: "voip-tok", PushService: "voip_apns"}},
		}},
		Pusher:   &dispatch.PushDispatcher{FCM: rec},
		Grouping: grouping.NewMemoryStore(),
	}).SendPush(context.Background(), map[string]delivery.DeliveryDecision{
		profileID.String(): {Push: true},
	}, delivery.DeliveryInput{
		Type: delivery.TypeNewMessage,
	}, push.Payload{
		Data: map[string]string{"type": "new_message"},
	}, "Hello")
	require.NoError(t, err)
	require.Empty(t, rec.sent)
}

func TestMatchmakingPusher_FallbackWhenNoTokens(t *testing.T) {
	rec := &recordingFCM{}
	pusher := &dispatch.MatchmakingPusher{
		Tokens: &fakeTokenRepo{byProfile: map[uuid.UUID][]store.DeviceToken{}},
		Pusher: &dispatch.PushDispatcher{FCM: rec},
	}
	err := pusher.SendPush(context.Background(), map[string]delivery.DeliveryDecision{
		uuid.NewString(): {Push: true},
	}, push.Payload{
		Data: map[string]string{"type": "match_found"},
	})
	require.NoError(t, err)
	require.Len(t, rec.sent, 1)
}

func TestMatchmakingPusher_SkipsVoIPToken(t *testing.T) {
	rec := &recordingFCM{}
	profileID := uuid.New()
	pusher := &dispatch.MatchmakingPusher{
		Tokens: &fakeTokenRepo{byProfile: map[uuid.UUID][]store.DeviceToken{
			profileID: {
				{Token: "fcm-tok", PushService: "fcm"},
				{Token: "voip-tok", PushService: "voip_apns"},
			},
		}},
		Pusher: &dispatch.PushDispatcher{FCM: rec},
	}
	err := pusher.SendPush(context.Background(), map[string]delivery.DeliveryDecision{
		profileID.String(): {Push: true},
	}, push.Payload{
		Data: map[string]string{"type": "match_found"},
	})
	require.NoError(t, err)
	require.Len(t, rec.sent, 1)
}

type explicitOfflinePresenceChecker struct{}

func (explicitOfflinePresenceChecker) IsOnline(context.Context, uuid.UUID) (bool, error) {
	return false, nil
}

func TestMessagePusher_EnrichDecision_OfflineGetsPush(t *testing.T) {
	profileID := uuid.New()
	decision, err := (&dispatch.MessagePusher{Presence: explicitOfflinePresenceChecker{}}).EnrichDecision(
		context.Background(),
		profileID.String(),
		uuid.New(),
		"chat-1",
		delivery.TypeNewMessage,
	)
	require.NoError(t, err)
	require.True(t, decision.Push)
}

type failingPresenceChecker struct{ err error }

func (p failingPresenceChecker) IsOnline(context.Context, uuid.UUID) (bool, error) {
	return false, p.err
}

func TestMessagePusher_EnrichDecision_FailsClosedWhenPresenceAuthorityIsUnavailable(t *testing.T) {
	profileID := uuid.New()
	wantErr := errors.New("presence authority unavailable")
	for name, pusher := range map[string]*dispatch.MessagePusher{
		"missing checker": {},
		"checker error":   {Presence: failingPresenceChecker{err: wantErr}},
	} {
		t.Run(name, func(t *testing.T) {
			decision, err := pusher.EnrichDecision(context.Background(), profileID.String(), uuid.New(), "chat-1", delivery.TypeNewMessage)
			require.Error(t, err)
			require.False(t, decision.Push, "an unavailable authority must not be treated as offline")
			if name == "checker error" {
				require.ErrorIs(t, err, wantErr)
			}
		})
	}
}

func TestMessagePusher_EnrichDecision_PresenceExceptionsDoNotCallPresence(t *testing.T) {
	for _, typ := range []delivery.NotificationType{delivery.TypeMatchFound, delivery.TypeVoiceMemberJoined} {
		t.Run(string(typ), func(t *testing.T) {
			profileID := uuid.New()
			presence := &countingPresenceChecker{}
			decision, err := (&dispatch.MessagePusher{Presence: presence}).EnrichDecision(
				context.Background(),
				profileID.String(),
				uuid.New(),
				"chat-1",
				typ,
			)
			require.NoError(t, err)
			require.Zero(t, presence.calls, "%s must skip the presence check", typ)
			require.True(t, decision.InApp)
			require.True(t, decision.Push)
		})
	}
}

type mutedPolicy struct{}

func (mutedPolicy) LoadPolicy(context.Context, uuid.UUID, string, delivery.NotificationType, time.Time) (delivery.SettingsSnapshot, delivery.QuietHoursSnapshot, error) {
	return delivery.SettingsSnapshot{ChatMuted: true}, delivery.QuietHoursSnapshot{}, nil
}

type quietHoursPolicy struct{}

func (quietHoursPolicy) LoadPolicy(context.Context, uuid.UUID, string, delivery.NotificationType, time.Time) (delivery.SettingsSnapshot, delivery.QuietHoursSnapshot, error) {
	return delivery.SettingsSnapshot{}, delivery.QuietHoursSnapshot{Enabled: true, StartTime: "00:00", EndTime: "23:59", Timezone: "UTC", At: time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)}, nil
}

func TestMessagePusherGameConsentCannotOverrideQuietHours(t *testing.T) {
	profileID := uuid.New()
	rec := &recordingFCM{}
	consent := &gamePushConsentSequence{answers: []bool{true}}
	pusher := &dispatch.MessagePusher{Tokens: &fakeTokenRepo{byProfile: map[uuid.UUID][]store.DeviceToken{profileID: {{Token: "tok", PushService: "fcm"}}}},
		Pusher: &dispatch.PushDispatcher{FCM: rec}, Policy: quietHoursPolicy{}, GameConsent: consent}
	err := pusher.SendPush(context.Background(), map[string]delivery.DeliveryDecision{profileID.String(): {Push: true}},
		delivery.DeliveryInput{Type: delivery.TypeNewMessage, GameApplicationID: uuid.New(), GameEnvironmentID: uuid.New(), GameCategory: "game_activity"}, push.Payload{}, "")
	require.NoError(t, err)
	require.Empty(t, rec.sent, "quiet hours suppress push before app consent is considered")
	require.Zero(t, consent.calls, "game consent does not override the user's DND policy")
}

func TestMessagePusherSuppressesGamePushWhenEitherAccountBlocks(t *testing.T) {
	profileID, senderID := uuid.New(), uuid.New()
	rec := &recordingFCM{}
	block := &blockedGamePush{}
	pusher := &dispatch.MessagePusher{Tokens: &fakeTokenRepo{byProfile: map[uuid.UUID][]store.DeviceToken{profileID: {{Token: "tok", PushService: "fcm"}}}},
		Pusher: &dispatch.PushDispatcher{FCM: rec}, GameConsent: &gamePushConsentSequence{answers: []bool{true}}, GameBlocks: block}
	err := pusher.SendPush(context.Background(), map[string]delivery.DeliveryDecision{profileID.String(): {Push: true}},
		delivery.DeliveryInput{SenderProfileID: senderID, Type: delivery.TypeNewMessage, GameApplicationID: uuid.New(), GameEnvironmentID: uuid.New(), GameCategory: "game_activity"}, push.Payload{}, "")
	require.NoError(t, err)
	require.Equal(t, 1, block.calls)
	require.Empty(t, rec.sent, "a directed account block suppresses a game push")
}

func TestMessagePusher_MutedChatSkipsPush(t *testing.T) {
	rec := &recordingFCM{}
	profileID := uuid.New()
	err := (&dispatch.MessagePusher{
		Tokens: &fakeTokenRepo{byProfile: map[uuid.UUID][]store.DeviceToken{
			profileID: {{Token: "tok", PushService: "fcm"}},
		}},
		Pusher:   &dispatch.PushDispatcher{FCM: rec},
		Grouping: grouping.NewMemoryStore(),
		Policy:   mutedPolicy{},
	}).SendPush(context.Background(), map[string]delivery.DeliveryDecision{
		profileID.String(): {Push: true},
	}, delivery.DeliveryInput{
		ChatID: "chat-1",
		Type:   delivery.TypeNewMessage,
	}, push.Payload{
		Data: map[string]string{"type": "new_message"},
	}, "Hello")
	require.NoError(t, err)
	require.Empty(t, rec.sent)
}

func TestMessagePusher_SendSilentPreservesGroupingButMarksPushSilent(t *testing.T) {
	rec := &recordingFCM{}
	profileID := uuid.New()
	err := (&dispatch.MessagePusher{
		Tokens: &fakeTokenRepo{byProfile: map[uuid.UUID][]store.DeviceToken{
			profileID: {{Token: "tok-fcm", PushService: "fcm"}},
		}},
		Pusher:   &dispatch.PushDispatcher{FCM: rec},
		Grouping: grouping.NewMemoryStore(),
	}).SendPush(context.Background(), map[string]delivery.DeliveryDecision{
		profileID.String(): {Push: true, InApp: true},
	}, delivery.DeliveryInput{ChatID: "chat-1", Type: delivery.TypeNewMessage}, push.Payload{
		Body:   "Hello",
		Data:   map[string]string{"type": "new_message"},
		Silent: true,
	}, "Hello")

	require.NoError(t, err)
	require.Len(t, rec.sent, 1)
	require.True(t, rec.sent[0].Silent)
	require.NotEmpty(t, rec.sent[0].CollapseTag)
}
