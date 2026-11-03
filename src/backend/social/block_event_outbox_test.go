package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	"voice/backend/pkg/integrationtest"
	"voice/backend/social/internal/store"
)

type blockWorkerFailureLog struct{ failed chan struct{} }

func (h blockWorkerFailureLog) Enabled(context.Context, slog.Level) bool { return true }
func (h blockWorkerFailureLog) WithAttrs([]slog.Attr) slog.Handler       { return h }
func (h blockWorkerFailureLog) WithGroup(string) slog.Handler            { return h }
func (h blockWorkerFailureLog) Handle(_ context.Context, record slog.Record) error {
	if record.Level >= slog.LevelWarn {
		select {
		case h.failed <- struct{}{}:
		default:
		}
	}
	return nil
}

func TestBlockEventWorkerRecoversUnavailableBrokerWithoutAnotherRequest(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "social-block-worker", "")
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	paths, err := filepath.Glob(filepath.Join(filepath.Dir(file), "..", "..", "..", "src", "backend", "migrations", "social_db", "*.up.sql"))
	require.NoError(t, err)
	require.NotEmpty(t, paths)
	for _, path := range paths {
		migration, err := os.ReadFile(path)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(migration))
		require.NoError(t, err)
	}
	opts := &server.Options{Host: "127.0.0.1", Port: -1, NoLog: true, NoSigs: true, JetStream: true, StoreDir: t.TempDir()}
	start := func() *server.Server {
		broker, err := server.NewServer(opts)
		require.NoError(t, err)
		go broker.Start()
		require.True(t, broker.ReadyForConnections(5*time.Second))
		t.Cleanup(func() { broker.Shutdown(); broker.WaitForShutdown() })
		return broker
	}
	broker := start()
	url := broker.ClientURL()
	opts.Port = broker.Addr().(*net.TCPAddr).Port
	nc, err := nats.Connect(url, nats.MaxReconnects(-1), nats.ReconnectWait(10*time.Millisecond))
	require.NoError(t, err)
	t.Cleanup(nc.Close)
	js, err := nc.JetStream()
	require.NoError(t, err)
	_, err = js.AddStream(&nats.StreamConfig{Name: "social_events", Subjects: []string{"social.user_blocked"}, Storage: nats.FileStorage})
	require.NoError(t, err)
	var consumers []*nats.Subscription
	for _, name := range []string{"realtime-one", "realtime-two"} {
		sub, err := js.PullSubscribe("social.user_blocked", name, nats.BindStream("social_events"), nats.AckExplicit())
		require.NoError(t, err)
		consumers = append(consumers, sub)
	}
	broker.Shutdown()
	broker.WaitForShutdown()
	blocks := &store.BlockStore{Pool: pool}
	require.NoError(t, blocks.BlockAccount(ctx, uuid.New(), uuid.New()))
	workerCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	failures := make(chan struct{}, 1)
	go func() {
		defer close(done)
		runBlockEventOutbox(workerCtx, blocks, url, slog.New(blockWorkerFailureLog{failed: failures}))
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(15 * time.Second):
			t.Error("block outbox worker did not stop")
		}
	})
	// Wait for an actual failed autonomous attempt before restoring the broker.
	select {
	case <-failures:
	case <-time.After(15 * time.Second):
		t.Fatal("worker did not observe the unavailable broker")
	}
	var pending int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM block_event_outbox WHERE delivered_at IS NULL`).Scan(&pending))
	require.Equal(t, 1, pending)
	start()
	require.Eventually(t, func() bool { return nc.FlushTimeout(100*time.Millisecond) == nil }, 5*time.Second, 20*time.Millisecond)
	for _, consumer := range consumers {
		messages, err := consumer.Fetch(1, nats.MaxWait(12*time.Second))
		require.NoError(t, err)
		require.Len(t, messages, 1)
		require.NoError(t, messages[0].AckSync())
	}
	require.Eventually(t, func() bool {
		var delivered int
		return pool.QueryRow(ctx, `SELECT count(*) FROM block_event_outbox WHERE delivered_at IS NOT NULL`).Scan(&delivered) == nil && delivered == 1
	}, 3*time.Second, 20*time.Millisecond)
}
