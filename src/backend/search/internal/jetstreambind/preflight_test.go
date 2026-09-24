package jetstreambind

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
)

func TestConnectUsesScopedRequestInbox(t *testing.T) {
	s := startJetStreamServer(t)
	nc, _, err := Connect(s.ClientURL(), "voice-search-test")
	require.NoError(t, err)
	t.Cleanup(nc.Close)
	require.True(t, strings.HasPrefix(nc.NewInbox(), "_INBOX.voice.search."))
}

func TestScopedSearchInboxSupportsJetStreamRepliesAndRejectsNeighbor(t *testing.T) {
	s, err := server.NewServer(&server.Options{
		Host: "127.0.0.1", Port: -1, NoLog: true, NoSigs: true,
		JetStream: true, StoreDir: t.TempDir(),
		Users: []*server.User{
			{Username: "bootstrap", Password: "bootstrap"},
			{Username: "search", Password: "search", Permissions: &server.Permissions{
				Publish:   &server.SubjectPermission{Allow: []string{"$JS.API.STREAM.INFO.search_events", "search.event"}},
				Subscribe: &server.SubjectPermission{Allow: []string{"_INBOX.voice.search.>"}},
			}},
		},
	})
	require.NoError(t, err)
	go s.Start()
	require.True(t, s.ReadyForConnections(5*time.Second))
	t.Cleanup(s.Shutdown)

	admin, err := nats.Connect(s.ClientURL(), nats.UserInfo("bootstrap", "bootstrap"))
	require.NoError(t, err)
	t.Cleanup(admin.Close)
	adminJS, err := admin.JetStream()
	require.NoError(t, err)
	_, err = adminJS.AddStream(&nats.StreamConfig{Name: "search_events", Subjects: []string{"search.event"}})
	require.NoError(t, err)

	permissionErrors := make(chan error, 1)
	scopedURL := strings.Replace(s.ClientURL(), "nats://", "nats://search:search@", 1)
	nc, _, err := Connect(scopedURL, "voice-search-scoped")
	require.NoError(t, err)
	t.Cleanup(nc.Close)
	nc.SetErrorHandler(func(_ *nats.Conn, _ *nats.Subscription, err error) {
		select {
		case permissionErrors <- err:
		default:
		}
	})
	js, err := nc.JetStream()
	require.NoError(t, err)
	_, err = js.StreamInfo("search_events")
	require.NoError(t, err)
	ack, err := js.Publish("search.event", []byte("event"))
	require.NoError(t, err)
	require.Equal(t, "search_events", ack.Stream)

	_, err = nc.SubscribeSync("_INBOX.voice.notification.probe")
	require.NoError(t, err)
	require.NoError(t, nc.Flush())
	select {
	case <-permissionErrors:
	case <-time.After(time.Second):
		t.Fatal("neighboring notification inbox subscription was not denied")
	}
}

func TestBindRejectsBroadenedDurableBeforeDelivery(t *testing.T) {
	s := startJetStreamServer(t)
	nc, err := nats.Connect(s.ClientURL())
	require.NoError(t, err)
	t.Cleanup(nc.Close)
	js, err := nc.JetStream()
	require.NoError(t, err)
	_, err = js.AddStream(&nats.StreamConfig{Name: "user_events", Subjects: []string{"user.>"}})
	require.NoError(t, err)
	_, err = js.Publish("user.unrelated", []byte("before-bind"))
	require.NoError(t, err)
	_, err = js.AddConsumer("user_events", &nats.ConsumerConfig{
		Durable:        "search-user-test",
		DeliverSubject: "_INBOX.voice.search.indexer.user",
		FilterSubject:  ">",
		DeliverPolicy:  nats.DeliverAllPolicy,
		AckPolicy:      nats.AckExplicitPolicy,
	})
	require.NoError(t, err)

	delivered := make(chan struct{}, 1)
	_, err = Bind(js, "user_events", "search-user-test", "user.>", "_INBOX.voice.search.indexer.user", func(*nats.Msg) { delivered <- struct{}{} })
	require.Error(t, err)
	select {
	case <-delivered:
		t.Fatal("broadened durable delivered before bind was rejected")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestWatchRejectsRecreatedBroadenedDurable(t *testing.T) {
	s := startJetStreamServer(t)
	nc, err := nats.Connect(s.ClientURL())
	require.NoError(t, err)
	t.Cleanup(nc.Close)
	js, err := nc.JetStream()
	require.NoError(t, err)
	_, err = js.AddStream(&nats.StreamConfig{Name: "user_events", Subjects: []string{"user.>"}})
	require.NoError(t, err)
	_, err = js.AddConsumer("user_events", &nats.ConsumerConfig{Durable: "search-user-test", DeliverSubject: "_INBOX.voice.search.indexer.user", FilterSubject: "user.>", DeliverPolicy: nats.DeliverNewPolicy, AckPolicy: nats.AckExplicitPolicy})
	require.NoError(t, err)

	err = js.DeleteConsumer("user_events", "search-user-test")
	require.NoError(t, err)
	_, err = js.AddConsumer("user_events", &nats.ConsumerConfig{Durable: "search-user-test", DeliverSubject: "_INBOX.voice.search.indexer.user", FilterSubject: ">", DeliverPolicy: nats.DeliverAllPolicy, AckPolicy: nats.AckExplicitPolicy})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err = watch(ctx, make(chan struct{}), js, "user_events", "search-user-test", "user.>", "_INBOX.voice.search.indexer.user", time.Millisecond)
	require.Error(t, err)
}

func startJetStreamServer(t *testing.T) *server.Server {
	t.Helper()
	s, err := server.NewServer(&server.Options{Host: "127.0.0.1", Port: -1, NoLog: true, NoSigs: true, JetStream: true, StoreDir: t.TempDir()})
	require.NoError(t, err)
	go s.Start()
	require.True(t, s.ReadyForConnections(5*time.Second))
	t.Cleanup(s.Shutdown)
	return s
}
