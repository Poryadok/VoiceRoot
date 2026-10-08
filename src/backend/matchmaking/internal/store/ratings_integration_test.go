package store

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestRatingStore_RecordMatchRatingPersistsVoteAndAggregate(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := StartMatchmakingDBForStoreTest(t, ctx)
	ApplyMatchmakingMigrationsForStoreTest(t, ctx, pool)

	games := &GameStore{Pool: pool}
	list, err := games.List(ctx, ListGamesParams{PageSize: 1, Status: StatusActive})
	require.NoError(t, err)
	require.NotEmpty(t, list.Games)
	matchID := insertRatingMatch(t, ctx, pool, list.Games[0].ID)
	params := InsertMatchRatingParams{
		MatchID: matchID, RaterProfileID: uuid.New(), RatedProfileID: uuid.New(), Stars: 4,
	}
	ratings := &RatingStore{Pool: pool}

	err = ratings.RecordMatchRating(ctx, params)
	require.NoError(t, err)
	var votes, total int
	var average float64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM match_ratings WHERE match_id = $1`, matchID,
	).Scan(&votes))
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT average_rating, total_ratings_received FROM player_ratings WHERE profile_id = $1 AND game_id = $2`,
		params.RatedProfileID, list.Games[0].ID,
	).Scan(&average, &total))
	require.Equal(t, 1, votes)
	require.Equal(t, 1, total)
	require.Equal(t, 4.0, average)
}

func TestRatingStore_UpsertPlayerRatingAggregatesStars(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := StartMatchmakingDBForStoreTest(t, ctx)
	ApplyMatchmakingMigrationsForStoreTest(t, ctx, pool)

	games := &GameStore{Pool: pool}
	list, err := games.List(ctx, ListGamesParams{PageSize: 1, Status: StatusActive})
	require.NoError(t, err)
	require.NotEmpty(t, list.Games)

	profileID := uuid.New()
	ratings := &RatingStore{Pool: pool}

	for _, stars := range []int{5, 3} {
		err := ratings.RecordMatchRating(ctx, InsertMatchRatingParams{
			MatchID: insertRatingMatch(t, ctx, pool, list.Games[0].ID),
			RaterProfileID: uuid.New(), RatedProfileID: profileID, Stars: stars,
		})
		require.NoError(t, err)
	}
	got, err := ratings.GetPlayerRating(ctx, profileID, list.Games[0].ID)
	require.NoError(t, err)
	require.Equal(t, 4.0, got.RatingValue)
	require.Equal(t, int32(0), got.GamesPlayed)
}

func TestRatingStore_GetPlayerRatingReturnsStoredAggregate(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := StartMatchmakingDBForStoreTest(t, ctx)
	ApplyMatchmakingMigrationsForStoreTest(t, ctx, pool)

	games := &GameStore{Pool: pool}
	list, err := games.List(ctx, ListGamesParams{PageSize: 1, Status: StatusActive})
	require.NoError(t, err)
	require.NotEmpty(t, list.Games)

	profileID := uuid.New()
	ratings := &RatingStore{Pool: pool}
	err = ratings.RecordMatchRating(ctx, InsertMatchRatingParams{
		MatchID: insertRatingMatch(t, ctx, pool, list.Games[0].ID),
		RaterProfileID: uuid.New(), RatedProfileID: profileID, Stars: 4,
	})
	require.NoError(t, err)

	got, err := ratings.GetPlayerRating(ctx, profileID, list.Games[0].ID)
	require.NoError(t, err)
	require.Equal(t, profileID, got.ProfileID)
	require.Equal(t, list.Games[0].ID, got.GameID)
	require.Equal(t, 4.0, got.RatingValue)
	require.Equal(t, int32(0), got.GamesPlayed)
}

func TestRatingStore_InsertMatchRatingEnforcesUniqueness(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := StartMatchmakingDBForStoreTest(t, ctx)
	ApplyMatchmakingMigrationsForStoreTest(t, ctx, pool)

	games := &GameStore{Pool: pool}
	list, err := games.List(ctx, ListGamesParams{PageSize: 1, Status: StatusActive})
	require.NoError(t, err)
	require.NotEmpty(t, list.Games)
	matchID := insertRatingMatch(t, ctx, pool, list.Games[0].ID)
	rater := uuid.New()
	rated := uuid.New()
	ratings := &RatingStore{Pool: pool}
	params := InsertMatchRatingParams{
		MatchID: matchID, RaterProfileID: rater, RatedProfileID: rated, Stars: 5,
	}
	require.NoError(t, ratings.RecordMatchRating(ctx, params))

	err = ratings.RecordMatchRating(ctx, params)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrDuplicateMatchRating)
}

func TestRatingStore_RecordMatchRatingRejectsInvalidStars(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := StartMatchmakingDBForStoreTest(t, ctx)
	ApplyMatchmakingMigrationsForStoreTest(t, ctx, pool)

	ratings := &RatingStore{Pool: pool}
	err := ratings.RecordMatchRating(ctx, InsertMatchRatingParams{
		MatchID:        uuid.New(),
		RaterProfileID: uuid.New(),
		RatedProfileID: uuid.New(),
		Stars:          0,
	})
	require.Error(t, err)

	err = ratings.RecordMatchRating(ctx, InsertMatchRatingParams{
		MatchID:        uuid.New(),
		RaterProfileID: uuid.New(),
		RatedProfileID: uuid.New(),
		Stars:          6,
	})
	require.Error(t, err)
}

func TestRatingStore_RecordMatchRatingRollsBackVoteWhenAggregateFails(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := StartMatchmakingDBForStoreTest(t, ctx)
	ApplyMatchmakingMigrationsForStoreTest(t, ctx, pool)

	games := &GameStore{Pool: pool}
	list, err := games.List(ctx, ListGamesParams{PageSize: 1, Status: StatusActive})
	require.NoError(t, err)
	require.NotEmpty(t, list.Games)
	matchID := insertRatingMatch(t, ctx, pool, list.Games[0].ID)
	profileID := uuid.New()
	_, err = pool.Exec(ctx, `
		CREATE FUNCTION fail_player_rating_update() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN RAISE EXCEPTION 'forced aggregate failure'; END;
		$$;
	`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
		CREATE TRIGGER fail_player_rating_update BEFORE UPDATE ON player_ratings
		FOR EACH ROW EXECUTE FUNCTION fail_player_rating_update();
	`)
	require.NoError(t, err)

	ratings := &RatingStore{Pool: pool}
	err = ratings.RecordMatchRating(ctx, InsertMatchRatingParams{
		MatchID: matchID, RaterProfileID: uuid.New(), RatedProfileID: profileID, Stars: 5,
	})
	require.Error(t, err)

	var votes, aggregates int
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM match_ratings WHERE match_id = $1`, matchID).Scan(&votes))
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM player_ratings WHERE profile_id = $1`, profileID).Scan(&aggregates))
	require.Zero(t, votes, "the raw vote must roll back with its failed aggregate update")
	require.Zero(t, aggregates, "the lock row must roll back with the vote")
}

func TestRatingStore_RecordMatchRatingSerializesDistinctConcurrentVotes(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := StartMatchmakingDBForStoreTest(t, ctx)
	ApplyMatchmakingMigrationsForStoreTest(t, ctx, pool)

	games := &GameStore{Pool: pool}
	list, err := games.List(ctx, ListGamesParams{PageSize: 1, Status: StatusActive})
	require.NoError(t, err)
	require.NotEmpty(t, list.Games)
	const count = 8
	stars := []int{1, 2, 3, 4, 5, 1, 2, 3}
	rated := uuid.New()
	params := make([]InsertMatchRatingParams, count)
	for i, score := range stars {
		params[i] = InsertMatchRatingParams{
			MatchID: insertRatingMatch(t, ctx, pool, list.Games[0].ID),
			RaterProfileID: uuid.New(),
			RatedProfileID: rated,
			Stars:          score,
		}
	}

	ratings := &RatingStore{Pool: pool}
	start := make(chan struct{})
	errs := make(chan error, count)
	var wg sync.WaitGroup
	for _, p := range params {
		wg.Add(1)
		go func(p InsertMatchRatingParams) {
			defer wg.Done()
			<-start
			errs <- ratings.RecordMatchRating(ctx, p)
		}(p)
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}

	var total int
	var average float64
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT total_ratings_received, average_rating
		FROM player_ratings WHERE profile_id = $1 AND game_id = $2
	`, rated, list.Games[0].ID).Scan(&total, &average))
	require.Equal(t, count, total)
	require.InDelta(t, 2.625, average, 0.000001)
}

func TestRatingStore_RecordDuplicateRepairsPartialAggregateWithoutDoubleCounting(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := StartMatchmakingDBForStoreTest(t, ctx)
	ApplyMatchmakingMigrationsForStoreTest(t, ctx, pool)

	games := &GameStore{Pool: pool}
	list, err := games.List(ctx, ListGamesParams{PageSize: 1, Status: StatusActive})
	require.NoError(t, err)
	require.NotEmpty(t, list.Games)
	matchID := insertRatingMatch(t, ctx, pool, list.Games[0].ID)
	p := InsertMatchRatingParams{MatchID: matchID, RaterProfileID: uuid.New(), RatedProfileID: uuid.New(), Stars: 4}
	_, err = pool.Exec(ctx, `
		INSERT INTO match_ratings (match_id, rater_profile_id, rated_profile_id, score)
		VALUES ($1, $2, $3, $4)
	`, p.MatchID, p.RaterProfileID, p.RatedProfileID, p.Stars)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
		INSERT INTO player_ratings (profile_id, game_id, average_rating, total_ratings_received)
		VALUES ($1, $2, 0, 0)
	`, p.RatedProfileID, list.Games[0].ID)
	require.NoError(t, err)

	err = (&RatingStore{Pool: pool}).RecordMatchRating(ctx, p)
	require.ErrorIs(t, err, ErrDuplicateMatchRating)

	var total int
	var average float64
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT total_ratings_received, average_rating FROM player_ratings
		WHERE profile_id = $1 AND game_id = $2
	`, p.RatedProfileID, list.Games[0].ID).Scan(&total, &average))
	require.Equal(t, 1, total, "duplicate delivery repairs the aggregate without adding a vote")
	require.Equal(t, 4.0, average)
}

func TestRatingStore_ReconciliationMigrationRepairsExistingAggregates(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := StartMatchmakingDBForStoreTest(t, ctx)
	applyMatchmakingMigrationsUpTo(t, ctx, pool, "000014_space_lifecycle_fences.up.sql")

	games := &GameStore{Pool: pool}
	list, err := games.List(ctx, ListGamesParams{PageSize: 1, Status: StatusActive})
	require.NoError(t, err)
	require.NotEmpty(t, list.Games)
	matchID := insertRatingMatch(t, ctx, pool, list.Games[0].ID)
	rated := uuid.New()
	ratedWithoutAggregate := uuid.New()
	_, err = pool.Exec(ctx, `
		INSERT INTO match_ratings (match_id, rater_profile_id, rated_profile_id, score)
		VALUES ($1, $2, $3, 5), ($1, $4, $3, 3), ($1, $5, $6, 2)
	`, matchID, uuid.New(), rated, uuid.New(), uuid.New(), ratedWithoutAggregate)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
		INSERT INTO player_ratings (profile_id, game_id, average_rating, total_ratings_received)
		VALUES ($1, $2, 5, 1)
	`, rated, list.Games[0].ID)
	require.NoError(t, err)

	repo := repoRoot(t)
	migration, err := os.ReadFile(filepath.Join(repo, "src", "backend", "migrations", "matchmaking_db", "000015_reconcile_player_rating_aggregates.up.sql"))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(migration))
	require.NoError(t, err)

	var total int
	var average float64
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT total_ratings_received, average_rating FROM player_ratings
		WHERE profile_id = $1 AND game_id = $2
	`, rated, list.Games[0].ID).Scan(&total, &average))
	require.Equal(t, 2, total)
	require.InDelta(t, 4.0, average, 0.000001)
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT total_ratings_received, average_rating FROM player_ratings
		WHERE profile_id = $1 AND game_id = $2
	`, ratedWithoutAggregate, list.Games[0].ID).Scan(&total, &average))
	require.Equal(t, 1, total, "migration also materializes an aggregate that the split write never created")
	require.Equal(t, 2.0, average)
}

func insertRatingMatch(t *testing.T, ctx context.Context, pool *pgxpool.Pool, gameID uuid.UUID) uuid.UUID {
	t.Helper()
	var matchID uuid.UUID
	err := pool.QueryRow(ctx, `
		INSERT INTO matches (game_id, mode, region, status)
		VALUES ($1, 'Duo', 'eu', 'completed') RETURNING id
	`, gameID).Scan(&matchID)
	require.NoError(t, err, fmt.Sprintf("insert rating test match for game %s", gameID))
	return matchID
}
