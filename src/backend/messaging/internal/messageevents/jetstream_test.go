package messageevents

import (
	"context"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"

	eventsv1 "voice.app/voice/events/v1"
	"voice/backend/pkg/correlation"
)

func startJSTestServer(t *testing.T) *server.Server {
	t.Helper()
	opts := &server.Options{
		Host:      "127.0.0.1",
		Port:      -1,
		NoLog:     true,
		NoSigs:    true,
		JetStream: true,
		StoreDir:  t.TempDir(),
	}
	s, err := server.NewServer(opts)
	require.NoError(t, err)
	go s.Start()
	if !s.ReadyForConnections(5 * time.Second) {
		t.Fatal("nats server not ready")
	}
	t.Cleanup(func() { s.Shutdown() })
	return s
}

func TestJetStreamPublisher_MessageSentRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := startJSTestServer(t)
	url := s.ClientURL()

	nc, err := nats.Connect(url)
	require.NoError(t, err)
	t.Cleanup(nc.Close)

	sub, err := nc.SubscribeSync(subjectMessageSent)
	require.NoError(t, err)
	t.Cleanup(func() { _ = sub.Unsubscribe() })

	pub, err := NewJetStreamPublisher(url)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pub.Close() })

	const mid, cid, sid = "11111111-1111-1111-1111-111111111111", "22222222-2222-2222-2222-222222222222", "33333333-3333-3333-3333-333333333333"
	require.NoError(t, pub.PublishMessageSent(ctx, mid, cid, sid))

	msg, err := sub.NextMsg(3 * time.Second)
	require.NoError(t, err)
	var env eventsv1.MessageStreamEvent
	require.NoError(t, proto.Unmarshal(msg.Data, &env))
	require.NotEmpty(t, env.GetEventId())
	require.NotNil(t, env.GetOccurredAt())
	sent := env.GetMessageSent()
	require.NotNil(t, sent)
	require.Equal(t, mid, sent.GetMessageId())
	require.Equal(t, cid, sent.GetChatId())
	require.Equal(t, sid, sent.GetSenderProfileId())
}

func TestJetStreamPublisher_RequestIDHeader(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(correlation.GRPCMetadataKey, "req-header-test"))
	s := startJSTestServer(t)
	url := s.ClientURL()

	nc, err := nats.Connect(url)
	require.NoError(t, err)
	t.Cleanup(nc.Close)

	sub, err := nc.SubscribeSync(subjectMessageSent)
	require.NoError(t, err)
	t.Cleanup(func() { _ = sub.Unsubscribe() })

	pub, err := NewJetStreamPublisher(url)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pub.Close() })

	require.NoError(t, pub.PublishMessageSent(ctx, "11111111-1111-1111-1111-111111111111", "22222222-2222-2222-2222-222222222222", "33333333-3333-3333-3333-333333333333"))

	msg, err := sub.NextMsg(3 * time.Second)
	require.NoError(t, err)
	require.Equal(t, "req-header-test", msg.Header.Get(correlation.RequestIDHeader))
}

func TestJetStreamPublisher_MessageEditedAndDeleted(t *testing.T) {
	ctx := context.Background()
	s := startJSTestServer(t)
	url := s.ClientURL()

	nc, err := nats.Connect(url)
	require.NoError(t, err)
	t.Cleanup(nc.Close)

	subEd, err := nc.SubscribeSync(subjectMessageEdited)
	require.NoError(t, err)
	t.Cleanup(func() { _ = subEd.Unsubscribe() })
	subDel, err := nc.SubscribeSync(subjectMessageDeleted)
	require.NoError(t, err)
	t.Cleanup(func() { _ = subDel.Unsubscribe() })

	pub, err := NewJetStreamPublisher(url)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pub.Close() })

	const mid, cid = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	require.NoError(t, pub.PublishMessageEdited(ctx, mid, cid))
	require.NoError(t, pub.PublishMessageDeleted(ctx, mid, cid))

	em, err := subEd.NextMsg(3 * time.Second)
	require.NoError(t, err)
	var edited eventsv1.MessageStreamEvent
	require.NoError(t, proto.Unmarshal(em.Data, &edited))
	require.Equal(t, mid, edited.GetMessageEdited().GetMessageId())
	require.Equal(t, cid, edited.GetMessageEdited().GetChatId())

	dm, err := subDel.NextMsg(3 * time.Second)
	require.NoError(t, err)
	var deleted eventsv1.MessageStreamEvent
	require.NoError(t, proto.Unmarshal(dm.Data, &deleted))
	require.Equal(t, mid, deleted.GetMessageDeleted().GetMessageId())
	require.Equal(t, cid, deleted.GetMessageDeleted().GetChatId())
}
