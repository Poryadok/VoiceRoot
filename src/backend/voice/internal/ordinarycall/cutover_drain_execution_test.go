package ordinarycall

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func TestOrdinaryCutoverExcludesGenericExecutionWithoutPretendingCleanup(t *testing.T) {
	pool := ordinaryExecutionPostgres(t)
	ctx := context.Background()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	ddl, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "..", "migrations", "voice_db", "000025_ordinary_cutover_drain.up.sql"))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(ddl))
	require.NoError(t, err)
	store := NewPostgresStore(pool, storageTestEncoder)
	media, projection := &mediaFake{}, &projectionFake{}
	coordinator := &Coordinator{Journal: store, Media: media, Projection: projection, Now: func() time.Time { return testNow }}
	pending, _, _ := pendingGroup(t)
	active, err := coordinator.Start(ctx, pending.Intent, nil)
	require.NoError(t, err)
	require.ErrorIs(t, store.CheckExecution(ctx, active.Intent.RoomID, uuid.New()), ErrConflict)
	owner := &CutoverDrainOwner{Store: store, Authority: cutoverAuthorityFixture{}}
	expected := CutoverExpectedRoom{WorkID: uuid.New(), RoomID: active.Intent.RoomID, Incarnation: active.Intent.OperationID, Version: active.Version}
	cut := ordinaryCutFixture()
	ref, err := owner.PrepareCutoverDrain(ctx, cut, expected)
	require.NoError(t, err)
	before, err := store.Load(ctx, active.Intent.RoomID)
	require.NoError(t, err)
	var beforeBytes []byte
	require.NoError(t, pool.QueryRow(ctx, `SELECT document FROM voice_ordinary_rooms WHERE room_id=$1`, active.Intent.RoomID).Scan(&beforeBytes))
	callback := false
	_, err = store.Mutate(ctx, active.Intent.RoomID, func(*Room) error { callback = true; return nil })
	require.ErrorIs(t, err, ErrCutoverOwned, "a stale ordinary callback cannot bypass native work ownership")
	require.False(t, callback)
	require.ErrorIs(t, store.MarkProjected(ctx, before.Intent.RoomID, before.Version), ErrCutoverOwned)
	media.closes, media.removed, projection.stages = 0, nil, nil
	require.ErrorIs(t, coordinator.Reconcile(ctx, before.Intent.RoomID), ErrCutoverOwned)
	require.ErrorIs(t, coordinator.project(ctx, active), ErrCutoverOwned, "a pre-capture snapshot cannot project after takeover")
	require.Zero(t, media.closes)
	require.Empty(t, media.removed)
	require.Empty(t, projection.stages)
	var afterBytes []byte
	require.NoError(t, pool.QueryRow(ctx, `SELECT document FROM voice_ordinary_rooms WHERE room_id=$1`, active.Intent.RoomID).Scan(&afterBytes))
	require.Equal(t, beforeBytes, afterBytes, "exclusion never manufactures SDK observations or native terminal progress")
	replay, err := owner.PrepareCutoverDrain(ctx, cut, expected)
	require.NoError(t, err)
	require.Equal(t, ref, replay)
	_, err = owner.ReadOwnerWork(ctx, ref)
	require.NoError(t, err, "protected owner continuation remains readable")

	// An ordinary same-realm tombstone without owner work must keep compensating.
	second, _, _ := pendingGroup(t)
	live, err := coordinator.Start(ctx, second.Intent, nil)
	require.NoError(t, err)
	_, err = coordinator.Change(ctx, live.Intent.RoomID, func(r *Room) error { return r.terminate(uuid.Nil, "normal_end", r.TransactionTime) })
	require.NoError(t, err)
	media.closes, projection.stages = 0, nil
	require.NoError(t, coordinator.Reconcile(ctx, live.Intent.RoomID))
	require.Equal(t, 1, media.closes)
	require.Len(t, projection.stages, 1)

	// Deterministic stale-worker schedules: takeover commits after the entry
	// check passed but before the branch dispatch check. Both old Closed and
	// pending-close branches, plus live membership reconciliation, must deny IO.
	for _, phase := range []string{"active", "closing", "closed"} {
		t.Run("takeover_after_entry_check_"+phase, func(t *testing.T) {
			pending, _, _ := pendingGroup(t)
			room, err := coordinator.Start(ctx, pending.Intent, nil)
			require.NoError(t, err)
			if phase != "active" {
				room, err = store.Mutate(ctx, room.Intent.RoomID, func(r *Room) error { return r.terminate(uuid.Nil, "normal_end", r.TransactionTime) })
				require.NoError(t, err)
			}
			if phase == "closed" {
				require.NoError(t, coordinator.Reconcile(ctx, room.Intent.RoomID))
				room, err = store.Load(ctx, room.Intent.RoomID)
				require.NoError(t, err)
			}
			media.closes, media.removed, media.listed, projection.stages = 0, nil, nil, nil
			guard := &cutoverDispatchJournal{Journal: store, afterFirstCheck: func() {
				_, e := owner.PrepareCutoverDrain(ctx, cut, CutoverExpectedRoom{WorkID: uuid.New(), RoomID: room.Intent.RoomID, Incarnation: room.Intent.OperationID, Version: room.Version})
				require.NoError(t, e)
			}}
			coordinator.Journal = guard
			err = coordinator.Reconcile(ctx, room.Intent.RoomID)
			coordinator.Journal = store
			require.ErrorIs(t, err, ErrCutoverOwned)
			require.GreaterOrEqual(t, guard.checks, 2)
			require.Zero(t, media.closes)
			require.Empty(t, media.removed)
			require.Empty(t, projection.stages)
		})
	}

	// Missing protected source is an error, never a permissive no-work fallback.
	_, err = pool.Exec(ctx, `ALTER TABLE voice_ordinary_cutover_works RENAME TO fixture_missing_cutover_works`)
	require.NoError(t, err)
	media.closes, projection.stages = 0, nil
	require.ErrorIs(t, coordinator.Reconcile(ctx, live.Intent.RoomID), ErrUnavailable)
	require.Zero(t, media.closes)
	require.Empty(t, projection.stages)
	_, err = pool.Exec(ctx, `ALTER TABLE fixture_missing_cutover_works RENAME TO voice_ordinary_cutover_works`)
	require.NoError(t, err)
}

type cutoverDispatchJournal struct {
	Journal
	checks          int
	afterFirstCheck func()
}

func (j *cutoverDispatchJournal) CheckExecution(ctx context.Context, room, incarnation uuid.UUID) error {
	err := j.Journal.CheckExecution(ctx, room, incarnation)
	j.checks++
	if err == nil && j.checks == 1 {
		j.afterFirstCheck()
	}
	return err
}

// The owned disposable runner supplies an internal-network DSN, without Docker
// socket access or published host ports. Ordinary CI retains its canonical
// testcontainer fixture when no explicit private runner is selected.
func ordinaryExecutionPostgres(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if testing.Short() {
		t.Skip("integration test skipped in short mode")
	}
	dsn := os.Getenv("ORDINARY_EXECUTION_TEST_DSN")
	if dsn == "" {
		return ordinaryPostgres(t)
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(admin.Close)
	database := "ordinary_exec_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	_, err = admin.Exec(ctx, `CREATE DATABASE `+database)
	require.NoError(t, err)
	config, err := pgxpool.ParseConfig(dsn)
	require.NoError(t, err)
	config.ConnConfig.Database = database
	pool, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	t.Cleanup(func() {
		pool.Close()
		_, e := admin.Exec(context.Background(), `DROP DATABASE `+database+` WITH (FORCE)`)
		require.NoError(t, e)
	})
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	dir := filepath.Join(filepath.Dir(file), "..", "..", "..", "migrations", "voice_db")
	for _, name := range []string{"000001_room_lifecycle", "000002_redis_divergence", "000003_matchmaking_membership", "000004_game_session_rooms", "000005_game_session_close", "000006_game_session_roster_lease", "000007_t17_sdk_conversion_fence", "000008_account_voice_fence", "000009_space_lifecycle", "000010_space_media_admission", "000011_match_squad_operations", "000012_ordinary_calls", "000013_a3_roster_outbox"} {
		body, e := os.ReadFile(filepath.Join(dir, name+".up.sql"))
		require.NoError(t, e)
		_, e = pool.Exec(ctx, string(body))
		require.NoError(t, e)
	}
	return pool
}
