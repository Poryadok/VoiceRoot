package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestRatingStore_InsertMatchRatingPersistsRow(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := StartMatchmakingDBForStoreTest(t, ctx)
	ApplyMatchmakingMigrationsForStoreTest(t, ctx, pool)

	matchID := uuid.New()
	rater := uuid.New()
	rated := uuid.New()
	ratings := &RatingStore{Pool: pool}

	err := ratings.InsertMatchRating(ctx, InsertMatchRatingParams{
		MatchID:        matchID,
		RaterProfileID: rater,
		RatedProfileID: rated,
		Stars:          4,
	})
	require.NoError(t, err)
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

	first, err := ratings.UpsertPlayerRating(ctx, profileID, list.Games[0].ID, 5)
	require.NoError(t, err)
	require.Equal(t, 5.0, first.RatingValue)
	require.Equal(t, int32(1), first.GamesPlayed)

	second, err := ratings.UpsertPlayerRating(ctx, profileID, list.Games[0].ID, 3)
	require.NoError(t, err)
	require.Equal(t, 4.0, second.RatingValue)
	require.Equal(t, int32(2), second.GamesPlayed)
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
	_, err = ratings.UpsertPlayerRating(ctx, profileID, list.Games[0].ID, 4)
	require.NoError(t, err)

	got, err := ratings.GetPlayerRating(ctx, profileID, list.Games[0].ID)
	require.NoError(t, err)
	require.Equal(t, profileID, got.ProfileID)
	require.Equal(t, list.Games[0].ID, got.GameID)
	require.Equal(t, 4.0, got.RatingValue)
	require.Equal(t, int32(1), got.GamesPlayed)
}

func TestRatingStore_InsertMatchRatingEnforcesUniqueness(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := StartMatchmakingDBForStoreTest(t, ctx)
	ApplyMatchmakingMigrationsForStoreTest(t, ctx, pool)

	matchID := uuid.New()
	rater := uuid.New()
	rated := uuid.New()
	ratings := &RatingStore{Pool: pool}
	params := InsertMatchRatingParams{
		MatchID:        matchID,
		RaterProfileID: rater,
		RatedProfileID: rated,
		Stars:          5,
	}
	require.NoError(t, ratings.InsertMatchRating(ctx, params))

	err := ratings.InsertMatchRating(ctx, params)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrDuplicateMatchRating)
}

func TestRatingStore_InsertMatchRatingRejectsInvalidStars(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := StartMatchmakingDBForStoreTest(t, ctx)
	ApplyMatchmakingMigrationsForStoreTest(t, ctx, pool)

	ratings := &RatingStore{Pool: pool}
	err := ratings.InsertMatchRating(ctx, InsertMatchRatingParams{
		MatchID:        uuid.New(),
		RaterProfileID: uuid.New(),
		RatedProfileID: uuid.New(),
		Stars:          0,
	})
	require.Error(t, err)

	err = ratings.InsertMatchRating(ctx, InsertMatchRatingParams{
		MatchID:        uuid.New(),
		RaterProfileID: uuid.New(),
		RatedProfileID: uuid.New(),
		Stars:          6,
	})
	require.Error(t, err)
}
