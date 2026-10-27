package timeout

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
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
