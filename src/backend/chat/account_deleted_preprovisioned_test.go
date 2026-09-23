package main

import (
	"testing"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
)

func TestAccountDeletedDurableIsFixedAcrossInstances(t *testing.T) {
	require.Equal(t, accountDeletedDurableName("pod-a"), accountDeletedDurableName("pod-b"))
}

func TestValidateAccountDeletedDurableFailsClosedWhenAbsentOrUnfiltered(t *testing.T) {
	server := startAccountDeletedJSTestServer(t)
	nc, err := nats.Connect(server.ClientURL())
	require.NoError(t, err)
	t.Cleanup(func() { _ = nc.Drain() })
	js, err := nc.JetStream()
	require.NoError(t, err)
	_, err = js.AddStream(&nats.StreamConfig{Name: userEventsStreamName, Subjects: []string{"user.>"}, Storage: nats.MemoryStorage})
	require.NoError(t, err)

	durable := accountDeletedDurableName("pod-a")
	require.Error(t, validateAccountDeletedDurable(js, durable))
	_, err = js.AddConsumer(userEventsStreamName, &nats.ConsumerConfig{
		Durable:        durable,
		DeliverSubject: accountDeletedDeliverySubject(durable),
		DeliverPolicy:  nats.DeliverAllPolicy,
		AckPolicy:      nats.AckExplicitPolicy,
	})
	require.NoError(t, err)
	require.Error(t, validateAccountDeletedDurable(js, durable))
}

func TestValidateAccountDeletedDurableAcceptsExactPreprovisionedConsumer(t *testing.T) {
	server := startAccountDeletedJSTestServer(t)
	nc, err := nats.Connect(server.ClientURL())
	require.NoError(t, err)
	t.Cleanup(func() { _ = nc.Drain() })
	js, err := nc.JetStream()
	require.NoError(t, err)
	_, err = js.AddStream(&nats.StreamConfig{Name: userEventsStreamName, Subjects: []string{"user.>"}, Storage: nats.MemoryStorage})
	require.NoError(t, err)

	durable := accountDeletedDurableName("pod-a")
	_, err = js.AddConsumer(userEventsStreamName, accountDeletedConsumerConfig(durable))
	require.NoError(t, err)
	require.NoError(t, validateAccountDeletedDurable(js, durable))
}

func provisionAccountDeletedDurable(t *testing.T, js nats.JetStreamContext) {
	t.Helper()
	durable := accountDeletedDurableName("bootstrap")
	_, err := js.AddConsumer(userEventsStreamName, accountDeletedConsumerConfig(durable))
	require.NoError(t, err)
}
