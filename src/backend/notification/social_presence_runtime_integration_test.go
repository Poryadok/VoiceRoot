package main

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	eventsv1 "voice.app/voice/events/v1"
	"voice/backend/notification/internal/consumer"
	"voice/backend/notification/internal/dispatch"
)

// This proof uses the actual signed/TLS User listener and real Social consumer,
// with an embedded JetStream server and a recording push sender, never a provider.
func TestNotificationSocialConsumerUsesRealUserRoutingPresence(t *testing.T) {
	fixture := newNotificationConsumerRestartFixture(t)
	user := newUserPresenceRuntimeFixture(t)
	sender := uuid.New()
	_, err := fixture.tokens.Register(t.Context(), user.profileID, "web", "social-presence-proof", "fcm")
	require.NoError(t, err)
	_, err = fixture.js.AddStream(&nats.StreamConfig{Name: jsStreamSocialEvents, Subjects: []string{"social.>"}})
	require.NoError(t, err)
	durable := consumer.SharedDurable("social")
	_, err = fixture.js.AddConsumer(jsStreamSocialEvents, &nats.ConsumerConfig{Durable: durable, FilterSubject: "social.>", DeliverSubject: "_INBOX.voice.notification.social", AckPolicy: nats.AckExplicitPolicy, DeliverPolicy: nats.DeliverNewPolicy, AckWait: 30 * time.Second})
	require.NoError(t, err)
	deliveries := make(chan notificationRestartPush, 16)
	barrier := &userPresenceOutageBarrier{checker: user.checker, second: make(chan struct{}), release: make(chan struct{})}
	runCtx, cancel := context.WithCancel(context.Background())
	ready := newNotificationConsumerReadiness("social")
	runner := &notificationConsumerRun{cancel: cancel, done: make(chan error, 1), exited: make(chan struct{})}
	go func() {
		defer close(runner.exited)
		runner.done <- runSocialEventsConsumer(withNotificationConsumerReadiness(runCtx, ready, "social"), fixture.serverURL, fixture.tokens, &dispatch.PushDispatcher{FCM: notificationRestartFCM{sent: deliveries}}, barrier, nil, nil)
	}()
	t.Cleanup(func() {
		barrier.unblock()
		if _, joined := runner.stop(); !joined {
			t.Error("Social consumer did not join before User cleanup")
		}
	})
	require.Eventually(t, ready.ready, 5*time.Second, 10*time.Millisecond)
	noPush := func() {
		t.Helper()
		select {
		case got := <-deliveries:
			t.Fatalf("unexpected push: %#v", got)
		default:
		}
	}
	acked := func(sequence uint64) {
		t.Helper()
		require.Eventually(t, func() bool {
			info, e := fixture.js.ConsumerInfo(jsStreamSocialEvents, durable)
			return e == nil && info.NumPending == 0 && info.NumAckPending == 0 && info.AckFloor.Stream >= sequence
		}, 5*time.Second, 20*time.Millisecond)
	}
	publish := func() (uint64, string) {
		t.Helper()
		request := uuid.NewString()
		data, e := proto.Marshal(&eventsv1.SocialStreamEvent{Payload: &eventsv1.SocialStreamEvent_FriendRequest{FriendRequest: &eventsv1.FriendRequest{RequestId: request, RequesterProfileId: sender.String(), TargetProfileId: user.profileID.String()}}})
		require.NoError(t, e)
		ack, e := fixture.js.Publish("social.friend_request", data)
		require.NoError(t, e)
		return ack.Sequence, request
	}
	pushed := func(request string) {
		t.Helper()
		select {
		case got := <-deliveries:
			require.Equal(t, user.profileID, got.profileID)
			require.Equal(t, request, got.payload.Data["friend_request_id"])
		case <-time.After(5 * time.Second):
			t.Fatal("eligible friend push missing")
		}
	}
	// Initial actual User store has no session, hence offline.
	sequence, request := publish()
	pushed(request)
	acked(sequence)
	noPush()
	for _, state := range []string{"online", "idle", "dnd"} {
		t.Run(state, func(t *testing.T) { user.setPresence(t, state); sequence, _ := publish(); acked(sequence); noPush() })
	}
	user.setPresence(t, "invisible")
	sequence, request = publish()
	pushed(request)
	acked(sequence)
	noPush()
	barrier.armed.Store(true)
	user.process.stop(t)
	sequence, request = publish()
	select {
	case <-barrier.second:
	case <-time.After(5 * time.Second):
		t.Fatal("authority outage did not NAK/redeliver before AckWait")
	}
	info, err := fixture.js.ConsumerInfo(jsStreamSocialEvents, durable)
	require.NoError(t, err)
	require.EqualValues(t, 1, info.NumRedelivered)
	require.EqualValues(t, 1, info.NumAckPending)
	require.Less(t, info.AckFloor.Stream, sequence)
	noPush()
	user.start(t)
	barrier.unblock()
	pushed(request)
	acked(sequence)
	noPush()
	err, joined := runner.stop()
	require.True(t, joined)
	require.ErrorIs(t, err, context.Canceled)
}
