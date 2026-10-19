package main

import (
	"context"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
)

func TestRealtimeConsumersBorrowOneNATSConnection(t *testing.T) {
	s := startRealtimeJSTestServer(t)
	nc, err := nats.Connect(s.ClientURL(), natsConnectOptions("shared-test")...)
	require.NoError(t, err)
	t.Cleanup(nc.Close)
	js, err := nc.JetStream()
	require.NoError(t, err)
	hub := newWSHub()
	const instance = "sharedtest"
	consumers := []struct {
		name, stream, filter, durable string
		run                           func(context.Context) error
	}{
		{"message", jsStreamMessageEvents, "message.>", consumerDurableName(instance), func(ctx context.Context) error {
			return runMessageEventsConsumer(ctx, hub, s.ClientURL(), instance, nil, nc)
		}},
		{"voice", jsStreamVoiceEvents, "voice.>", voiceConsumerDurableName(instance), func(ctx context.Context) error {
			return runVoiceEventsConsumer(ctx, hub, s.ClientURL(), instance, nil, nc)
		}},
		{"chat", jsStreamChatEvents, "chat.>", chatConsumerDurableName(instance), func(ctx context.Context) error {
			return runChatEventsConsumer(ctx, hub, s.ClientURL(), instance, nil, nc)
		}},
		{"social", jsStreamSocialEvents, "social.user_blocked", socialConsumerDurableName(instance), func(ctx context.Context) error {
			return runSocialEventsConsumer(ctx, hub, s.ClientURL(), instance, nil, nc)
		}},
		{"matchmaking", jsStreamMatchmakingEvents, "mm.>", matchmakingConsumerDurableName(instance), func(ctx context.Context) error {
			return runMatchmakingEventsConsumer(ctx, hub, s.ClientURL(), instance, nil, nc)
		}},
		{"role", jsStreamRoleEvents, "role.>", roleConsumerDurableName(instance), func(ctx context.Context) error {
			return runRoleEventsConsumer(ctx, hub, s.ClientURL(), instance, nil, nc)
		}},
		{"user", jsStreamUserEvents, "user.presence_changed", userConsumerDurableName(instance), func(ctx context.Context) error {
			return runUserEventsConsumer(ctx, hub, nil, nil, s.ClientURL(), instance, nil, nc)
		}},
	}
	for _, c := range consumers {
		filter := c.filter
		if c.name == "social" {
			filter = "social.>"
		}
		_, err = js.AddStream(&nats.StreamConfig{Name: c.stream, Subjects: []string{filter}})
		require.NoError(t, err)
		preprovisionRealtimeConsumer(t, js, c.stream, c.durable, c.filter)
	}
	preprovisionRealtimeConsumer(t, js, jsStreamSocialEvents, friendRequestConsumerDurableName(instance), "social.friend_request")
	preprovisionRealtimeConsumer(t, js, jsStreamSocialEvents, friendRemovedConsumerDurableName(instance), "social.friend_removed")
	for _, c := range consumers {
		t.Run(c.name, func(t *testing.T) {
			tracker := newRealtimeConsumerReadiness(c.name)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			finished := make(chan error, 1)
			go func() { finished <- c.run(withRealtimeConsumerReadiness(ctx, tracker, c.name)) }()
			require.Eventually(t, tracker.ready, 5*time.Second, 5*time.Millisecond)
			require.Equal(t, 1, s.NumClients(), "consumer must not create another connection")
			cancel()
			select {
			case err := <-finished:
				require.ErrorIs(t, err, context.Canceled)
			case <-time.After(5 * time.Second):
				t.Fatal("consumer did not stop")
			}
			require.NoError(t, nc.FlushTimeout(time.Second), "consumer shutdown must leave shared connection usable")
		})
	}
	// Both readiness and the delivery publisher use the same live transport.
	_, err = newJetstreamDeliveryAckPublisher(nc)
	require.NoError(t, err)
	for i := 0; i < 3; i++ {
		status, _ := checkReadiness(context.Background(), readinessDeps{NatsURL: s.ClientURL(), NATS: nc})
		require.Equal(t, "ok", status)
	}
	require.Equal(t, 1, s.NumClients())
	nc.Close()
	status, reason := checkReadiness(context.Background(), readinessDeps{NatsURL: s.ClientURL(), NATS: nc})
	require.Equal(t, "degraded", status)
	require.Equal(t, "nats_unavailable", reason)
}

func TestConsumerNATSConnectionRejectsInvalidBorrow(t *testing.T) {
	_, _, err := consumerNATSConnection("nats://unused", "test", nil)
	require.Error(t, err)
	_, _, err = consumerNATSConnection("nats://unused", "test", nil, nil)
	require.Error(t, err)
}
