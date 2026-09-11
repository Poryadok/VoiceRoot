package spaceevents

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"

	"voice/backend/space/internal/outboxdelivery"
)

var _ outboxdelivery.Transport = (*JetStreamPublisher)(nil)

type preparedMessageJetStream struct {
	nats.JetStreamContext
	message *nats.Msg
	options []nats.PubOpt
	ack     *nats.PubAck
	err     error
}

func (c *preparedMessageJetStream) PublishMsg(message *nats.Msg, options ...nats.PubOpt) (*nats.PubAck, error) {
	c.message = message
	c.options = append([]nats.PubOpt(nil), options...)
	return c.ack, c.err
}

func preparedOutboxMessage() *nats.Msg {
	message := &nats.Msg{
		Subject: subjectSpaceUpdated,
		Data:    []byte{0x0a, 0x24, 0x38, 0x65, 0x63, 0x37, 0x35, 0x34},
		Header:  nats.Header{},
	}
	message.Header.Set(nats.MsgIdHdr, "8ec754c8-c06d-4a79-a72c-1315ef7ca573")
	return message
}

func clonePreparedHeader(header nats.Header) nats.Header {
	cloned := nats.Header{}
	for key, values := range header {
		cloned[key] = append([]string(nil), values...)
	}
	return cloned
}

func TestJetStreamPublisher_PublishPreparedPassesExactMessageAndContext(t *testing.T) {
	wantAck := &nats.PubAck{Stream: streamName, Sequence: 41}
	capture := &preparedMessageJetStream{ack: wantAck}
	publisher := &JetStreamPublisher{js: capture}
	publisher.ensureOnce.Do(func() {})
	message := preparedOutboxMessage()
	wantData := bytes.Clone(message.Data)
	wantHeader := clonePreparedHeader(message.Header)
	type contextKey string
	ctx := context.WithValue(context.Background(), contextKey("delivery"), "r21")

	ack, err := publisher.Publish(ctx, message)
	require.NoError(t, err)
	require.Same(t, wantAck, ack)
	require.Same(t, message, capture.message, "transport must publish the coordinator-prepared message itself")
	require.Equal(t, subjectSpaceUpdated, capture.message.Subject)
	require.Equal(t, wantData, capture.message.Data)
	require.Equal(t, wantHeader, capture.message.Header)
	require.Equal(t, nats.Header{nats.MsgIdHdr: []string{"8ec754c8-c06d-4a79-a72c-1315ef7ca573"}}, capture.message.Header)
	require.Len(t, capture.options, 1)
	contextOption, ok := capture.options[0].(nats.ContextOpt)
	require.True(t, ok, "PublishMsg must receive nats.Context")
	require.Equal(t, "r21", contextOption.Context.Value(contextKey("delivery")))
}

func TestJetStreamPublisher_PublishPreparedReturnsChatEventsServerAck(t *testing.T) {
	server := startJSTestServer(t)
	observer, err := nats.Connect(server.ClientURL())
	require.NoError(t, err)
	t.Cleanup(observer.Close)
	subscription, err := observer.SubscribeSync(subjectSpaceUpdated)
	require.NoError(t, err)
	require.NoError(t, observer.Flush())

	publisher, err := NewJetStreamPublisher(server.ClientURL())
	require.NoError(t, err)
	t.Cleanup(func() { _ = publisher.Close() })
	message := preparedOutboxMessage()
	wantData := bytes.Clone(message.Data)
	wantHeader := clonePreparedHeader(message.Header)

	ack, err := publisher.Publish(context.Background(), message)
	require.NoError(t, err)
	require.NotNil(t, ack)
	require.Equal(t, streamName, ack.Stream)
	require.NotZero(t, ack.Sequence)

	received, err := subscription.NextMsg(5 * time.Second)
	require.NoError(t, err)
	require.Equal(t, subjectSpaceUpdated, received.Subject)
	require.Equal(t, wantData, received.Data)
	require.Equal(t, wantHeader, received.Header)
}

func TestJetStreamPublisher_PublishPreparedPreservesFailureAndCancellation(t *testing.T) {
	publishErr := errors.New("jetstream unavailable")
	capture := &preparedMessageJetStream{err: publishErr}
	publisher := &JetStreamPublisher{js: capture}
	publisher.ensureOnce.Do(func() {})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := publisher.Publish(ctx, preparedOutboxMessage())
	require.ErrorIs(t, err, publishErr)
	require.Len(t, capture.options, 1)
	contextOption, ok := capture.options[0].(nats.ContextOpt)
	require.True(t, ok)
	require.ErrorIs(t, contextOption.Context.Err(), context.Canceled)
}
