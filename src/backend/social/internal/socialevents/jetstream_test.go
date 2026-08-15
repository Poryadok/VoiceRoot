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

	sub, err := nc.SubscribeSync(subjectFriendRequest)
	require.NoError(t, err)
	t.Cleanup(func() { _ = sub.Unsubscribe() })

	pub, err := NewJetStreamPublisher(url)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pub.Close() })

	const (
		requestID  = "req-1"
		requester  = "11111111-1111-4111-8111-111111111111"
		target     = "22222222-2222-4222-8222-222222222222"
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
