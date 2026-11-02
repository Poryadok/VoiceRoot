package store

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	eventsv1 "voice.app/voice/events/v1"
	"voice/backend/social/internal/socialevents"
)

// A committed block must retain revocation even when the process exits before
// opening any publisher.
func TestBlockCommitRetainsRevocationBeforePublish(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startStorePostgres(t, ctx)
	applyStoreMigrations(t, ctx, pool)
	s := &BlockStore{Pool: pool}
	blocker, blocked := uuid.New(), uuid.New()
	require.NoError(t, s.BlockAccount(ctx, blocker, blocked))
	exists, err := s.DirectedBlockExists(ctx, blocker, blocked)
	require.NoError(t, err)
	require.True(t, exists)
	var pending int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM block_event_outbox WHERE delivered_at IS NULL`).Scan(&pending))
	require.Equal(t, 1, pending, "successful block must retain durable revocation without a publisher or another RPC")
}

func TestBlockOutboxBrokerRestartDeliversToBothConsumers(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startStorePostgres(t, ctx)
	applyStoreMigrations(t, ctx, pool)
	opts := &server.Options{Host: "127.0.0.1", Port: -1, NoLog: true, NoSigs: true, JetStream: true, StoreDir: t.TempDir()}
	start := func() *server.Server {
		s, err := server.NewServer(opts)
		require.NoError(t, err)
		go s.Start()
		require.True(t, s.ReadyForConnections(5*time.Second))
		t.Cleanup(func() { s.Shutdown(); s.WaitForShutdown() })
		return s
	}
	broker := start()
	opts.Port = broker.Addr().(*net.TCPAddr).Port
	nc, err := nats.Connect(broker.ClientURL(), nats.MaxReconnects(-1), nats.ReconnectWait(10*time.Millisecond))
	require.NoError(t, err)
	t.Cleanup(nc.Close)
	js, err := nc.JetStream()
	require.NoError(t, err)
	_, err = js.AddStream(&nats.StreamConfig{Name: "social_events", Subjects: []string{"social.user_blocked"}, Storage: nats.FileStorage})
	require.NoError(t, err)
	var consumers []*nats.Subscription
	for _, instance := range []string{"realtime-a", "realtime-b"} {
		sub, err := js.PullSubscribe("social.user_blocked", instance, nats.BindStream("social_events"), nats.AckExplicit())
		require.NoError(t, err)
		consumers = append(consumers, sub)
	}
	publisher, err := socialevents.NewJetStreamPublisher(broker.ClientURL())
	require.NoError(t, err)
	t.Cleanup(func() { _ = publisher.Close() })
	blocks := &BlockStore{Pool: pool}
	blocker, blocked := uuid.New(), uuid.New()
	require.NoError(t, blocks.BlockAccount(ctx, blocker, blocked))
	var savedID uuid.UUID
	var savedBytes []byte
	require.NoError(t, pool.QueryRow(ctx, `SELECT event_id,payload FROM block_event_outbox`).Scan(&savedID, &savedBytes))
	broker.Shutdown()
	broker.WaitForShutdown()
	failed, cancel := context.WithTimeout(ctx, 250*time.Millisecond)
	n, err := blocks.DispatchBlockEventOutbox(failed, 1, publisher.PublishBlockOutboxEvent)
	cancel()
	require.Error(t, err)
	require.Zero(t, n)
	// A fresh process/store retries committed intent after the broker returns.
	start()
	require.Eventually(t, func() bool { return nc.FlushTimeout(100*time.Millisecond) == nil }, 5*time.Second, 20*time.Millisecond)
	restarted := &BlockStore{Pool: pool}
	require.Eventually(t, func() bool {
		n, err = restarted.DispatchBlockEventOutbox(ctx, 1, publisher.PublishBlockOutboxEvent)
		return err == nil && n == 1
	}, 5*time.Second, 20*time.Millisecond)
	for _, consumer := range consumers {
		messages, err := consumer.Fetch(1, nats.MaxWait(3*time.Second))
		require.NoError(t, err)
		require.Len(t, messages, 1)
		require.Equal(t, savedBytes, messages[0].Data)
		require.Equal(t, savedID.String(), messages[0].Header.Get(nats.MsgIdHdr))
		var env eventsv1.SocialStreamEvent
		require.NoError(t, proto.Unmarshal(messages[0].Data, &env))
		require.Equal(t, blocker.String(), env.GetUserBlocked().GetBlockerAccountId())
		require.Equal(t, blocked.String(), env.GetUserBlocked().GetBlockedAccountId())
		require.NoError(t, messages[0].AckSync())
	}
	n, err = restarted.DispatchBlockEventOutbox(ctx, 1, func(context.Context, uuid.UUID, []byte) error { t.Fatal("delivered event replayed"); return nil })
	require.NoError(t, err)
	require.Zero(t, n)
}

func TestBlockOutboxUncertainPublishReplaysImmutableIntent(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startStorePostgres(t, ctx)
	applyStoreMigrations(t, ctx, pool)
	blocks := &BlockStore{Pool: pool}
	blocker, blocked := uuid.New(), uuid.New()
	require.NoError(t, blocks.BlockAccount(ctx, blocker, blocked))
	var firstID uuid.UUID
	var firstBytes []byte
	unknown := errors.New("broker accepted but PubAck unknown")
	_, err := blocks.DispatchBlockEventOutbox(ctx, 1, func(_ context.Context, id uuid.UUID, payload []byte) error {
		firstID = id
		firstBytes = append([]byte(nil), payload...)
		return unknown
	})
	require.ErrorIs(t, err, unknown)
	// Unblocking must not cascade-delete undelivered historical revocation.
	require.NoError(t, blocks.UnblockAccount(ctx, blocker, blocked))
	n, err := (&BlockStore{Pool: pool}).DispatchBlockEventOutbox(ctx, 1, func(_ context.Context, id uuid.UUID, payload []byte) error {
		require.Equal(t, firstID, id)
		require.Equal(t, firstBytes, payload)
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, 1, n)

	t.Run("publish succeeds but commit is canceled", func(t *testing.T) {
		require.NoError(t, blocks.BlockAccount(ctx, uuid.New(), uuid.New()))
		dispatch, cancel := context.WithCancel(ctx)
		defer cancel()
		_, err := blocks.DispatchBlockEventOutbox(dispatch, 1, func(_ context.Context, id uuid.UUID, payload []byte) error {
			firstID, firstBytes = id, append([]byte(nil), payload...)
			cancel()
			return nil
		})
		require.Error(t, err)
		n, err := (&BlockStore{Pool: pool}).DispatchBlockEventOutbox(ctx, 1, func(_ context.Context, id uuid.UUID, payload []byte) error {
			require.Equal(t, firstID, id)
			require.Equal(t, firstBytes, payload)
			return nil
		})
		require.NoError(t, err)
		require.Equal(t, 1, n)
	})

	t.Run("concurrent worker skips in-flight intent", func(t *testing.T) {
		require.NoError(t, blocks.BlockAccount(ctx, uuid.New(), uuid.New()))
		started, release := make(chan struct{}), make(chan struct{})
		result := make(chan error, 1)
		go func() {
			_, err := blocks.DispatchBlockEventOutbox(ctx, 1, func(context.Context, uuid.UUID, []byte) error {
				close(started)
				<-release
				return unknown
			})
			result <- err
		}()
		defer close(release)
		select {
		case <-started:
		case <-time.After(5 * time.Second):
			t.Fatal("first worker did not claim intent")
		}
		n, err := (&BlockStore{Pool: pool}).DispatchBlockEventOutbox(ctx, 1, func(context.Context, uuid.UUID, []byte) error {
			t.Fatal("second worker must not publish a locked intent")
			return nil
		})
		require.NoError(t, err)
		require.Zero(t, n)
		// Release at function exit; ensure the worker finishes before pool cleanup.
		t.Cleanup(func() { require.ErrorIs(t, <-result, unknown) })
	})
}

func TestBlockOutboxEnqueueFailureRollsBackBlockAndFriendshipCleanup(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startStorePostgres(t, ctx)
	applyStoreMigrations(t, ctx, pool)
	_, err := pool.Exec(ctx, `ALTER TABLE block_event_outbox ADD CONSTRAINT reject_event CHECK(false)`)
	require.NoError(t, err)
	a, b, pa, pb := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	friends := &FriendshipStore{Pool: pool}
	require.NoError(t, friends.SendInvitation(ctx, pa, pb))
	blocks := &BlockStore{Pool: pool}
	require.Error(t, blocks.BlockAccountAndSeverFriendships(ctx, a, b, []uuid.UUID{pa}, []uuid.UUID{pb}))
	exists, err := blocks.DirectedBlockExists(ctx, a, b)
	require.NoError(t, err)
	require.False(t, exists)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM friendships`).Scan(&count))
	require.Equal(t, 1, count, "failed durable enqueue must roll back the friendship cleanup too")
}
