package main

import (
	"bytes"
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	natsserver "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	eventsv1 "voice.app/voice/events/v1"
	"voice/backend/notification/internal/consumer"
)

type subscriptionConsumptionLog struct {
	mu     sync.Mutex
	buffer bytes.Buffer
}

func (l *subscriptionConsumptionLog) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buffer.Write(p)
}

func (l *subscriptionConsumptionLog) snapshot() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buffer.String()
}

func TestSubscriptionEventsJetStreamRestartRetainsCanonicalReminderUntilDeliveryRuntimeExists(t *testing.T) {
	testSubscriptionEventsJetStreamRetainsReminder(t, 1)
}

func TestSubscriptionEventsJetStreamRestartRetainsLegacyReminderUntilDeliveryRuntimeExists(t *testing.T) {
	testSubscriptionEventsJetStreamRetainsReminder(t, 0)
}

func testSubscriptionEventsJetStreamRetainsReminder(t *testing.T, version uint32) {
	// A recognized canonical reminder is not a durable delivery. Until A7's
	// protected authority and channel runtime exist, preserve it for retry.
	server, err := natsserver.NewServer(&natsserver.Options{JetStream: true, StoreDir: t.TempDir(), Port: -1})
	require.NoError(t, err)
	go server.Start()
	t.Cleanup(server.Shutdown)
	require.True(t, server.ReadyForConnections(10*time.Second))
	provisioner, err := nats.Connect(server.ClientURL())
	require.NoError(t, err)
	t.Cleanup(provisioner.Close)
	js, err := provisioner.JetStream()
	require.NoError(t, err)
	_, err = js.AddStream(&nats.StreamConfig{Name: jsStreamSubscriptionEvents, Subjects: []string{"subscription.>"}})
	require.NoError(t, err)
	durable := consumer.SharedDurable("subscription")
	_, err = js.AddConsumer(jsStreamSubscriptionEvents, &nats.ConsumerConfig{
		Durable: durable, FilterSubject: "subscription.>", DeliverSubject: "_INBOX.voice.notification.subscription",
		AckPolicy: nats.AckExplicitPolicy, DeliverPolicy: nats.DeliverNewPolicy,
	})
	require.NoError(t, err)
	var recorded subscriptionConsumptionLog
	logger := slog.New(slog.NewJSONHandler(&recorded, nil))
	start := func() *notificationConsumerRun {
		runner := startNotificationAcceptanceConsumer(t, "subscription", func(ctx context.Context) error {
			return runSubscriptionEventsConsumer(ctx, server.ClientURL(), logger)
		})
		waitNotificationConsumerPushBound(t, js, jsStreamSubscriptionEvents, durable, true)
		return runner
	}
	first := start()
	err, joined := first.stop()
	require.True(t, joined, "subscription consumer is joined before publishing backlog")
	require.ErrorIs(t, err, context.Canceled)
	waitNotificationConsumerPushBound(t, js, jsStreamSubscriptionEvents, durable, false)
	before, err := js.ConsumerInfo(jsStreamSubscriptionEvents, durable)
	require.NoError(t, err)

	encoded, err := proto.Marshal(&eventsv1.SubscriptionStreamEvent{ProtocolVersion: version, Payload: &eventsv1.SubscriptionStreamEvent_GraceReminder{
		GraceReminder: &eventsv1.GraceReminder{AccountId: uuid.NewString(), Plan: "premium", Day: 1},
	}})
	require.NoError(t, err)
	published, err := js.Publish("subscription.grace_reminder", encoded)
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		info, infoErr := js.ConsumerInfo(jsStreamSubscriptionEvents, durable)
		return infoErr == nil && info.NumPending == 1 && info.NumAckPending == 0 && info.AckFloor.Stream < published.Sequence
	}, 5*time.Second, 20*time.Millisecond, "grace reminder stays pending while the durable consumer is stopped")
	require.Empty(t, recorded.snapshot(), "stopped consumer must not handle the backlog")

	second := start()
	require.Eventually(t, func() bool {
		info, infoErr := js.ConsumerInfo(jsStreamSubscriptionEvents, durable)
		return infoErr == nil && info.NumPending == 0 && info.NumAckPending == 1 &&
			info.AckFloor.Stream < published.Sequence && info.NumRedelivered > 0 && info.Created.Equal(before.Created)
	}, 5*time.Second, 20*time.Millisecond, "rebound durable retries canonical work instead of ACKing a recognized stub")
	err, joined = second.stop()
	require.True(t, joined, "restarted subscription consumer is joined before fixture shutdown")
	require.ErrorIs(t, err, context.Canceled)
	require.NotContains(t, recorded.snapshot(), `"msg":"subscription notification event consumed"`)
	require.Contains(t, recorded.snapshot(), `"subject":"subscription.grace_reminder"`)
	require.Contains(t, recorded.snapshot(), "subscription delivery runtime unavailable")
}
