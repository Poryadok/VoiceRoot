package matcher_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"voice/backend/matchmaking/internal/criteria"
	"voice/backend/matchmaking/internal/matcher"
	"voice/backend/matchmaking/internal/queue"
	"voice/backend/matchmaking/internal/store"
)

func TestWorker_SpaceQueueIsolatedFromGlobal(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := store.StartMatchmakingDBForStoreTest(t, ctx)
	store.ApplyMatchmakingMigrationsForStoreTest(t, ctx, pool)

	games := &store.GameStore{Pool: pool}
	game, err := games.Create(ctx, "Space MM Duo", duoGameConfig(), uuid.New())
	require.NoError(t, err)

	sessions := &store.SessionStore{Pool: pool}
	matches := &store.MatchStore{Pool: pool}
	bans := &store.BanStore{Pool: pool}
	timeout := time.Now().UTC().Add(30 * time.Minute)
	crit := criteria.MustMarshal(criteria.SearchCriteria{Region: "eu"})
	spaceID := uuid.New()

	spaceA, err := sessions.Create(ctx, store.CreateSessionParams{
		ProfileID: uuid.New(),
		GameID:    game.ID,
		Mode:      "Duo",
		Criteria:  crit,
		TimeoutAt: timeout,
		SpaceID:   &spaceID,
	})
	require.NoError(t, err)
	spaceB, err := sessions.Create(ctx, store.CreateSessionParams{
		ProfileID: uuid.New(),
		GameID:    game.ID,
		Mode:      "Duo",
		Criteria:  crit,
		TimeoutAt: timeout,
		SpaceID:   &spaceID,
	})
	require.NoError(t, err)
	globalOutsider, err := sessions.Create(ctx, store.CreateSessionParams{
		ProfileID: uuid.New(),
		GameID:    game.ID,
		Mode:      "Duo",
		Criteria:  crit,
		TimeoutAt: timeout,
	})
	require.NoError(t, err)

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		_ = rdb.Close()
		mr.Close()
	})
	q := &queue.RedisQueue{Client: rdb, Prefix: "space-mm-iso"}
	now := time.Now().UTC()
	require.NoError(t, q.EnqueueScoped(ctx, &spaceID, game.ID, "Duo", "eu", spaceA.ID, now))
	require.NoError(t, q.EnqueueScoped(ctx, &spaceID, game.ID, "Duo", "eu", spaceB.ID, now.Add(time.Millisecond)))
	require.NoError(t, q.Enqueue(ctx, game.ID, "Duo", "eu", globalOutsider.ID, now.Add(2*time.Millisecond)))

	events := &recordingMatchEvents{}
	worker := &matcher.Worker{
		Queue:    q,
		Sessions: sessions,
		Matches:  matches,
		Games:    games,
		Bans:     bans,
		Events:   events,
	}
	require.NoError(t, worker.RunOnce(ctx))
	require.Equal(t, 1, events.matchFound)

	updatedA, err := sessions.Get(ctx, spaceA.ID)
	require.NoError(t, err)
	updatedB, err := sessions.Get(ctx, spaceB.ID)
	require.NoError(t, err)
	updatedGlobal, err := sessions.Get(ctx, globalOutsider.ID)
	require.NoError(t, err)

	require.Equal(t, store.SessionStatusPendingAccept, updatedA.Status)
	require.Equal(t, store.SessionStatusPendingAccept, updatedB.Status)
	require.Equal(t, store.SessionStatusSearching, updatedGlobal.Status)

	depth, err := q.QueueDepth(ctx, game.ID, "Duo", "eu")
	require.NoError(t, err)
	require.Equal(t, int64(1), depth)
}
