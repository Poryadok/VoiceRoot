package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
)

// preprovisionRealtimeConsumer mirrors the bootstrap contract. Tests must not
// rely on a Realtime subscription creating a consumer as a side effect.
func preprovisionRealtimeConsumer(t *testing.T, js nats.JetStreamContext, stream, durable, filter string) {
	t.Helper()
	consumer := map[string]string{
		jsStreamMessageEvents:     "message",
		jsStreamChatEvents:        "chat",
		jsStreamUserEvents:        "user",
		jsStreamSocialEvents:      "social",
		jsStreamRoleEvents:        "role",
		jsStreamVoiceEvents:       "voice",
		jsStreamMatchmakingEvents: "matchmaking",
	}[stream]
	if consumer == "" {
		t.Fatalf("missing test consumer target for stream %q", stream)
	}
	suffix := map[string]string{
		"message": "_msg", "chat": "_chat", "user": "_user", "social": "_social",
		"role": "_role", "voice": "_voice", "matchmaking": "_mm",
	}[consumer]
	instanceID := strings.TrimSuffix(strings.TrimPrefix(durable, "rt_"), suffix)
	_, err := js.AddConsumer(stream, &nats.ConsumerConfig{
		Durable:        durable,
		DeliverSubject: realtimeConsumerDeliverSubject(instanceID, consumer),
		FilterSubject:  filter,
		DeliverPolicy:  nats.DeliverNewPolicy,
		AckPolicy:      nats.AckExplicitPolicy,
	})
	if err != nil {
		t.Fatalf("pre-provision consumer %s/%s: %v", stream, durable, err)
	}
}

func TestRunUserEventsConsumerRejectsBroadDurableWithoutHistoricalDelivery(t *testing.T) {
	s := startRealtimeJSTestServer(t)
	nc, err := nats.Connect(s.ClientURL())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(nc.Close)
	js, err := nc.JetStream()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := js.AddStream(&nats.StreamConfig{Name: jsStreamUserEvents, Subjects: []string{"user.>"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := js.Publish("user.account_deleted", []byte("historical-account-deleted")); err != nil {
		t.Fatal(err)
	}

	instanceID := "realtime-security-test"
	durable := userConsumerDurableName(instanceID)
	preprovisionRealtimeConsumer(t, js, jsStreamUserEvents, durable, "user.presence_changed")
	if err := js.DeleteConsumer(jsStreamUserEvents, durable); err != nil {
		t.Fatal(err)
	}
	if _, err := js.AddConsumer(jsStreamUserEvents, &nats.ConsumerConfig{
		Durable:        durable,
		DeliverSubject: realtimeConsumerDeliverSubject(instanceID, "user"),
		FilterSubject:  "user.>",
		DeliverPolicy:  nats.DeliverAllPolicy,
		AckPolicy:      nats.AckExplicitPolicy,
	}); err != nil {
		t.Fatal(err)
	}

	ready := newRealtimeConsumerReadiness("user")
	ctx, cancel := context.WithTimeout(withRealtimeConsumerReadiness(context.Background(), ready, "user"), time.Second)
	defer cancel()
	err = runUserEventsConsumer(ctx, newWSHub(), nil, nil, s.ClientURL(), instanceID, nil)
	if err == nil || !strings.Contains(err.Error(), "incompatible configuration") {
		t.Fatalf("runUserEventsConsumer error = %v, want incompatible configuration", err)
	}
	if ready.ready() {
		t.Fatal("consumer became ready after rejecting broadened durable")
	}
	if status, reason := checkReadiness(t.Context(), readinessDeps{Consumers: ready}); status != "degraded" || reason != "jetstream_consumers_unready" {
		t.Fatalf("readiness = %q, %q; want unready", status, reason)
	}
	info, err := js.ConsumerInfo(jsStreamUserEvents, durable)
	if err != nil {
		t.Fatal(err)
	}
	if info.NumPending != 1 {
		t.Fatalf("historical account_deleted pending = %d, want 1 because Realtime never bound", info.NumPending)
	}
}

func TestRealtimeSubscribersBindOnlyExactConsumers(t *testing.T) {
	s := startRealtimeJSTestServer(t)
	nc, err := nats.Connect(s.ClientURL())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(nc.Close)
	js, err := nc.JetStream()
	if err != nil {
		t.Fatal(err)
	}
	instanceID := "realtime-exact-config"
	hub := newWSHub()
	cases := []struct {
		name, stream, subject, durable, filter string
		subscribe                              func() (*nats.Subscription, error)
	}{
		{"message", jsStreamMessageEvents, "message.>", consumerDurableName(instanceID), "message.>", func() (*nats.Subscription, error) { return subscribeMessageEvents(js, hub, instanceID, nil) }},
		{"chat", jsStreamChatEvents, "chat.>", chatConsumerDurableName(instanceID), "chat.>", func() (*nats.Subscription, error) { return subscribeChatEvents(js, hub, instanceID, nil) }},
		{"user", jsStreamUserEvents, "user.>", userConsumerDurableName(instanceID), "user.presence_changed", func() (*nats.Subscription, error) { return subscribeUserEvents(js, hub, nil, nil, instanceID, nil) }},
		{"social", jsStreamSocialEvents, "social.>", socialConsumerDurableName(instanceID), "social.user_blocked", func() (*nats.Subscription, error) { return subscribeSocialEvents(js, hub, instanceID, nil) }},
		{"role", jsStreamRoleEvents, "role.>", roleConsumerDurableName(instanceID), "role.>", func() (*nats.Subscription, error) { return subscribeRoleEvents(js, hub, instanceID, nil) }},
		{"voice", jsStreamVoiceEvents, "voice.>", voiceConsumerDurableName(instanceID), "voice.>", func() (*nats.Subscription, error) { return subscribeVoiceEvents(js, hub, instanceID, nil) }},
		{"matchmaking", jsStreamMatchmakingEvents, "mm.>", matchmakingConsumerDurableName(instanceID), "mm.>", func() (*nats.Subscription, error) { return subscribeMatchmakingEvents(js, hub, instanceID, nil) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := js.AddStream(&nats.StreamConfig{Name: tc.stream, Subjects: []string{tc.subject}}); err != nil {
				t.Fatal(err)
			}
			preprovisionRealtimeConsumer(t, js, tc.stream, tc.durable, tc.filter)
			sub, err := tc.subscribe()
			if err != nil {
				t.Fatalf("bind exact consumer: %v", err)
			}
			t.Cleanup(func() { _ = sub.Unsubscribe() })
		})
	}
}

func TestSubscribeRoleEventsFailsClosedWhenConsumerIsMissing(t *testing.T) {
	s := startRealtimeJSTestServer(t)
	nc, err := nats.Connect(s.ClientURL())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(nc.Close)
	js, err := nc.JetStream()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := js.AddStream(&nats.StreamConfig{Name: jsStreamRoleEvents, Subjects: []string{"role.>"}}); err != nil {
		t.Fatal(err)
	}

	if _, err := subscribeRoleEvents(js, newWSHub(), "missing-consumer", nil); err == nil {
		t.Fatal("subscribeRoleEvents succeeded without its centrally pre-provisioned consumer")
	}
	if _, err := js.ConsumerInfo(jsStreamRoleEvents, roleConsumerDurableName("missing-consumer")); err == nil {
		t.Fatal("subscribeRoleEvents created a consumer instead of failing closed")
	}
}
