package messageevents

import (
	"context"
	"log/slog"
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
	require.NoError(t, pub.PublishMessageSent(ctx, mid, cid, sid, false))

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

	require.NoError(t, pub.PublishMessageSent(ctx, "11111111-1111-1111-1111-111111111111", "22222222-2222-2222-2222-222222222222", "33333333-3333-3333-3333-333333333333", true))

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

func TestJetStreamPublisher_ReactionAddedAndRemoved(t *testing.T) {
	ctx := context.Background()
	s := startJSTestServer(t)
	url := s.ClientURL()

	nc, err := nats.Connect(url)
	require.NoError(t, err)
	t.Cleanup(nc.Close)

	subAdd, err := nc.SubscribeSync(subjectReactionAdded)
	require.NoError(t, err)
	t.Cleanup(func() { _ = subAdd.Unsubscribe() })
	subRem, err := nc.SubscribeSync(subjectReactionRemoved)
	require.NoError(t, err)
	t.Cleanup(func() { _ = subRem.Unsubscribe() })

	pub, err := NewJetStreamPublisher(url)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pub.Close() })

	const mid, cid, pid, authorID, emoji = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", "cccccccc-cccc-cccc-cccc-cccccccccccc", "dddddddd-dddd-dddd-dddd-dddddddddddd", "👍"
	require.NoError(t, pub.PublishReactionAdded(ctx, mid, cid, pid, authorID, emoji))
	require.NoError(t, pub.PublishReactionRemoved(ctx, mid, cid, pid, emoji))

	am, err := subAdd.NextMsg(3 * time.Second)
	require.NoError(t, err)
	var added eventsv1.MessageStreamEvent
	require.NoError(t, proto.Unmarshal(am.Data, &added))
	ra := added.GetReactionAdded()
	require.NotNil(t, ra)
	require.Equal(t, mid, ra.GetMessageId())
	require.Equal(t, cid, ra.GetChatId())
	require.Equal(t, pid, ra.GetProfileId())
	require.Equal(t, authorID, ra.GetMessageAuthorProfileId())
	require.Equal(t, emoji, ra.GetEmoji())

	rm, err := subRem.NextMsg(3 * time.Second)
	require.NoError(t, err)
	var removed eventsv1.MessageStreamEvent
	require.NoError(t, proto.Unmarshal(rm.Data, &removed))
	rr := removed.GetReactionRemoved()
	require.NotNil(t, rr)
	require.Equal(t, mid, rr.GetMessageId())
	require.Equal(t, cid, rr.GetChatId())
	require.Equal(t, pid, rr.GetProfileId())
	require.Equal(t, emoji, rr.GetEmoji())
}

func TestJetStreamPublisher_PublishPinEvents(t *testing.T) {
	ctx := context.Background()
	s := startJSTestServer(t)
	url := s.ClientURL()
	nc, err := nats.Connect(url)
	require.NoError(t, err)
	t.Cleanup(nc.Close)

	subPin, err := nc.SubscribeSync(subjectMessagePinned)
	require.NoError(t, err)
	t.Cleanup(func() { _ = subPin.Unsubscribe() })
	subUnpin, err := nc.SubscribeSync(subjectMessageUnpinned)
	require.NoError(t, err)
	t.Cleanup(func() { _ = subUnpin.Unsubscribe() })

	pub, err := NewJetStreamPublisher(url)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pub.Close() })

	const mid, cid, pid = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", "cccccccc-cccc-cccc-cccc-cccccccccccc"
	require.NoError(t, pub.PublishMessagePinned(ctx, mid, cid, pid))
	require.NoError(t, pub.PublishMessageUnpinned(ctx, mid, cid, pid))

	pm, err := subPin.NextMsg(3 * time.Second)
	require.NoError(t, err)
	var pinned eventsv1.MessageStreamEvent
	require.NoError(t, proto.Unmarshal(pm.Data, &pinned))
	pp := pinned.GetMessagePinned()
	require.NotNil(t, pp)
	require.Equal(t, mid, pp.GetMessageId())
	require.Equal(t, cid, pp.GetChatId())
	require.Equal(t, pid, pp.GetPinnedBy())

	um, err := subUnpin.NextMsg(3 * time.Second)
	require.NoError(t, err)
	var unpinned eventsv1.MessageStreamEvent
	require.NoError(t, proto.Unmarshal(um.Data, &unpinned))
	up := unpinned.GetMessageUnpinned()
	require.NotNil(t, up)
	require.Equal(t, mid, up.GetMessageId())
}

func TestNewJetStreamPublisher_emptyURL(t *testing.T) {
	t.Parallel()
	_, err := NewJetStreamPublisher("")
	require.Error(t, err)
}

func TestJetStreamPublisher_ensureStreamUninitialized(t *testing.T) {
	t.Parallel()
	var p JetStreamPublisher
	require.Error(t, p.EnsureStream())
}

func TestJetStreamPublisher_EnsureStream(t *testing.T) {
	s := startJSTestServer(t)
	pub, err := NewJetStreamPublisher(s.ClientURL())
	require.NoError(t, err)
	t.Cleanup(func() { _ = pub.Close() })
	require.NoError(t, pub.EnsureStream())
	require.NoError(t, pub.EnsureStream())
}

func slogAttrString(attrs []slog.Attr, key string) string {
	for _, a := range attrs {
		if a.Key == key {
			return a.Value.String()
		}
	}
	return ""
}

func TestMessageEventLogAttrs(t *testing.T) {
	t.Parallel()
	require.Nil(t, messageEventLogAttrs(nil))

	sent := messageEventLogAttrs(&eventsv1.MessageStreamEvent{
		EventId: "e1",
		Payload: &eventsv1.MessageStreamEvent_MessageSent{MessageSent: &eventsv1.MessageSent{
			MessageId: "m", ChatId: "c",
		}},
	})
	require.Equal(t, "e1", slogAttrString(sent, "event_id"))
	require.Equal(t, "m", slogAttrString(sent, "message_id"))
	require.Equal(t, "c", slogAttrString(sent, "chat_id"))

	edited := messageEventLogAttrs(&eventsv1.MessageStreamEvent{
		EventId: "e2",
		Payload: &eventsv1.MessageStreamEvent_MessageEdited{MessageEdited: &eventsv1.MessageEdited{
			MessageId: "m2", ChatId: "c2",
		}},
	})
	require.Equal(t, "m2", slogAttrString(edited, "message_id"))
	require.Equal(t, "c2", slogAttrString(edited, "chat_id"))

	deleted := messageEventLogAttrs(&eventsv1.MessageStreamEvent{
		EventId: "e3",
		Payload: &eventsv1.MessageStreamEvent_MessageDeleted{MessageDeleted: &eventsv1.MessageDeleted{
			MessageId: "m3", ChatId: "c3",
		}},
	})
	require.Equal(t, "m3", slogAttrString(deleted, "message_id"))

	read := messageEventLogAttrs(&eventsv1.MessageStreamEvent{
		EventId: "e4",
		Payload: &eventsv1.MessageStreamEvent_MessageRead{MessageRead: &eventsv1.MessageRead{
			MessageId: "m4", ChatId: "c4", ProfileId: "p4",
		}},
	})
	require.Equal(t, "m4", slogAttrString(read, "message_id"))
	require.Equal(t, "c4", slogAttrString(read, "chat_id"))
	require.Equal(t, "p4", slogAttrString(read, "profile_id"))
}

func TestJetStreamPublisher_ensureStreamAlreadyExists(t *testing.T) {
	s := startJSTestServer(t)
	pub, err := NewJetStreamPublisher(s.ClientURL())
	require.NoError(t, err)
	t.Cleanup(func() { _ = pub.Close() })
	require.NoError(t, pub.PublishMessageSent(context.Background(), "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", "cccccccc-cccc-cccc-cccc-cccccccccccc", false))
	require.NoError(t, pub.EnsureStream())
}

func TestJetStreamPublisher_MessageRead(t *testing.T) {
	ctx := context.Background()
	s := startJSTestServer(t)
	url := s.ClientURL()

	nc, err := nats.Connect(url)
	require.NoError(t, err)
	t.Cleanup(nc.Close)

	sub, err := nc.SubscribeSync(subjectMessageRead)
	require.NoError(t, err)
	t.Cleanup(func() { _ = sub.Unsubscribe() })

	pub, err := NewJetStreamPublisher(url)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pub.Close() })

	const mid, cid, pid = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", "cccccccc-cccc-cccc-cccc-cccccccccccc"
	require.NoError(t, pub.PublishMessageRead(ctx, mid, cid, pid))

	msg, err := sub.NextMsg(3 * time.Second)
	require.NoError(t, err)
	var env eventsv1.MessageStreamEvent
	require.NoError(t, proto.Unmarshal(msg.Data, &env))
	read := env.GetMessageRead()
	require.NotNil(t, read)
	require.Equal(t, mid, read.GetMessageId())
	require.Equal(t, cid, read.GetChatId())
	require.Equal(t, pid, read.GetProfileId())
}
