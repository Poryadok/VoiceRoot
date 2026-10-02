package store

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestSpaceLifecycleFenceSerializesSearchAdmissionAndAllowsCancellation(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := StartMatchmakingDBForStoreTest(t, ctx)
	ApplyMatchmakingMigrationsForStoreTest(t, ctx, pool)

	gameStore := &GameStore{Pool: pool}
	games, err := gameStore.List(ctx, ListGamesParams{PageSize: 1, Status: StatusActive})
	require.NoError(t, err)
	require.NotEmpty(t, games.Games)

	spaceID := uuid.New()
	sessions := &SessionStore{Pool: pool}
	params := CreateSessionParams{
		ProfileID: uuid.New(),
		GameID:    games.Games[0].ID,
		Mode:      "5v5 Ranked",
		Criteria:  `{"region":"eu","self":{"role":"Carry","rank":"Herald"}}`,
		TimeoutAt: time.Now().UTC().Add(time.Hour),
		SpaceID:   &spaceID,
	}
	active, err := sessions.Create(ctx, params)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `UPDATE matchmaking_space_lifecycle_fence_heads SET state='FROZEN' WHERE space_id=$1`, spaceID)
	require.NoError(t, err)

	_, err = sessions.Create(ctx, params)
	require.Error(t, err, "new Space searches must fail after the durable freeze fence")
	_, err = pool.Exec(ctx, `UPDATE search_sessions SET status='pending_accept' WHERE id=$1`, active.ID)
	require.Error(t, err, "matcher promotion must fail after the durable freeze fence")

	cancelled, err := sessions.Cancel(ctx, active.ID)
	require.NoError(t, err, "users must still be able to cancel a frozen search")
	require.Equal(t, SessionStatusCancelled, cancelled.Status)
}
