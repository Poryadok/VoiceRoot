package consumer

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"

	"voice/backend/analytics/internal/adapters"
	"voice/backend/analytics/internal/buffer"
	"voice/backend/analytics/internal/store"
)

func TestHandleAnalyticsMsgUsesJetStreamIdentityForInvalidEnvelopeID(t *testing.T) {
	srv := startAnalyticsJetStream(t)
	nc, err := nats.Connect(srv.ClientURL())
	require.NoError(t, err)
	t.Cleanup(nc.Close)
	js, err := nc.JetStream()
	require.NoError(t, err)
	_, err = js.AddStream(&nats.StreamConfig{Name: "analytics_events", Subjects: []string{"analytics.>"}})
	require.NoError(t, err)

	const durable = "analytics_v2_identity"
	provisionAnalyticsDurable(t, js, "analytics_events", "analytics.>", durable, durable)
	sub, err := js.QueueSubscribeSync("analytics.>", durable, nats.Bind("analytics_events", durable), nats.ManualAck())
	require.NoError(t, err)
	t.Cleanup(func() { _ = sub.Unsubscribe() })

	var mu sync.Mutex
	var rows []store.EventRow
	acc := buffer.New(1, time.Hour, func(_ context.Context, batch []store.EventRow) error {
		mu.Lock()
		defer mu.Unlock()
		rows = append(rows, batch...)
		return nil
	}, nil)
	runner := &Runner{Mapper: adapters.Mapper{}, Buffer: acc}

	publish := func() *nats.Msg {
		_, err := js.Publish("analytics.test", []byte(`{"eventId":"legacy-not-uuid","eventType":"test","sourceService":"test"}`))
		require.NoError(t, err)
		msg, err := sub.NextMsg(time.Second)
		require.NoError(t, err)
		return msg
	}
	first := publish()
	require.NoError(t, runner.handleAnalyticsMsg(first))
	// The same redelivered source message must retain its identity.
	require.NoError(t, runner.handleAnalyticsMsg(first))
	second := publish()
	require.NoError(t, runner.handleAnalyticsMsg(second))

	mu.Lock()
	got := append([]store.EventRow(nil), rows...)
	mu.Unlock()
	require.Len(t, got, 3)
	require.Equal(t, got[0].EventID, got[1].EventID)
	require.NotEqual(t, got[0].EventID, got[2].EventID)
}
