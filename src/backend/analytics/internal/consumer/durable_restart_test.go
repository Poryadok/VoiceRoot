package consumer

import (
	"context"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
)

func startAnalyticsJetStream(t *testing.T) *server.Server {
	t.Helper()
	srv, err := server.NewServer(&server.Options{Host: "127.0.0.1", Port: -1, NoLog: true, NoSigs: true, JetStream: true, StoreDir: t.TempDir()})
	require.NoError(t, err)
	go srv.Start()
	require.True(t, srv.ReadyForConnections(5*time.Second))
	t.Cleanup(srv.Shutdown)
	return srv
}

func TestExplicitAnalyticsDurableSurvivesPodRestartWithPendingMessage(t *testing.T) {
	srv := startAnalyticsJetStream(t)
	nc, err := nats.Connect(srv.ClientURL())
	require.NoError(t, err)
	t.Cleanup(nc.Close)
	js, err := nc.JetStream()
	require.NoError(t, err)
	_, err = js.AddStream(&nats.StreamConfig{Name: "events", Subjects: []string{"events"}})
	require.NoError(t, err)

	const durable, queue = "analytics_v2_test", "analytics_v2_test"
	provisionAnalyticsDurable(t, js, "events", "events", durable, queue)
	info, err := js.ConsumerInfo("events", durable)
	require.NoError(t, err)
	info.Config.AckWait = 50 * time.Millisecond
	_, err = js.UpdateConsumer("events", &info.Config)
	require.NoError(t, err)

	first, err := js.QueueSubscribeSync("events", queue, nats.Bind("events", durable), nats.ManualAck())
	require.NoError(t, err)
	require.NoError(t, nc.Flush())
	_, err = js.Publish("events", []byte("pending"))
	require.NoError(t, err)
	received, err := first.NextMsg(time.Second)
	require.NoError(t, err)
	require.Equal(t, []byte("pending"), received.Data)
	require.NoError(t, first.Unsubscribe())
	nc.Close()

	restarted, err := nats.Connect(srv.ClientURL())
	require.NoError(t, err)
	t.Cleanup(restarted.Close)
	restartedJS, err := restarted.JetStream()
	require.NoError(t, err)
	second, err := restartedJS.QueueSubscribeSync("events", queue, nats.Bind("events", durable), nats.ManualAck())
	require.NoError(t, err)
	replay, err := second.NextMsg(time.Second)
	require.NoError(t, err)
	require.Equal(t, []byte("pending"), replay.Data)
	require.NoError(t, replay.Ack())

	_, err = restartedJS.ConsumerInfo("events", durable)
	require.NoError(t, err, "bound subscriptions must not delete the shared durable")
}

func TestRunnerRefusesToSubscribeWithoutClickHousePersistence(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err := (&Runner{}).Start(ctx, "nats://127.0.0.1:1", "")
	require.ErrorContains(t, err, "missing buffer")
}
