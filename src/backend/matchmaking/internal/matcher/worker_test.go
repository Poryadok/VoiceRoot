package matcher_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"voice/backend/matchmaking/internal/config"
	"voice/backend/matchmaking/internal/criteria"
	"voice/backend/matchmaking/internal/matcher"
	"voice/backend/matchmaking/internal/queue"
	"voice/backend/matchmaking/internal/store"
)

type recordingMatchEvents struct {
	matchFound int
}

func (r *recordingMatchEvents) PublishMatchFound(context.Context, matcher.MatchFoundEvent) error {
	r.matchFound++
	return nil
}

func duoGameConfig() config.GameConfig {
	return config.GameConfig{
		Regions: []string{"eu"},
		Modes: []config.Mode{{
			Name:          "Duo",
			Slots:         2,
			PartySizeMin:  1,
			PartySizeMax:  1,
			RolesRequired: false,
			RankRequired:  false,
		}},
	}
}

func TestWorker_TwoCompatibleSoloSessionsMatchFor2SlotGame(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := store.StartMatchmakingDBForStoreTest(t, ctx)
	store.ApplyMatchmakingMigrationsForStoreTest(t, ctx, pool)

	games := &store.GameStore{Pool: pool}
	game, err := games.Create(ctx, "MM Duo Test", duoGameConfig(), uuid.New())
	require.NoError(t, err)

	sessions := &store.SessionStore{Pool: pool}
	matches := &store.MatchStore{Pool: pool}
	timeout := time.Now().UTC().Add(30 * time.Minute)
	crit := criteria.MustMarshal(criteria.SearchCriteria{Region: "eu"})

	profileA := uuid.New()
	profileB := uuid.New()
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
	q := &queue.RedisQueue{Client: rdb, Prefix: "matcher-test"}
	now := time.Now().UTC()
	require.NoError(t, q.Enqueue(ctx, game.ID, "Duo", "eu", sessA.ID, now))
	require.NoError(t, q.Enqueue(ctx, game.ID, "Duo", "eu", sessB.ID, now.Add(time.Millisecond)))

	events := &recordingMatchEvents{}
	worker := &matcher.Worker{
		Queue:    q,
		Sessions: sessions,
		Matches:  matches,
		Games:    games,
		Events:   events,
	}
	require.NoError(t, worker.RunOnce(ctx))

	updatedA, err := sessions.Get(ctx, sessA.ID)
	require.NoError(t, err)
	updatedB, err := sessions.Get(ctx, sessB.ID)
	require.NoError(t, err)
	require.Equal(t, "pending_accept", updatedA.Status)
	require.Equal(t, "pending_accept", updatedB.Status)
	require.NotNil(t, updatedA.MatchID)
	require.Equal(t, updatedA.MatchID, updatedB.MatchID)

	match, err := matches.Get(ctx, *updatedA.MatchID)
	require.NoError(t, err)
	require.Equal(t, "pending_accept", match.Status)
	require.Equal(t, "eu", match.Region)
	require.Equal(t, 2, match.SlotCount())

	require.Equal(t, 1, events.matchFound, "mm.match_found must be published once")
}
