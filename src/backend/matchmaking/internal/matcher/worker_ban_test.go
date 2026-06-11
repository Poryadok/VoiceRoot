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

func TestWorker_PeerBannedUsersNotMatched(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := store.StartMatchmakingDBForStoreTest(t, ctx)
	store.ApplyMatchmakingMigrationsForStoreTest(t, ctx, pool)

	games := &store.GameStore{Pool: pool}
	game, err := games.Create(ctx, "MM Ban Test", duoGameConfig(), uuid.New())
	require.NoError(t, err)

	sessions := &store.SessionStore{Pool: pool}
	matches := &store.MatchStore{Pool: pool}
	bans := &store.BanStore{Pool: pool}
	timeout := time.Now().UTC().Add(30 * time.Minute)
	crit := criteria.MustMarshal(criteria.SearchCriteria{Region: "eu"})

	profileA := uuid.New()
	profileB := uuid.New()
	require.NoError(t, bans.InsertMMPeerBan(ctx, store.InsertMMPeerBanParams{
		BannerProfileID: profileA,
		TargetProfileID: profileB,
	}))

	sessA, err := sessions.Create(ctx, store.CreateSessionParams{
		ProfileID: profileA,
		GameID:    game.ID,
		Mode:      "Duo",
		Criteria:  crit,
		TimeoutAt: timeout,
	})
	require.NoError(t, err)
	sessB, err := sessions.Create(ctx, store.CreateSessionParams{
		ProfileID: profileB,
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
	q := &queue.RedisQueue{Client: rdb, Prefix: "matcher-ban-test"}
	now := time.Now().UTC()
	require.NoError(t, q.Enqueue(ctx, game.ID, "Duo", "eu", sessA.ID, now))
	require.NoError(t, q.Enqueue(ctx, game.ID, "Duo", "eu", sessB.ID, now.Add(time.Millisecond)))

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

	updatedA, err := sessions.Get(ctx, sessA.ID)
	require.NoError(t, err)
	updatedB, err := sessions.Get(ctx, sessB.ID)
	require.NoError(t, err)
	require.Equal(t, "searching", updatedA.Status)
	require.Equal(t, "searching", updatedB.Status)
	require.Nil(t, updatedA.MatchID)
	require.Nil(t, updatedB.MatchID)
	require.Equal(t, 0, events.matchFound, "peer-banned users must not be matched together")
}
