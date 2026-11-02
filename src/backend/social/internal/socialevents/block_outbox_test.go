package socialevents

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
)

func TestBlockOutboxReplayUsesPersistedBytesAndDeduplicates(t *testing.T) {
	broker := startJSTestServer(t)
	nc, err := nats.Connect(broker.ClientURL())
	require.NoError(t, err)
	t.Cleanup(nc.Close)
	js, err := nc.JetStream()
	require.NoError(t, err)
	_, err = js.AddStream(&nats.StreamConfig{Name: streamName, Subjects: []string{subjectUserBlocked}})
	require.NoError(t, err)
	pub, err := NewJetStreamPublisher(broker.ClientURL())
	require.NoError(t, err)
	t.Cleanup(func() { _ = pub.Close() })
	id, payload, err := NewBlockOutboxEvent(uuid.New(), uuid.New())
	require.NoError(t, err)
	for range 2 {
		require.NoError(t, pub.PublishBlockOutboxEvent(context.Background(), id, payload))
	}
	info, err := js.StreamInfo(streamName)
	require.NoError(t, err)
	require.Equal(t, uint64(1), info.State.Msgs)
	stored, err := js.GetMsg(streamName, 1)
	require.NoError(t, err)
	require.Equal(t, payload, stored.Data)
	require.Equal(t, id.String(), stored.Header.Get(nats.MsgIdHdr))
	require.Error(t, pub.PublishBlockOutboxEvent(context.Background(), uuid.New(), payload), "mismatched immutable identity must not publish")
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	require.Error(t, pub.PublishBlockOutboxEvent(canceled, id, payload), "canceled publication is not a positive PubAck")
}
