package main

import (
	"context"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	"voice/backend/pkg/reminderhint"
)

func TestReminderHintsBorrowSharedNATSConnectionAcrossRestart(t *testing.T) {
	s := startRealtimeJSTestServer(t)
	nc, err := nats.Connect(s.ClientURL(), natsConnectOptions("shared-reminder-test")...)
	require.NoError(t, err)
	t.Cleanup(nc.Close)
	js, err := nc.JetStream()
	require.NoError(t, err)
	_, err = js.AddStream(&nats.StreamConfig{Name: reminderhint.Stream, Subjects: []string{reminderhint.Subject}})
	require.NoError(t, err)
	const instance = "shared-reminder-test"
	durable := reminderHintDurable(instance)
	_, err = js.AddConsumer(reminderhint.Stream, &nats.ConsumerConfig{
		Durable: durable, FilterSubject: reminderhint.Subject,
		DeliverSubject: realtimeConsumerDeliverSubject(instance, "subscription_reminder"),
		AckPolicy:      nats.AckExplicitPolicy, DeliverPolicy: nats.DeliverNewPolicy,
	})
	require.NoError(t, err)
	baselineSubscriptions := nc.NumSubscriptions()
	hub := newWSHub()
	for attempt := 0; attempt < 2; attempt++ {
		ready := newRealtimeConsumerReadiness("subscription_reminder")
		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)
		finished := make(chan error, 1)
		go func() {
			finished <- runReminderHints(withRealtimeConsumerReadiness(ctx, ready, "subscription_reminder"), hub, s.ClientURL(), instance, nil, nc)
		}()
		require.Eventually(t, ready.ready, 5*time.Second, 5*time.Millisecond, "runner must bind the same durable on each start")
		require.Equal(t, 1, s.NumClients(), "runner must borrow the process connection")
		require.Greater(t, nc.NumSubscriptions(), baselineSubscriptions, "runner must own a subscription on the shared connection")
		cancel()
		select {
		case err := <-finished:
			require.ErrorIs(t, err, context.Canceled)
		case <-time.After(5 * time.Second):
			t.Fatal("reminder runner did not stop")
		}
		require.NoError(t, nc.FlushTimeout(time.Second), "runner cancellation must not drain the process connection")
		require.Equal(t, baselineSubscriptions, nc.NumSubscriptions(), "runner must release its own subscription")
		require.Eventually(t, func() bool {
			info, err := js.ConsumerInfo(reminderhint.Stream, durable)
			return err == nil && !info.PushBound
		}, time.Second, 5*time.Millisecond, "cancellation must leave the durable available for restart")
		require.Equal(t, 1, s.NumClients())
	}
}
