package timeout

import (
	"context"
	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"voice/backend/matchmaking/internal/queue"
	"voice/backend/matchmaking/internal/store"
	"voice/backend/matchmaking/internal/terminalrouting"
)

func TestTerminalRoutingExpiryRunsWithoutRedisOrSessionProjection(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	pool := store.StartMatchmakingDBForStoreTest(t, ctx)
	store.ApplyMatchmakingMigrationsForStoreTest(t, ctx, pool)
	schedule := terminalrouting.Schedule{FenceID: "sweeper-fence", CycleID: "sweeper-cycle", Revision: 41,
		ScheduledAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), PurgeAt: time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)}
	life, err := terminalrouting.New(schedule)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO matchmaking_terminal_routing_cycles
 (fence_id,cycle_id,purge_at,lifecycle_json,routing_erased) VALUES($1,$2,$3,$4,false)`,
		schedule.FenceID, schedule.CycleID, schedule.PurgeAt, life)
	require.NoError(t, err)
	sweeper := &Sweeper{Matches: &store.MatchStore{Pool: pool}}
	require.NoError(t, sweeper.RunOnce(ctx))
	var erased bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT routing_erased FROM matchmaking_terminal_routing_cycles WHERE fence_id=$1`, schedule.FenceID).Scan(&erased))
	require.True(t, erased, "deadline erasure must precede the ordinary session/Redis availability guard")
}

func TestTerminalCoverageCannotQualifyAlreadyDispatchedLegacyProjection(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	pool := store.StartMatchmakingDBForStoreTest(t, ctx)
	store.ApplyMatchmakingMigrationsForStoreTest(t, ctx, pool)
	mr := miniredis.RunT(t)
	entered, resume := make(chan struct{}), make(chan struct{})
	client := redis.NewClient(&redis.Options{Addr: mr.Addr(), Dialer: func(c context.Context, network, address string) (net.Conn, error) {
		close(entered)
		select {
		case <-resume:
		case <-c.Done():
			return nil, c.Err()
		}
		return (&net.Dialer{}).DialContext(c, network, address)
	}})
	t.Cleanup(func() { _ = client.Close() })
	profile, session := uuid.New(), uuid.New()
	_, err := pool.Exec(ctx, `INSERT INTO matchmaking_terminal_search_releases(search_session_id,generation,profile_id,game_id,mode,region) VALUES($1,3,$2,$3,'duo','eu')`, session, profile, uuid.New())
	require.NoError(t, err)
	s := &store.MatchStore{Pool: pool}
	done := make(chan error, 1)
	go func() {
		done <- (&Sweeper{Matches: s, Sessions: &store.SessionStore{Pool: pool}, Queue: &queue.RedisQueue{Client: client, Prefix: "claimed-legacy"}}).RunOnce(ctx)
	}()
	defer func() { close(resume); require.NoError(t, <-done) }()
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal("legacy dispatch did not reach Redis barrier")
	}
	var now time.Time
	require.NoError(t, pool.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&now))
	schedule := terminalrouting.Schedule{FenceID: "legacy", CycleID: "cycle", Revision: 41, ScheduledAt: now.Add(-30*24*time.Hour + 250*time.Millisecond), PurgeAt: now.Add(250 * time.Millisecond)}
	_, err = s.ObserveTerminalRouting(ctx, terminalrouting.Observation{Schedule: schedule, Current: terminalrouting.Current{Revision: 41, State: terminalrouting.DeleteScheduled, CycleID: schedule.CycleID}})
	require.NoError(t, err)
	coverage := terminalrouting.Coverage{Schedule: schedule, Revision: 17, Complete: true, Profiles: []uuid.UUID{profile}}
	coverage.SHA256, err = coverage.Digest()
	require.NoError(t, err)
	err = s.BindTerminalRoutingCoverage(ctx, coverage)
	require.ErrorIs(t, err, terminalrouting.ErrReconciliationRequired, "coverage must not qualify a copied legacy dispatch that has no absolute Redis deadline")
}
