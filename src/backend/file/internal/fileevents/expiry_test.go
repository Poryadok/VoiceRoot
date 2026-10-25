package fileevents

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

type lostExpiryAck struct {
	inner jetStreamClient
	lost  bool
}

func (j *lostExpiryAck) StreamInfo(name string, options ...nats.JSOpt) (*nats.StreamInfo, error) {
	return j.inner.StreamInfo(name, options...)
}
func (j *lostExpiryAck) PublishMsg(msg *nats.Msg, options ...nats.PubOpt) (*nats.PubAck, error) {
	ack, err := j.inner.PublishMsg(msg, options...)
	if err == nil && !j.lost {
		j.lost = true
		return nil, errors.New("injected lost PubAck")
	}
	return ack, err
}

func TestExpiryOutboxJetStreamLostAckRetriesStableEnvelope(t *testing.T) {
	server := startJSTestServer(t)
	pub, err := NewJetStreamPublisher(server.ClientURL())
	require.NoError(t, err)
	t.Cleanup(func() { _ = pub.Close() })
	nc, err := nats.Connect(server.ClientURL())
	require.NoError(t, err)
	t.Cleanup(nc.Close)
	sub, err := nc.SubscribeSync(subjectFileExpired)
	require.NoError(t, err)
	require.NoError(t, nc.Flush())
	event, err := NewExpiryEvent(uuid.NewString(), nil)
	require.NoError(t, err)
	raw, err := (proto.MarshalOptions{Deterministic: true}).Marshal(event)
	require.NoError(t, err)
	pub.js = &lostExpiryAck{inner: pub.js}
	require.ErrorContains(t, pub.PublishExpiryEvent(context.Background(), raw), "injected lost PubAck")
	require.NoError(t, pub.PublishExpiryEvent(context.Background(), raw))
	msg, err := sub.NextMsg(3 * time.Second)
	require.NoError(t, err)
	require.Equal(t, event.EventId, msg.Header.Get(nats.MsgIdHdr))
	require.Equal(t, raw, msg.Data)
	js, err := nc.JetStream()
	require.NoError(t, err)
	info, err := js.StreamInfo(streamName)
	require.NoError(t, err)
	require.EqualValues(t, 1, info.State.Msgs, "same stable event ID must deduplicate uncertain delivery")
}

func TestExpiryOutboxRejectsMalformedEnvelope(t *testing.T) {
	pub := new(JetStreamPublisher)
	require.ErrorContains(t, pub.PublishExpiryEvent(context.Background(), []byte("invalid")), "outbox payload")
	event, err := NewExpiryEvent(uuid.NewString(), nil)
	require.NoError(t, err)
	event.EventId = "bad"
	raw, err := proto.Marshal(event)
	require.NoError(t, err)
	require.ErrorContains(t, pub.PublishExpiryEvent(context.Background(), raw), "event envelope")
	_, err = NewExpiryEvent(uuid.NewString(), proto.String("bad-chat"))
	require.Error(t, err)
}
