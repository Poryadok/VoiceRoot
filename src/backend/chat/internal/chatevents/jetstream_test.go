package chatevents

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	eventsv1 "voice.app/voice/events/v1"
)

func TestJetStreamPublisherWorksWithChatScopedReplyInbox(t *testing.T) {
	s, err := server.NewServer(&server.Options{
		Host: "127.0.0.1", Port: -1, NoLog: true, NoSigs: true, JetStream: true, StoreDir: t.TempDir(),
		Users: []*server.User{
			{Username: "admin", Password: "admin"},
			{Username: "chat", Password: "chat", Permissions: &server.Permissions{
				Publish:   &server.SubjectPermission{Allow: []string{"$JS.API.STREAM.INFO.chat_events", subjectChatCreated}},
				Subscribe: &server.SubjectPermission{Allow: []string{"_INBOX.voice.chat.>"}},
			}},
		},
	})
	require.NoError(t, err)
	go s.Start()
	require.True(t, s.ReadyForConnections(5*time.Second))
	t.Cleanup(s.Shutdown)
	admin, err := nats.Connect(s.ClientURL(), nats.UserInfo("admin", "admin"))
	require.NoError(t, err)
	t.Cleanup(admin.Close)
	js, err := admin.JetStream()
	require.NoError(t, err)
	_, err = js.AddStream(&nats.StreamConfig{Name: streamName, Subjects: chatEventStreamSubjects(), Retention: nats.LimitsPolicy, MaxAge: 7 * 24 * time.Hour, Storage: nats.FileStorage})
	require.NoError(t, err)
	url := "nats://chat:chat@" + strings.TrimPrefix(s.ClientURL(), "nats://")
	pub, err := NewJetStreamPublisher(url)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pub.Close() })
	require.Equal(t, "_INBOX.voice.chat", pub.nc.Opts.InboxPrefix)
	require.NoError(t, pub.PublishChatCreated(context.Background(), "11111111-1111-1111-1111-111111111111", "dm"))
}

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
	nc, err := nats.Connect(s.ClientURL())
	require.NoError(t, err)
	js, err := nc.JetStream()
	require.NoError(t, err)
	_, err = js.AddStream(&nats.StreamConfig{Name: streamName, Subjects: chatEventStreamSubjects(), Retention: nats.LimitsPolicy, MaxAge: 7 * 24 * time.Hour, Storage: nats.FileStorage})
	require.NoError(t, err)
	nc.Close()
	t.Cleanup(func() { s.Shutdown() })
	return s
}

func TestJetStreamPublisher_ChatCreatedRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := startJSTestServer(t)
	url := s.ClientURL()

	nc, err := nats.Connect(url)
	require.NoError(t, err)
	t.Cleanup(nc.Close)

	sub, err := nc.SubscribeSync(subjectChatCreated)
	require.NoError(t, err)
	require.NoError(t, nc.Flush())
	t.Cleanup(func() { _ = sub.Unsubscribe() })

	pub, err := NewJetStreamPublisher(url)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pub.Close() })

	const cid = "11111111-1111-1111-1111-111111111111"
	require.NoError(t, pub.PublishChatCreated(ctx, cid, "dm"))

	msg, err := sub.NextMsg(3 * time.Second)
	require.NoError(t, err)
	var env eventsv1.ChatStreamEvent
	require.NoError(t, proto.Unmarshal(msg.Data, &env))
	require.NotEmpty(t, env.GetEventId())
	require.NotNil(t, env.GetOccurredAt())
	cc := env.GetChatCreated()
	require.NotNil(t, cc)
	require.Equal(t, cid, cc.GetChatId())
	require.Equal(t, "dm", cc.GetType())
}

func TestJetStreamPublisher_ChatMemberChangedRoundTrip(t *testing.T) {
	ctx := context.Background()
	srv := startJSTestServer(t)
	url := srv.ClientURL()

	nc, err := nats.Connect(url)
	require.NoError(t, err)
	t.Cleanup(nc.Close)

	sub, err := nc.SubscribeSync(subjectChatMemberChanged)
	require.NoError(t, err)
	require.NoError(t, nc.Flush())
	t.Cleanup(func() { _ = sub.Unsubscribe() })

	pub, err := NewJetStreamPublisher(url)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pub.Close() })

	const cid, pid = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	require.NoError(t, pub.PublishChatMemberChanged(ctx, cid, pid, "joined"))

	msg, err := sub.NextMsg(3 * time.Second)
	require.NoError(t, err)
	var env eventsv1.ChatStreamEvent
	require.NoError(t, proto.Unmarshal(msg.Data, &env))
	mc := env.GetChatMemberChanged()
	require.NotNil(t, mc)
	require.Equal(t, cid, mc.GetChatId())
	require.Equal(t, pid, mc.GetProfileId())
	require.Equal(t, "joined", mc.GetChange())
}
