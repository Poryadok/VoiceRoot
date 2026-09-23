package consumer

import (
	"testing"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
)

func TestValidateAnalyticsDurableFailsClosedWhenAbsentOrDrifted(t *testing.T) {
	srv := startAnalyticsJetStream(t)
	nc, err := nats.Connect(srv.ClientURL())
	require.NoError(t, err)
	t.Cleanup(nc.Close)
	js, err := nc.JetStream()
	require.NoError(t, err)
	_, err = js.AddStream(&nats.StreamConfig{Name: "events", Subjects: []string{"events.>"}})
	require.NoError(t, err)

	const durable = "analytics_v2_events"
	require.Error(t, validateAnalyticsDurable(js, "events", "events.>", durable, durable))

	_, err = js.AddConsumer("events", &nats.ConsumerConfig{
		Durable:        durable,
		DeliverSubject: "_INBOX.voice.analytics." + durable,
		DeliverGroup:   durable,
		FilterSubject:  ">",
		DeliverPolicy:  nats.DeliverNewPolicy,
		AckPolicy:      nats.AckExplicitPolicy,
	})
	require.NoError(t, err)
	require.Error(t, validateAnalyticsDurable(js, "events", "events.>", durable, durable))
}

func TestValidateAnalyticsDurableAcceptsExactPreprovisionedConsumer(t *testing.T) {
	srv := startAnalyticsJetStream(t)
	nc, err := nats.Connect(srv.ClientURL())
	require.NoError(t, err)
	t.Cleanup(nc.Close)
	js, err := nc.JetStream()
	require.NoError(t, err)
	_, err = js.AddStream(&nats.StreamConfig{Name: "events", Subjects: []string{"events.>"}})
	require.NoError(t, err)

	const durable = "analytics_v2_events"
	_, err = js.AddConsumer("events", &nats.ConsumerConfig{
		Durable:        durable,
		DeliverSubject: "_INBOX.voice.analytics." + durable,
		DeliverGroup:   durable,
		FilterSubject:  "events.>",
		DeliverPolicy:  nats.DeliverNewPolicy,
		AckPolicy:      nats.AckExplicitPolicy,
	})
	require.NoError(t, err)
	require.NoError(t, validateAnalyticsDurable(js, "events", "events.>", durable, durable))
}

func provisionAnalyticsDurable(t *testing.T, js nats.JetStreamContext, stream, subject, durable, queue string) {
	t.Helper()
	_, err := js.AddConsumer(stream, &nats.ConsumerConfig{
		Durable:        durable,
		DeliverSubject: "_INBOX.voice.analytics." + durable,
		DeliverGroup:   queue,
		FilterSubject:  subject,
		DeliverPolicy:  nats.DeliverNewPolicy,
		AckPolicy:      nats.AckExplicitPolicy,
	})
	require.NoError(t, err)
}
