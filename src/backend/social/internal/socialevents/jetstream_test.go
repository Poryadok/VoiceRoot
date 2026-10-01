package socialevents

import (
	"context"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	eventsv1 "voice.app/voice/events/v1"
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

func TestJetStreamPublisher_FriendRequestRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := startJSTestServer(t)
	url := s.ClientURL()

	nc, err := nats.Connect(url)
	require.NoError(t, err)
	t.Cleanup(nc.Close)
	js, err := nc.JetStream()
	require.NoError(t, err)
	_, err = js.AddStream(&nats.StreamConfig{Name: streamName, Subjects: []string{subjectFriendRequest}})
	require.NoError(t, err)

	sub, err := nc.SubscribeSync(subjectFriendRequest)
	require.NoError(t, err)
	t.Cleanup(func() { _ = sub.Unsubscribe() })

	pub, err := NewJetStreamPublisher(url)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pub.Close() })
	require.Regexp(t, `^_INBOX\.voice\.social\.`, pub.nc.NewRespInbox())

	const (
		requestID = "req-1"
		requester = "11111111-1111-4111-8111-111111111111"
		target    = "22222222-2222-4222-8222-222222222222"
	)
	require.NoError(t, pub.PublishFriendRequest(ctx, requestID, requester, target))

	msg, err := sub.NextMsg(3 * time.Second)
	require.NoError(t, err)
	var env eventsv1.SocialStreamEvent
	require.NoError(t, proto.Unmarshal(msg.Data, &env))
	fr := env.GetFriendRequest()
	require.NotNil(t, fr)
	require.Equal(t, requestID, fr.GetRequestId())
	require.Equal(t, requester, fr.GetRequesterProfileId())
	require.Equal(t, target, fr.GetTargetProfileId())
}

func TestJetStreamPublisher_FriendRequestRetryUsesStableMessageID(t *testing.T) {
	ctx := context.Background()
	s := startJSTestServer(t)
	url := s.ClientURL()

	nc, err := nats.Connect(url)
	require.NoError(t, err)
	t.Cleanup(nc.Close)
	js, err := nc.JetStream()
	require.NoError(t, err)
	_, err = js.AddStream(&nats.StreamConfig{Name: streamName, Subjects: []string{subjectFriendRequest}})
	require.NoError(t, err)
	sub, err := nc.SubscribeSync(subjectFriendRequest)
	require.NoError(t, err)
	t.Cleanup(func() { _ = sub.Unsubscribe() })

	pub, err := NewJetStreamPublisher(url)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pub.Close() })

	const (
		eventID   = "33333333-3333-4333-8333-333333333333"
		requestID = "11111111-1111-4111-8111-111111111111"
		requester = "22222222-2222-4222-8222-222222222222"
		target    = "44444444-4444-4444-8444-444444444444"
	)
	for range 2 {
		require.NoError(t, pub.PublishFriendRequestWithEventID(ctx, eventID, requestID, requester, target))
		msg, err := sub.NextMsg(3 * time.Second)
		require.NoError(t, err)
		require.Equal(t, eventID, msg.Header.Get(nats.MsgIdHdr))
		var env eventsv1.SocialStreamEvent
		require.NoError(t, proto.Unmarshal(msg.Data, &env))
		require.Equal(t, eventID, env.GetEventId())
		require.Equal(t, requestID, env.GetFriendRequest().GetRequestId())
	}

	info, err := js.StreamInfo(streamName)
	require.NoError(t, err)
	require.Equal(t, uint64(1), info.State.Msgs, "JetStream should retain only one event for a repeated Nats-Msg-Id")
}
