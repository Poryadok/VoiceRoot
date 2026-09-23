package main

import (
	"testing"

	"github.com/nats-io/nats.go"
)

// preprovisionRealtimeConsumer mirrors the bootstrap contract. Tests must not
// rely on a Realtime subscription creating a consumer as a side effect.
func preprovisionRealtimeConsumer(t *testing.T, js nats.JetStreamContext, stream, durable, filter string) {
	t.Helper()
	_, err := js.AddConsumer(stream, &nats.ConsumerConfig{
		Durable:        durable,
		DeliverSubject: "_INBOX.voice.realtime.test." + durable,
		FilterSubject:  filter,
		DeliverPolicy:  nats.DeliverNewPolicy,
		AckPolicy:      nats.AckExplicitPolicy,
	})
	if err != nil {
		t.Fatalf("pre-provision consumer %s/%s: %v", stream, durable, err)
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
