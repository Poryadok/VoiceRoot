package main

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	eventsv1 "voice.app/voice/events/v1"
	userv1 "voice.app/voice/user/v1"
	"voice/backend/notification/internal/consumer"
	"voice/backend/notification/internal/dispatch"
	"voice/backend/notification/internal/grouping"
	"voice/backend/notification/internal/push"
	"voice/backend/notification/internal/pushenrich"
)

func TestNotificationMessageConsumerUsesRealUserPrivatePresenceDespitePublicPrivacy(t *testing.T) {
	fixture := newNotificationConsumerRestartFixture(t)
	user := newUserPresenceRuntimeFixture(t)
	senderID := uuid.New()
	for _, id := range []uuid.UUID{senderID, user.profileID} {
		_, err := fixture.tokens.Register(context.Background(), id, "web", "presence-proof-"+id.String(), "fcm")
		require.NoError(t, err)
	}
	_, err := fixture.js.AddStream(&nats.StreamConfig{Name: jsStreamMessageEvents, Subjects: []string{jsSubjectMessageEvents}})
	require.NoError(t, err)
	durable := consumer.SharedDurable("message")
	_, err = fixture.js.AddConsumer(jsStreamMessageEvents, &nats.ConsumerConfig{Durable: durable, FilterSubject: jsSubjectMessageEvents,
		DeliverSubject: "_INBOX.voice.notification.message", AckPolicy: nats.AckExplicitPolicy, DeliverPolicy: nats.DeliverNewPolicy, AckWait: 30 * time.Second})
	require.NoError(t, err)
	deliveries := make(chan notificationRestartPush, 16)
	barrier := &userPresenceOutageBarrier{checker: user.checker, second: make(chan struct{}), release: make(chan struct{})}
	pusher := &dispatch.MessagePusher{Tokens: fixture.tokens, Presence: barrier, Grouping: grouping.NewMemoryStore(), Pusher: &dispatch.PushDispatcher{FCM: notificationRestartFCM{sent: deliveries}}, GameChatScope: ordinaryGameChatScope{}}
	members := stubChatMembers{ids: []string{senderID.String(), user.profileID.String()}}
	runCtx, cancel := context.WithCancel(context.Background())
	readiness := newNotificationConsumerReadiness("message")
	runner := &notificationConsumerRun{cancel: cancel, done: make(chan error, 1), exited: make(chan struct{})}
	go func() {
		defer close(runner.exited)
		runner.done <- runMessageEventsConsumer(withNotificationConsumerReadiness(runCtx, readiness, "message"), fixture.serverURL, fixture.tokens, members, pusher, pushenrich.NoopResolver{}, nil)
	}()
	t.Cleanup(func() {
		barrier.unblock()
		if _, joined := runner.stop(); !joined {
			t.Error("message consumer did not join before User cleanup")
		}
	})
	require.Eventually(t, readiness.ready, 5*time.Second, 10*time.Millisecond, "actual message consumer binds precreated durable")

	assertNoPush := func(t *testing.T) {
		t.Helper()
		select {
		case got := <-deliveries:
			t.Fatalf("unexpected push, including sender or duplicate: %#v", got)
		default:
		}
	}
	publish := func(t *testing.T) (uint64, push.Payload) {
		t.Helper()
		messageID, chatID := uuid.NewString(), uuid.NewString()
		event := &eventsv1.MessageStreamEvent{EventId: uuid.NewString(), Payload: &eventsv1.MessageStreamEvent_MessageSent{MessageSent: &eventsv1.MessageSent{MessageId: messageID, ChatId: chatID, SenderProfileId: senderID.String()}}}
		encoded, encodeErr := proto.Marshal(event)
		require.NoError(t, encodeErr)
		ack, publishErr := fixture.js.Publish("message.sent", encoded)
		require.NoError(t, publishErr)
		return ack.Sequence, push.Payload{Title: "New message", Body: "You have a new message", CollapseTag: "push:group:" + user.profileID.String() + ":" + chatID, Counter: 1,
			Data: map[string]string{"type": "new_message", "chat_id": chatID, "message_id": messageID, "sender_profile_id": senderID.String(), "deep_link": "https://voice.gg/ch/" + chatID + "/m/" + messageID}}
	}
	acked := func(t *testing.T, sequence uint64) {
		t.Helper()
		require.Eventually(t, func() bool {
			info, e := fixture.js.ConsumerInfo(jsStreamMessageEvents, durable)
			return e == nil && info.NumPending == 0 && info.NumAckPending == 0 && info.AckFloor.Stream >= sequence
		}, 5*time.Second, 20*time.Millisecond, "actual consumer ACKs the handled event")
	}
	assertPush := func(t *testing.T, expected push.Payload) {
		t.Helper()
		select {
		case got := <-deliveries:
			require.Equal(t, user.profileID, got.profileID, "only recipient is pushed; sender has a registered token too")
			require.Equal(t, expected, got.payload)
		case <-time.After(5 * time.Second):
			t.Fatal("eligible recipient was not pushed")
		}
	}
	publicMasked := func(t *testing.T) {
		t.Helper()
		ctx, stop := context.WithTimeout(context.Background(), 2*time.Second)
		defer stop()
		response, e := user.public.GetBulkPresence(ctx, &userv1.GetBulkPresenceRequest{ProfileIds: []string{user.profileID.String()}})
		require.NoError(t, e)
		visible := response.GetByProfileId()[user.profileID.String()]
		require.NotNil(t, visible)
		require.Equal(t, user.profileID.String(), visible.GetProfileId())
		require.Empty(t, visible.GetStatus(), "public offline projection carries no live status")
		require.Equal(t, userv1.PresenceOnlineStatus_PRESENCE_ONLINE_STATUS_UNSPECIFIED, visible.GetStatusEnum())
		require.Nil(t, visible.LastSeen)
		require.Nil(t, visible.GameTitle)
		require.Nil(t, visible.CustomStatus)
		require.Nil(t, visible.CallInfoJson)
	}

	// No live session has been written: actual User returns offline, not a fake.
	t.Run("offline_delivers_and_acks", func(t *testing.T) {
		publicMasked(t)
		sequence, expected := publish(t)
		assertPush(t, expected)
		acked(t, sequence)
		assertNoPush(t)
	})
	for _, state := range []string{"online", "idle", "dnd"} {
		t.Run(state+"_public_masked_private_suppressed", func(t *testing.T) {
			user.setPresence(t, state)
			publicMasked(t)
			ctx, stop := context.WithTimeout(context.Background(), 2*time.Second)
			online, e := user.checker.IsOnline(ctx, user.profileID)
			stop()
			require.NoError(t, e)
			require.True(t, online, "actual authenticated authority sees the live session hidden by public privacy")
			sequence, _ := publish(t)
			acked(t, sequence)
			assertNoPush(t)
		})
	}
	t.Run("invisible_delivers_and_acks", func(t *testing.T) {
		user.setPresence(t, "invisible")
		publicMasked(t)
		sequence, expected := publish(t)
		assertPush(t, expected)
		acked(t, sequence)
		assertNoPush(t)
	})
	t.Run("routing_is_private_and_request_bound", func(t *testing.T) {
		req := &userv1.GetNotificationRoutingPresenceRequest{ProfileId: user.profileID.String()}
		ctx, stop := context.WithTimeout(context.Background(), 2*time.Second)
		_, e := user.public.GetNotificationRoutingPresence(ctx, req)
		stop()
		require.Equal(t, codes.Unimplemented, status.Code(e), "ordinary User listener cannot expose internal routing")
		for _, test := range []struct {
			name, issuer string
			bound        *userv1.GetNotificationRoutingPresenceRequest
		}{
			{name: "unsigned"}, {name: "wrong_principal", issuer: "social", bound: req},
			{name: "different_request_body", issuer: "notification", bound: &userv1.GetNotificationRoutingPresenceRequest{ProfileId: uuid.NewString()}},
		} {
			t.Run(test.name, func(t *testing.T) {
				base := context.Background()
				if test.issuer != "" {
					base = user.signedContext(t, test.issuer, test.bound)
				}
				ctx, stop := context.WithTimeout(base, 2*time.Second)
				defer stop()
				_, e := user.private.GetNotificationRoutingPresence(ctx, req)
				require.Equal(t, codes.Unauthenticated, status.Code(e))
			})
		}
	})
	t.Run("actual_authority_outage_naks_then_recovers", func(t *testing.T) {
		barrier.armed.Store(true)
		user.process.stop(t)
		ctx, stop := context.WithTimeout(context.Background(), 2*time.Second)
		_, e := user.checker.IsOnline(ctx, user.profileID)
		stop()
		require.Equal(t, codes.Unavailable, status.Code(e), "absence of actual User is an authority error")
		sequence, expected := publish(t)
		select {
		case <-barrier.second:
		case <-time.After(5 * time.Second):
			t.Fatal("actual User error was not explicitly NAKed and redelivered")
		}
		info, e := fixture.js.ConsumerInfo(jsStreamMessageEvents, durable)
		require.NoError(t, e)
		require.EqualValues(t, 1, info.NumRedelivered, "redelivery precedes the 30-second AckWait")
		require.EqualValues(t, 1, info.NumAckPending)
		require.Less(t, info.AckFloor.Stream, sequence, "failed presence resolution must not ACK")
		assertNoPush(t)
		user.start(t)
		barrier.unblock()
		assertPush(t, expected)
		acked(t, sequence)
		require.EqualValues(t, 2, barrier.errors.Load(), "only two real outage results were returned before recovery")
		assertNoPush(t)
	})
	err, joined := runner.stop()
	require.True(t, joined)
	require.ErrorIs(t, err, context.Canceled)
	assertNoPush(t)
}
