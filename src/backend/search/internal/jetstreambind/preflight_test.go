package jetstreambind

import (
	"context"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
)

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
