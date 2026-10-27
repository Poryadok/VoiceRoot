package store

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/matchmaking/internal/terminalrouting"
)

func TestTerminalRoutingDeadlineErasureDoesNotWaitForRedisCallback(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	pool := StartMatchmakingDBForStoreTest(t, ctx)
	ApplyMatchmakingMigrationsForStoreTest(t, ctx, pool)
	sessionID, otherID := uuid.New(), uuid.New()
	_, err := pool.Exec(ctx, `INSERT INTO matchmaking_terminal_search_releases
		(search_session_id,generation,profile_id,game_id,mode,region,created_at)
		VALUES($1,3,$2,$3,'duo','eu',clock_timestamp()-interval '1 minute'),
		      ($4,3,$5,$3,'duo','eu',clock_timestamp())`, sessionID, uuid.New(), uuid.New(), otherID, uuid.New())
	require.NoError(t, err)
	entered := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)
	matches := &MatchStore{Pool: pool}
	go func() {
		done <- matches.ApplyPendingTerminalSearchReleases(ctx, 1, func(projectionCtx context.Context, effect TerminalSearchRelease) error {
			if effect.SessionID != sessionID {
				return context.Canceled
			}
			close(entered)
			select {
			case <-release:
				return nil
			case <-projectionCtx.Done():
				return projectionCtx.Err()
			}
		})
	}()
	defer func() {
		close(release)
		require.NoError(t, <-done)
	}()
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal("terminal projector did not reach the controlled Redis callback")
	}
	// This is the local deadline's SQL erasure, deliberately independent of the
	// unavailable/in-flight Redis command. No timeout is inflated to let Redis win.
	eraser, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = eraser.Rollback(ctx) }()
	_, err = eraser.Exec(ctx, `SET LOCAL lock_timeout='250ms'`)
	require.NoError(t, err)
	_, err = eraser.Exec(ctx, `DELETE FROM matchmaking_terminal_search_releases WHERE search_session_id=$1 AND generation=3`, sessionID)
	require.NoError(t, err, "deadline SQL erasure must not wait for an in-flight Redis callback")
	require.NoError(t, eraser.Commit(ctx))
	var otherCount int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM matchmaking_terminal_search_releases WHERE search_session_id=$1`, otherID).Scan(&otherCount))
	require.Equal(t, 1, otherCount)
}

func TestTerminalRoutingExpiryIgnoresPausedProjectionClaim(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	pool := StartMatchmakingDBForStoreTest(t, ctx)
	ApplyMatchmakingMigrationsForStoreTest(t, ctx, pool)
	var now time.Time
	require.NoError(t, pool.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&now))
	deadline := now.Add(250 * time.Millisecond)
	schedule := terminalrouting.Schedule{FenceID: "paused-fence", CycleID: "paused-cycle", Revision: 41,
		ScheduledAt: deadline.Add(-30 * 24 * time.Hour), PurgeAt: deadline}
	s := &MatchStore{Pool: pool}
	_, err := s.ObserveTerminalRouting(ctx, terminalrouting.Observation{Schedule: schedule,
		Current: terminalrouting.Current{Revision: 41, State: terminalrouting.DeleteScheduled, CycleID: schedule.CycleID}})
	require.NoError(t, err)
	profile, session := uuid.New(), uuid.New()
	_, err = pool.Exec(ctx, `INSERT INTO matchmaking_terminal_search_releases
		(search_session_id,generation,profile_id,game_id,mode,region) VALUES($1,3,$2,$3,'duo','eu')`, session, profile, uuid.New())
	require.NoError(t, err)
	coverage := terminalrouting.Coverage{Schedule: schedule, Revision: 17, Complete: true, Profiles: []uuid.UUID{profile}}
	coverage.SHA256, err = coverage.Digest()
	require.NoError(t, err)
	require.NoError(t, s.BindTerminalRoutingCoverage(ctx, coverage))
	entered, release := make(chan struct{}), make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- s.ApplyPendingTerminalSearchReleases(ctx, 1, func(_ context.Context, effect TerminalSearchRelease) error {
			if effect.RoutingLifecycle == nil {
				return terminalrouting.ErrAuthorityInvalid
			}
			close(entered)
			<-release
			return nil
		})
	}()
	defer func() { close(release); require.NoError(t, <-done) }()
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal("projection barrier not reached")
	}
	require.Eventually(t, func() bool {
		var due bool
		err := pool.QueryRow(ctx, `SELECT clock_timestamp()>=$1`, deadline).Scan(&due)
		return err == nil && due
	}, time.Second, 10*time.Millisecond, "owning database clock must reach the original deadline")
	require.NoError(t, s.ExpireTerminalRouting(ctx, 100))
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM matchmaking_terminal_search_releases WHERE search_session_id=$1`, session).Scan(&count))
	require.Zero(t, count)
	var erased bool
	var digest []byte
	require.NoError(t, pool.QueryRow(ctx, `SELECT routing_erased,coverage_sha256 FROM matchmaking_terminal_routing_cycles WHERE fence_id=$1 AND cycle_id=$2`, schedule.FenceID, schedule.CycleID).Scan(&erased, &digest))
	require.True(t, erased)
	require.Empty(t, digest)
}

func TestRestoredRoutingCycleCannotStarveAnotherDueCycle(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	pool := StartMatchmakingDBForStoreTest(t, ctx)
	ApplyMatchmakingMigrationsForStoreTest(t, ctx, pool)
	schedule := terminalrouting.Schedule{FenceID: "a-restored", CycleID: "cycle", Revision: 41,
		ScheduledAt: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
		PurgeAt:     time.Date(2026, time.January, 31, 0, 0, 0, 0, time.UTC)}
	life, err := terminalrouting.New(schedule)
	require.NoError(t, err)
	life, err = life.Reconcile(terminalrouting.Observation{Schedule: schedule,
		Decision: &terminalrouting.Decision{Kind: terminalrouting.Restored, Revision: 42, At: schedule.PurgeAt.Add(-time.Second)},
		Current:  terminalrouting.Current{Revision: 42, State: terminalrouting.Active}}, schedule.PurgeAt.Add(-time.Second))
	require.NoError(t, err)
	raw, err := json.Marshal(life)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO matchmaking_terminal_routing_cycles(fence_id,cycle_id,purge_at,lifecycle_json,routing_erased)
		VALUES($1,$2,$3,$4,false)`, schedule.FenceID, schedule.CycleID, schedule.PurgeAt, raw)
	require.NoError(t, err)
	schedule.FenceID = "b-pending"
	life, err = terminalrouting.New(schedule)
	require.NoError(t, err)
	raw, err = json.Marshal(life)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO matchmaking_terminal_routing_cycles(fence_id,cycle_id,purge_at,lifecycle_json,routing_erased)
		VALUES($1,$2,$3,$4,false)`, schedule.FenceID, schedule.CycleID, schedule.PurgeAt, raw)
	require.NoError(t, err)
	s := &MatchStore{Pool: pool}
	require.NoError(t, s.ExpireTerminalRouting(ctx, 1))
	require.NoError(t, s.ExpireTerminalRouting(ctx, 1))
	var erased bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT routing_erased FROM matchmaking_terminal_routing_cycles WHERE fence_id='b-pending'`).Scan(&erased))
	require.True(t, erased, "a restored cycle with no payload must not occupy every bounded expiry batch")
}

func TestTerminalRoutingLateCoverageErasesDurablyWithoutRedis(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	pool := StartMatchmakingDBForStoreTest(t, ctx)
	ApplyMatchmakingMigrationsForStoreTest(t, ctx, pool)
	profile := uuid.MustParse("00000000-0000-7000-8000-000000000001")
	session, unrelated := uuid.New(), uuid.New()
	schedule := terminalrouting.Schedule{
		FenceID: "fixture-auth-lifetime-fence-a", CycleID: "fixture-deletion-cycle-a1", Revision: 41,
		ScheduledAt: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
		PurgeAt:     time.Date(2026, time.January, 31, 0, 0, 0, 0, time.UTC),
	}
	_, err := pool.Exec(ctx, `INSERT INTO matchmaking_terminal_search_releases
		(search_session_id,generation,profile_id,game_id,mode,region,created_at)
		VALUES($1,3,$2,$3,'duo','eu',$4),($5,3,$6,$3,'duo','eu',clock_timestamp())`,
		session, profile, uuid.New(), schedule.ScheduledAt.Add(time.Hour), unrelated, uuid.New())
	require.NoError(t, err)
	s := &MatchStore{Pool: pool}
	life, err := s.ObserveTerminalRouting(ctx, terminalrouting.Observation{
		Schedule: schedule, Current: terminalrouting.Current{Revision: 41, State: terminalrouting.DeleteScheduled, CycleID: schedule.CycleID},
	})
	require.NoError(t, err)
	require.True(t, life.RoutingErased, "DB time is already past the original deadline, not receipt plus thirty days")
	coverage := terminalrouting.Coverage{Schedule: schedule, Revision: 17, Complete: true, Profiles: []uuid.UUID{profile}}
	coverage.SHA256, err = coverage.Digest()
	require.NoError(t, err)
	err = s.BindTerminalRoutingCoverage(ctx, coverage)
	require.ErrorIs(t, err, terminalrouting.ErrRoutingRetired)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM matchmaking_terminal_search_releases WHERE search_session_id=$1`, session).Scan(&count))
	require.Zero(t, count)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM matchmaking_terminal_search_releases WHERE search_session_id=$1`, unrelated).Scan(&count))
	require.Equal(t, 1, count)
	var state []byte
	var digest []byte
	require.NoError(t, pool.QueryRow(ctx, `SELECT lifecycle_json,coverage_sha256 FROM matchmaking_terminal_routing_cycles WHERE fence_id=$1 AND cycle_id=$2`, schedule.FenceID, schedule.CycleID).Scan(&state, &digest))
	require.Empty(t, digest)
	require.False(t, bytes.Contains(state, []byte(profile.String())))
	observation := terminalrouting.Observation{Schedule: schedule,
		Decision: &terminalrouting.Decision{Kind: terminalrouting.Restored, Revision: 42, At: schedule.PurgeAt.Add(-time.Second)},
		Current:  terminalrouting.Current{Revision: 42, State: terminalrouting.Active},
	}
	life, err = s.ObserveTerminalRouting(ctx, observation)
	require.NoError(t, err)
	require.True(t, life.RoutingErased)
	require.True(t, life.FreshAdmission(42, true))
	newer := terminalrouting.Observation{Schedule: schedule, Current: terminalrouting.Current{
		Revision: 43, State: terminalrouting.DeleteScheduled, CycleID: "fixture-deletion-cycle-a2",
	}}
	_, err = s.ObserveTerminalRouting(ctx, newer)
	require.NoError(t, err)
	// A new consumer instance must retain the durable current revision, not
	// mistake replay of the old valid restore for fresh current authority.
	restarted := &MatchStore{Pool: pool}
	life, err = restarted.ObserveTerminalRouting(ctx, observation)
	require.NoError(t, err)
	require.EqualValues(t, 43, life.Current.Revision)
	require.False(t, life.FreshAdmission(42, true))
	require.False(t, life.FreshAdmission(43, true))
	require.Equal(t, schedule.PurgeAt, life.Schedule.PurgeAt)
}

func TestTerminalRoutingCoverageCannotGuessMissingOldCycleDecision(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	pool := StartMatchmakingDBForStoreTest(t, ctx)
	ApplyMatchmakingMigrationsForStoreTest(t, ctx, pool)
	var now time.Time
	require.NoError(t, pool.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&now))
	schedule := terminalrouting.Schedule{FenceID: "ambiguous", CycleID: "old-cycle", Revision: 41,
		ScheduledAt: now.Add(-time.Hour), PurgeAt: now.Add(-time.Hour).Add(30 * 24 * time.Hour)}
	s := &MatchStore{Pool: pool}
	_, err := s.ObserveTerminalRouting(ctx, terminalrouting.Observation{Schedule: schedule,
		Current: terminalrouting.Current{Revision: 41, State: terminalrouting.DeleteScheduled, CycleID: schedule.CycleID}})
	require.NoError(t, err)
	newer := schedule
	newer.CycleID, newer.Revision = "new-cycle", 43
	newer.ScheduledAt = now
	newer.PurgeAt = now.Add(30 * 24 * time.Hour)
	_, err = s.ObserveTerminalRouting(ctx, terminalrouting.Observation{Schedule: newer,
		Current: terminalrouting.Current{Revision: 43, State: terminalrouting.DeleteScheduled, CycleID: newer.CycleID}})
	require.NoError(t, err)
	profile, session := uuid.New(), uuid.New()
	_, err = pool.Exec(ctx, `INSERT INTO matchmaking_terminal_search_releases
		(search_session_id,generation,profile_id,game_id,mode,region) VALUES($1,3,$2,$3,'duo','eu')`, session, profile, uuid.New())
	require.NoError(t, err)
	coverage := terminalrouting.Coverage{Schedule: schedule, Revision: 17, Complete: true, Profiles: []uuid.UUID{profile}}
	coverage.SHA256, err = coverage.Digest()
	require.NoError(t, err)
	require.ErrorIs(t, s.BindTerminalRoutingCoverage(ctx, coverage), terminalrouting.ErrReconciliationRequired,
		"without the old cycle decision, original purge_at cannot distinguish fresh post-restore routing")
	var unbound bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT routing_fence_id IS NULL FROM matchmaking_terminal_search_releases WHERE search_session_id=$1`, session).Scan(&unbound))
	require.True(t, unbound)
}
