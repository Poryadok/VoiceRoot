package main

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	natsserver "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

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
	sent   []push.Payload
	sentTo []uuid.UUID
}

func (r *recordingMessageFCM) Send(_ context.Context, recipient uuid.UUID, _ store.DeviceToken, payload fcm.PushPayload) error {
	r.sent = append(r.sent, push.Payload(payload))
	r.sentTo = append(r.sentTo, recipient)
	return nil
}

type failOnceMessagePolicy struct {
	calls atomic.Int32
	err   error
}

type failOnMessagePolicyCall struct {
	calls  atomic.Int32
	failAt int32
	err    error
}

type failOnceMessagePresence struct {
	calls atomic.Int32
	err   error
}

func (p *failOnceMessagePresence) IsOnline(context.Context, uuid.UUID) (bool, error) {
	if p.calls.Add(1) == 1 {
		return false, p.err
	}
	return false, nil
}

func (p *failOnceMessagePolicy) LoadPolicy(ctx context.Context, profileID uuid.UUID, chatID string, typ delivery.NotificationType, at time.Time) (delivery.SettingsSnapshot, delivery.QuietHoursSnapshot, error) {
	if p.calls.Add(1) == 1 {
		return delivery.SettingsSnapshot{}, delivery.QuietHoursSnapshot{}, p.err
	}
	return delivery.PermissivePolicyLoader{}.LoadPolicy(ctx, profileID, chatID, typ, at)
}

func (p *failOnMessagePolicyCall) LoadPolicy(ctx context.Context, profileID uuid.UUID, chatID string, typ delivery.NotificationType, at time.Time) (delivery.SettingsSnapshot, delivery.QuietHoursSnapshot, error) {
	if p.calls.Add(1) == p.failAt {
		return delivery.SettingsSnapshot{}, delivery.QuietHoursSnapshot{}, p.err
	}
	return delivery.PermissivePolicyLoader{}.LoadPolicy(ctx, profileID, chatID, typ, at)
}

type channelMessageFCM struct {
	sent chan push.Payload
}

func (r channelMessageFCM) Send(_ context.Context, _ uuid.UUID, _ store.DeviceToken, payload fcm.PushPayload) error {
	r.sent <- push.Payload(payload)
	return nil
}

type messageTokenRepo struct {
	byProfile map[uuid.UUID][]store.DeviceToken
}

type recordingGameConsent struct {
	profileID, appID, envID uuid.UUID
	category                string
	allowed                 bool
	calls                   int
}

type recordingGameChatScope struct {
	profileID, chatID uuid.UUID
	category          string
	gameScoped        bool
	allowed           bool
	calls             int
}

func (r *recordingGameChatScope) ResolveGamePushChat(_ context.Context, profileID, chatID uuid.UUID, category string) (bool, bool, error) {
	r.profileID, r.chatID, r.category = profileID, chatID, category
	r.calls++
	return r.gameScoped, r.allowed, nil
}

type retryThenOptedOutConsent struct {
	calls         atomic.Int32
	redeliveryHit chan struct{}
	release       chan struct{}
}

func (c *retryThenOptedOutConsent) AllowsGamePush(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, string) (bool, error) {
	switch c.calls.Add(1) {
	case 1:
		return false, errors.New("temporary GIS consent lookup failure")
	case 2:
		c.redeliveryHit <- struct{}{}
		<-c.release
		return false, nil
	default:
		return false, nil
	}
}

type noGameBlock struct{}

func (noGameBlock) IsGamePushBlocked(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return false, nil
}

func (r *recordingGameConsent) AllowsGamePush(_ context.Context, profileID, appID, envID uuid.UUID, category string) (bool, error) {
	r.profileID, r.appID, r.envID, r.category = profileID, appID, envID, category
	r.calls++
	return r.allowed, nil
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

func TestRouteMessageNotificationPolicyFailureRetriesAllRecipientsBeforeSending(t *testing.T) {
	senderID, firstRecipientID, secondRecipientID := uuid.New(), uuid.New(), uuid.New()
	recorder := &recordingMessageFCM{}
	policyErr := errors.New("recipient policy unavailable")
	policy := &failOnMessagePolicyCall{failAt: 2, err: policyErr}
	pusher := &dispatch.MessagePusher{
		Tokens: messageTokenRepo{byProfile: map[uuid.UUID][]store.DeviceToken{
			firstRecipientID:  {{Token: "first-token", PushService: "fcm"}},
			secondRecipientID: {{Token: "second-token", PushService: "fcm"}},
		}},
		Pusher: &dispatch.PushDispatcher{FCM: recorder}, Grouping: grouping.NewMemoryStore(), Policy: policy,
	}
	handler := &consumer.MessageEventHandler{Router: delivery.DecideRouting}
	members := stubChatMembers{rows: []chatmembers.Member{
		{ProfileID: senderID.String(), InboxBucket: "main"},
		{ProfileID: firstRecipientID.String(), InboxBucket: "main"},
		{ProfileID: secondRecipientID.String(), InboxBucket: "main"},
	}}
	event := &eventsv1.MessageStreamEvent{Payload: &eventsv1.MessageStreamEvent_MessageSent{
		MessageSent: &eventsv1.MessageSent{MessageId: uuid.NewString(), ChatId: uuid.NewString(), SenderProfileId: senderID.String()},
	}}

	err := routeMessageNotification(context.Background(), handler, members, pusher, pushenrich.NoopResolver{}, event)
	require.ErrorIs(t, err, policyErr, "a transient recipient policy error must fail the durable attempt")
	require.EqualValues(t, 2, policy.calls.Load(), "the second recipient's policy must fail after the first recipient was prepared")
	require.Empty(t, recorder.sentTo, "resolve every recipient before dispatch so retry cannot duplicate an earlier recipient")

	err = routeMessageNotification(context.Background(), handler, members, pusher, pushenrich.NoopResolver{}, event)
	require.NoError(t, err)
	require.EqualValues(t, 4, policy.calls.Load(), "retry should preflight both recipients")
	require.ElementsMatch(t, []uuid.UUID{firstRecipientID, secondRecipientID}, recorder.sentTo)
}

func TestRouteMessageNotificationDoesNotReloadPoliciesDuringDispatch(t *testing.T) {
	senderID, firstRecipientID, secondRecipientID := uuid.New(), uuid.New(), uuid.New()
	recorder := &recordingMessageFCM{}
	policyErr := errors.New("unexpected second policy read")
	policy := &failOnMessagePolicyCall{failAt: 3, err: policyErr}
	pusher := &dispatch.MessagePusher{
		Tokens: messageTokenRepo{byProfile: map[uuid.UUID][]store.DeviceToken{
			firstRecipientID:  {{Token: "first-token", PushService: "fcm"}},
			secondRecipientID: {{Token: "second-token", PushService: "fcm"}},
		}},
		Pusher: &dispatch.PushDispatcher{FCM: recorder}, Grouping: grouping.NewMemoryStore(), Policy: policy,
	}
	handler := &consumer.MessageEventHandler{Router: delivery.DecideRouting}
	members := stubChatMembers{rows: []chatmembers.Member{
		{ProfileID: senderID.String(), InboxBucket: "main"},
		{ProfileID: firstRecipientID.String(), InboxBucket: "main"},
		{ProfileID: secondRecipientID.String(), InboxBucket: "main"},
	}}
	event := &eventsv1.MessageStreamEvent{Payload: &eventsv1.MessageStreamEvent_MessageSent{
		MessageSent: &eventsv1.MessageSent{MessageId: uuid.NewString(), ChatId: uuid.NewString(), SenderProfileId: senderID.String()},
	}}

	err := routeMessageNotification(context.Background(), handler, members, pusher, pushenrich.NoopResolver{}, event)
	require.NoError(t, err, "the event should use the policies already preflighted for all recipients")
	require.EqualValues(t, 2, policy.calls.Load(), "dispatch must not reload a policy after sending starts")
	require.ElementsMatch(t, []uuid.UUID{firstRecipientID, secondRecipientID}, recorder.sentTo)
}

func TestRouteMessageNotificationPresenceOutageDoesNotBecomeOfflinePushAndRetries(t *testing.T) {
	senderID, firstRecipientID, secondRecipientID := uuid.New(), uuid.New(), uuid.New()
	recorder := &recordingMessageFCM{}
	presenceErr := errors.New("presence authority unavailable")
	presence := &failOnceMessagePresence{err: presenceErr}
	pusher := &dispatch.MessagePusher{
		Tokens: messageTokenRepo{byProfile: map[uuid.UUID][]store.DeviceToken{
			firstRecipientID:  {{Token: "first-token", PushService: "fcm"}},
			secondRecipientID: {{Token: "second-token", PushService: "fcm"}},
		}},
		Pusher: &dispatch.PushDispatcher{FCM: recorder}, Presence: presence, Grouping: grouping.NewMemoryStore(),
	}
	handler := &consumer.MessageEventHandler{Router: delivery.DecideRouting}
	members := stubChatMembers{rows: []chatmembers.Member{
		{ProfileID: senderID.String(), InboxBucket: "main"},
		{ProfileID: firstRecipientID.String(), InboxBucket: "main"},
		{ProfileID: secondRecipientID.String(), InboxBucket: "main"},
	}}
	event := &eventsv1.MessageStreamEvent{Payload: &eventsv1.MessageStreamEvent_MessageSent{
		MessageSent: &eventsv1.MessageSent{MessageId: uuid.NewString(), ChatId: uuid.NewString(), SenderProfileId: senderID.String()},
	}}

	err := routeMessageNotification(context.Background(), handler, members, pusher, pushenrich.NoopResolver{}, event)
	require.ErrorIs(t, err, presenceErr, "authority failure must NAK rather than route the recipient as offline")
	require.Empty(t, recorder.sentTo)

	err = routeMessageNotification(context.Background(), handler, members, pusher, pushenrich.NoopResolver{}, event)
	require.NoError(t, err)
	require.ElementsMatch(t, []uuid.UUID{firstRecipientID, secondRecipientID}, recorder.sentTo)
}

func TestRouteMessageNotificationCarriesGameScopeAndUsesPrivatePushCopy(t *testing.T) {
	senderID, recipientID, appID, envID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	recorder := &recordingMessageFCM{}
	consent := &recordingGameConsent{allowed: true}
	pusher := &dispatch.MessagePusher{
		Tokens: messageTokenRepo{byProfile: map[uuid.UUID][]store.DeviceToken{recipientID: {{Token: "recipient-token", PushService: "fcm"}}}},
		Pusher: &dispatch.PushDispatcher{FCM: recorder}, Grouping: grouping.NewMemoryStore(), GameConsent: consent, GameBlocks: noGameBlock{},
	}
	err := routeMessageNotification(context.Background(), &consumer.MessageEventHandler{Router: delivery.DecideRouting},
		stubChatMembers{ids: []string{senderID.String(), recipientID.String()}}, pusher, pushenrich.NoopResolver{},
		&eventsv1.MessageStreamEvent{Payload: &eventsv1.MessageStreamEvent_MessageSent{MessageSent: &eventsv1.MessageSent{
			MessageId: uuid.NewString(), ChatId: uuid.NewString(), SenderProfileId: senderID.String(),
			GameApplicationId: stringPtr(appID.String()), GameEnvironmentId: stringPtr(envID.String()),
		}}})
	require.NoError(t, err)
	require.Equal(t, 1, consent.calls)
	require.Equal(t, recipientID, consent.profileID)
	require.Equal(t, appID, consent.appID)
	require.Equal(t, envID, consent.envID)
	require.Equal(t, "game_activity", consent.category)
	require.Len(t, recorder.sent, 1)
	require.Equal(t, "Game update", recorder.sent[0].Title)
	require.Equal(t, "A game event is waiting in Voice.", recorder.sent[0].Body)
	for _, key := range []string{"action_id", "command", "state_version", "execution_permit"} {
		require.NotContains(t, recorder.sent[0].Data, key, "game pushes are navigation-only and must not expose executable actions")
	}
}

func TestRouteMessageNotificationResolvesChatScopeForPush(t *testing.T) {
	for _, tc := range []struct {
		name, category string
		payload        *eventsv1.MessageStreamEvent
	}{
		{
			name:     "message",
			category: "game_activity",
			payload: &eventsv1.MessageStreamEvent{Payload: &eventsv1.MessageStreamEvent_MessageSent{MessageSent: &eventsv1.MessageSent{
				MessageId: uuid.NewString(), ChatId: uuid.NewString(), SenderProfileId: uuid.NewString(),
			}}},
		},
		{
			name:     "mention",
			category: "game_social",
			payload: &eventsv1.MessageStreamEvent{Payload: &eventsv1.MessageStreamEvent_MentionAdded{MentionAdded: &eventsv1.MentionAdded{
				MessageId: uuid.NewString(), ChatId: uuid.NewString(), SenderProfileId: uuid.NewString(), MentionedProfileIds: []string{uuid.NewString()},
			}}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			message := tc.payload.GetMessageSent()
			var chatID, senderID, recipientID string
			if message != nil {
				chatID, senderID, recipientID = message.GetChatId(), message.GetSenderProfileId(), uuid.NewString()
				members := stubChatMembers{ids: []string{senderID, recipientID}}
				assertChatScopePush(t, tc.category, chatID, senderID, recipientID, members, tc.payload)
				return
			}
			mention := tc.payload.GetMentionAdded()
			chatID, senderID, recipientID = mention.GetChatId(), mention.GetSenderProfileId(), mention.GetMentionedProfileIds()[0]
			members := stubChatMembers{ids: []string{senderID, recipientID}}
			assertChatScopePush(t, tc.category, chatID, senderID, recipientID, members, tc.payload)
		})
	}
}

func assertChatScopePush(t *testing.T, category, chatID, senderID, recipientID string, members chatmembers.Lister, event *eventsv1.MessageStreamEvent) {
	t.Helper()
	recipientUUID, chatUUID := uuid.MustParse(recipientID), uuid.MustParse(chatID)
	recorder := &recordingMessageFCM{}
	scope := &recordingGameChatScope{gameScoped: true, allowed: false}
	pusher := &dispatch.MessagePusher{
		Tokens: messageTokenRepo{byProfile: map[uuid.UUID][]store.DeviceToken{recipientUUID: {{Token: "recipient-token", PushService: "fcm"}}}},
		Pusher: &dispatch.PushDispatcher{FCM: recorder}, GameChatScope: scope, GameBlocks: noGameBlock{}, Grouping: grouping.NewMemoryStore(),
	}
	err := routeMessageNotification(context.Background(), &consumer.MessageEventHandler{Router: delivery.DecideRouting}, members, pusher, pushenrich.NoopResolver{}, event)
	require.NoError(t, err)
	require.Equal(t, 1, scope.calls)
	require.Equal(t, recipientUUID, scope.profileID)
	require.Equal(t, chatUUID, scope.chatID)
	require.Equal(t, category, scope.category)
	require.Empty(t, recorder.sent, "game chat opt-out suppresses push")
}

func TestRouteQueuedGameMessageSuppressesPushAfterOptOutForEveryDevice(t *testing.T) {
	senderID, recipientID, appID, envID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	recorder := &recordingMessageFCM{}
	consent := &recordingGameConsent{allowed: true}
	pusher := &dispatch.MessagePusher{
		Tokens: messageTokenRepo{byProfile: map[uuid.UUID][]store.DeviceToken{
			recipientID: {
				{Token: "device-one", PushService: "fcm"},
				{Token: "device-two", PushService: "fcm"},
			},
		}},
		Pusher: &dispatch.PushDispatcher{FCM: recorder}, Grouping: grouping.NewMemoryStore(),
		GameConsent: consent, GameBlocks: noGameBlock{},
	}
	queuedEvent := &eventsv1.MessageStreamEvent{Payload: &eventsv1.MessageStreamEvent_MessageSent{MessageSent: &eventsv1.MessageSent{
		MessageId: uuid.NewString(), ChatId: uuid.NewString(), SenderProfileId: senderID.String(),
		GameApplicationId: stringPtr(appID.String()), GameEnvironmentId: stringPtr(envID.String()),
	}}}
	// Consent is revoked after the event exists but before the durable consumer
	// routes it. Delivery must read current consent instead of trusting enqueue time.
	consent.allowed = false
	err := routeMessageNotification(context.Background(), &consumer.MessageEventHandler{Router: delivery.DecideRouting},
		stubChatMembers{ids: []string{senderID.String(), recipientID.String()}}, pusher, pushenrich.NoopResolver{}, queuedEvent)
	require.NoError(t, err)
	require.Equal(t, 2, consent.calls, "current app consent must be checked separately before each device delivery")
	require.Empty(t, recorder.sent, "a queued event cannot bypass a later opt-out on any device")
}

func TestMessageEventsJetStreamRedeliveryRechecksQueuedGameConsentAndSuppressesEveryDevice(t *testing.T) {
	options := &natsserver.Options{JetStream: true, StoreDir: t.TempDir(), Port: -1}
	server, err := natsserver.NewServer(options)
	require.NoError(t, err)
	go server.Start()
	require.True(t, server.ReadyForConnections(10*time.Second))
	defer server.Shutdown()

	provisioner, err := nats.Connect(server.ClientURL())
	require.NoError(t, err)
	defer provisioner.Close()
	js, err := provisioner.JetStream()
	require.NoError(t, err)
	_, err = js.AddStream(&nats.StreamConfig{Name: jsStreamMessageEvents, Subjects: []string{jsSubjectMessageEvents}})
	require.NoError(t, err)
	_, err = js.AddConsumer(jsStreamMessageEvents, &nats.ConsumerConfig{
		Durable: consumer.SharedDurable("message"), FilterSubject: jsSubjectMessageEvents,
		DeliverSubject: "_INBOX.voice.notification.message", AckPolicy: nats.AckExplicitPolicy,
		DeliverPolicy: nats.DeliverNewPolicy,
	})
	require.NoError(t, err)

	senderID, recipientID, appID, envID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	recorder := &recordingMessageFCM{}
	consent := &retryThenOptedOutConsent{redeliveryHit: make(chan struct{}, 1), release: make(chan struct{})}
	pusher := &dispatch.MessagePusher{
		Tokens: messageTokenRepo{byProfile: map[uuid.UUID][]store.DeviceToken{recipientID: {
			{Token: "device-one", PushService: "fcm"}, {Token: "device-two", PushService: "fcm"},
		}}},
		Pusher: &dispatch.PushDispatcher{FCM: recorder}, Grouping: grouping.NewMemoryStore(),
		GameConsent: consent, GameBlocks: noGameBlock{},
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	readiness := newNotificationConsumerReadiness("message")
	done := make(chan error, 1)
	go func() {
		done <- runMessageEventsConsumer(withNotificationConsumerReadiness(ctx, readiness, "message"), server.ClientURL(), &store.DeviceTokenStore{},
			stubChatMembers{ids: []string{senderID.String(), recipientID.String()}}, pusher, pushenrich.NoopResolver{}, nil)
	}()
	require.Eventually(t, readiness.ready, 5*time.Second, 10*time.Millisecond, "message durable binds before publication")

	event := &eventsv1.MessageStreamEvent{EventId: uuid.NewString(), Payload: &eventsv1.MessageStreamEvent_MessageSent{
		MessageSent: &eventsv1.MessageSent{MessageId: uuid.NewString(), ChatId: uuid.NewString(), SenderProfileId: senderID.String(),
			GameApplicationId: stringPtr(appID.String()), GameEnvironmentId: stringPtr(envID.String())},
	}}
	encoded, err := proto.Marshal(event)
	require.NoError(t, err)
	_, err = js.Publish(jsSubjectMessageEvents[:len(jsSubjectMessageEvents)-1]+"sent", encoded)
	require.NoError(t, err)
	select {
	case <-consent.redeliveryHit:
		// The first GIS lookup failed and the durable consumer is processing its retry.
	case <-time.After(5 * time.Second):
		t.Fatal("queued game message was not redelivered after consent authority failure")
	}
	close(consent.release)
	require.Eventually(t, func() bool {
		info, infoErr := js.ConsumerInfo(jsStreamMessageEvents, consumer.SharedDurable("message"))
		return infoErr == nil && info.NumPending == 0 && info.NumAckPending == 0 && consent.calls.Load() == 3
	}, 5*time.Second, 20*time.Millisecond, "successful opt-out recheck acknowledges the queued event")
	require.Empty(t, recorder.sent, "the queued event cannot bypass consent revoked before its retry")
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("message consumer did not stop after cancellation")
	}
}

func TestMessageEventsJetStreamRestartDrainsBacklogFromSameDurable(t *testing.T) {
	options := &natsserver.Options{JetStream: true, StoreDir: t.TempDir(), Port: -1}
	server, err := natsserver.NewServer(options)
	require.NoError(t, err)
	go server.Start()
	t.Cleanup(func() { server.Shutdown() })
	require.True(t, server.ReadyForConnections(10*time.Second))

	provisioner, err := nats.Connect(server.ClientURL())
	require.NoError(t, err)
	defer provisioner.Close()
	js, err := provisioner.JetStream()
	require.NoError(t, err)
	_, err = js.AddStream(&nats.StreamConfig{Name: jsStreamMessageEvents, Subjects: []string{jsSubjectMessageEvents}})
	require.NoError(t, err)
	durable := consumer.SharedDurable("message")
	_, err = js.AddConsumer(jsStreamMessageEvents, &nats.ConsumerConfig{
		Durable: durable, FilterSubject: jsSubjectMessageEvents,
		DeliverSubject: "_INBOX.voice.notification.message", AckPolicy: nats.AckExplicitPolicy,
		DeliverPolicy: nats.DeliverNewPolicy,
	})
	require.NoError(t, err)

	senderID, recipientID := uuid.New(), uuid.New()
	deliveries := make(chan push.Payload, 2)
	pusher := &dispatch.MessagePusher{
		Tokens: messageTokenRepo{byProfile: map[uuid.UUID][]store.DeviceToken{recipientID: {{Token: "recipient-token", PushService: "fcm"}}}},
		Pusher: &dispatch.PushDispatcher{FCM: channelMessageFCM{sent: deliveries}}, Grouping: grouping.NewMemoryStore(),
	}
	members := stubChatMembers{ids: []string{senderID.String(), recipientID.String()}}

	firstCtx, stopFirst := context.WithCancel(context.Background())
	firstReadiness := newNotificationConsumerReadiness("message")
	firstDone := make(chan struct{})
	go func() {
		defer close(firstDone)
		_ = runMessageEventsConsumer(withNotificationConsumerReadiness(firstCtx, firstReadiness, "message"), server.ClientURL(), &store.DeviceTokenStore{},
			members, pusher, pushenrich.NoopResolver{}, nil)
	}()
	t.Cleanup(func() {
		stopFirst()
		select {
		case <-firstDone:
		case <-time.After(5 * time.Second):
			t.Errorf("first message consumer did not stop during cleanup")
		}
	})
	require.Eventually(t, firstReadiness.ready, 5*time.Second, 10*time.Millisecond, "first message consumer binds the pre-provisioned durable")
	stopFirst()
	select {
	case <-firstDone:
	case <-time.After(5 * time.Second):
		t.Fatal("first message consumer did not stop after cancellation")
	}
	require.Eventually(t, func() bool {
		info, infoErr := js.ConsumerInfo(jsStreamMessageEvents, durable)
		return infoErr == nil && !info.PushBound
	}, 5*time.Second, 20*time.Millisecond, "message durable is unbound before publishing the offline event")

	messageID, chatID := uuid.NewString(), uuid.NewString()
	event := &eventsv1.MessageStreamEvent{EventId: uuid.NewString(), Payload: &eventsv1.MessageStreamEvent_MessageSent{
		MessageSent: &eventsv1.MessageSent{MessageId: messageID, ChatId: chatID, SenderProfileId: senderID.String()},
	}}
	encoded, err := proto.Marshal(event)
	require.NoError(t, err)
	_, err = js.Publish(jsSubjectMessageEvents[:len(jsSubjectMessageEvents)-1]+"sent", encoded)
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		info, infoErr := js.ConsumerInfo(jsStreamMessageEvents, durable)
		return infoErr == nil && info.NumPending == 1 && info.NumAckPending == 0
	}, 5*time.Second, 20*time.Millisecond, "published event remains pending on the durable while the consumer is stopped")

	secondCtx, stopSecond := context.WithCancel(context.Background())
	secondReadiness := newNotificationConsumerReadiness("message")
	secondDone := make(chan struct{})
	go func() {
		defer close(secondDone)
		_ = runMessageEventsConsumer(withNotificationConsumerReadiness(secondCtx, secondReadiness, "message"), server.ClientURL(), &store.DeviceTokenStore{},
			members, pusher, pushenrich.NoopResolver{}, nil)
	}()
	t.Cleanup(func() {
		stopSecond()
		select {
		case <-secondDone:
		case <-time.After(5 * time.Second):
			t.Errorf("restarted message consumer did not stop during cleanup")
		}
	})
	require.Eventually(t, secondReadiness.ready, 5*time.Second, 10*time.Millisecond, "restarted message consumer rebinds the same durable")
	select {
	case payload := <-deliveries:
		require.Equal(t, "New message", payload.Title)
		require.Equal(t, chatID, payload.Data["chat_id"])
		require.Equal(t, messageID, payload.Data["message_id"])
	case <-time.After(5 * time.Second):
		t.Fatal("restarted message consumer did not deliver the pending event")
	}
	require.Eventually(t, func() bool {
		info, infoErr := js.ConsumerInfo(jsStreamMessageEvents, durable)
		return infoErr == nil && info.NumPending == 0 && info.NumAckPending == 0
	}, 5*time.Second, 20*time.Millisecond, "successful delivery ACK clears the durable backlog")
	select {
	case payload := <-deliveries:
		t.Fatalf("backlogged event was delivered more than once: %#v", payload)
	default:
	}

	stopSecond()
	select {
	case <-secondDone:
	case <-time.After(5 * time.Second):
		t.Fatal("restarted message consumer did not stop after cancellation")
	}
}

func TestRouteMessageNotificationRejectsPartialGameScope(t *testing.T) {
	appID := uuid.NewString()
	err := routeMessageNotification(context.Background(), &consumer.MessageEventHandler{Router: delivery.DecideRouting},
		stubChatMembers{ids: []string{uuid.NewString(), uuid.NewString()}}, &dispatch.MessagePusher{Grouping: grouping.NewMemoryStore()}, pushenrich.NoopResolver{},
		&eventsv1.MessageStreamEvent{Payload: &eventsv1.MessageStreamEvent_MessageSent{MessageSent: &eventsv1.MessageSent{
			MessageId: uuid.NewString(), ChatId: uuid.NewString(), SenderProfileId: uuid.NewString(), GameApplicationId: &appID,
		}}})
	require.Error(t, err, "malformed scoped event must not fall through to ordinary chat push")
}

func TestRouteMessageNotification_MessageSentEmptyMetadataFailsClosed(t *testing.T) {
	err := routeMessageNotification(context.Background(), &consumer.MessageEventHandler{Router: delivery.DecideRouting}, chatmembers.NoopLister{}, &dispatch.MessagePusher{Grouping: grouping.NewMemoryStore()}, pushenrich.NoopResolver{}, &eventsv1.MessageStreamEvent{Payload: &eventsv1.MessageStreamEvent_MessageSent{
		MessageSent: &eventsv1.MessageSent{MessageId: uuid.NewString(), ChatId: uuid.NewString(), SenderProfileId: uuid.NewString()},
	}})
	require.Error(t, err)
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
	err := routeMessageNotification(context.Background(), handler, stubChatMembers{ids: []string{senderID, mentionedID}}, pusher, pushenrich.NoopResolver{}, env)
	require.NoError(t, err)
}

func TestRouteMessageNotification_SendSilentMarksPushWithoutSuppressingIt(t *testing.T) {
	senderID := uuid.New()
	recipientID := uuid.New()
	recorder := &recordingMessageFCM{}
	err := routeMessageNotification(context.Background(), &consumer.MessageEventHandler{Router: delivery.DecideRouting}, stubChatMembers{ids: []string{senderID.String(), recipientID.String()}}, &dispatch.MessagePusher{
		Tokens: messageTokenRepo{byProfile: map[uuid.UUID][]store.DeviceToken{
			recipientID: {{Token: "recipient-token", PushService: "fcm"}},
		}},
		Pusher:   &dispatch.PushDispatcher{FCM: recorder},
		Grouping: grouping.NewMemoryStore(),
	}, pushenrich.NoopResolver{}, &eventsv1.MessageStreamEvent{Payload: &eventsv1.MessageStreamEvent_MessageSent{
		MessageSent: &eventsv1.MessageSent{MessageId: uuid.NewString(), ChatId: uuid.NewString(), SenderProfileId: senderID.String(), SendSilent: true},
	}})

	require.NoError(t, err)
	require.Len(t, recorder.sent, 1)
	require.True(t, recorder.sent[0].Silent)
}

func TestRouteMessageNotification_SilentMentionDoesNotIntroduceAudiblePush(t *testing.T) {
	senderID := uuid.New()
	recipientID := uuid.New()
	recorder := &recordingMessageFCM{}
	err := routeMessageNotification(context.Background(), &consumer.MessageEventHandler{Router: delivery.DecideRouting}, stubChatMembers{ids: []string{senderID.String(), recipientID.String()}}, &dispatch.MessagePusher{
		Tokens: messageTokenRepo{byProfile: map[uuid.UUID][]store.DeviceToken{
			recipientID: {{Token: "recipient-token", PushService: "fcm"}},
		}},
		Pusher:   &dispatch.PushDispatcher{FCM: recorder},
		Grouping: grouping.NewMemoryStore(),
	}, pushenrich.NoopResolver{}, &eventsv1.MessageStreamEvent{Payload: &eventsv1.MessageStreamEvent_MentionAdded{
		MentionAdded: &eventsv1.MentionAdded{MessageId: uuid.NewString(), ChatId: uuid.NewString(), SenderProfileId: senderID.String(), MentionedProfileIds: []string{recipientID.String()}, SendSilent: true},
	}})

	require.NoError(t, err)
	require.Len(t, recorder.sent, 1)
	require.True(t, recorder.sent[0].Silent)
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
			decision, err := deliveryForMember(map[string]delivery.DeliveryDecision{"recipient": {InApp: true, Push: true}}, chatmembers.Member{
				ProfileID:  uuid.NewString(),
				IsArchived: true,
			})
			require.NoError(t, err)

			require.False(t, decision["recipient"].InApp, "archiving must suppress notification-center delivery")
			require.False(t, decision["recipient"].Push, "archived %s recipient must not receive push", chatKind)
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
	decision, err := deliveryForMember(map[string]delivery.DeliveryDecision{"recipient": {InApp: true, Push: true}}, chatmembers.Member{
		ProfileID: uuid.NewString(),
	})
	require.NoError(t, err)

	require.True(t, decision["recipient"].InApp)
	require.True(t, decision["recipient"].Push)
}

func TestRouteMessageNotification_MissingRecipientMetadataFailsClosed(t *testing.T) {
	senderID := uuid.NewString()
	recipientID := uuid.NewString()
	handler := &consumer.MessageEventHandler{Router: delivery.DecideRouting}
	members := stubChatMembers{rows: []chatmembers.Member{{ProfileID: senderID, InboxBucket: "main"}}}

	err := routeMessageNotification(context.Background(), handler, members, &dispatch.MessagePusher{Grouping: grouping.NewMemoryStore()}, parentAuthorResolver{author: recipientID}, &eventsv1.MessageStreamEvent{Payload: &eventsv1.MessageStreamEvent_MessageSent{
		MessageSent: &eventsv1.MessageSent{MessageId: uuid.NewString(), ChatId: uuid.NewString(), SenderProfileId: senderID, ThreadParentId: stringPtr("parent-message")},
	}})

	require.Error(t, err, "missing recipient routing metadata must retry rather than send")
}
