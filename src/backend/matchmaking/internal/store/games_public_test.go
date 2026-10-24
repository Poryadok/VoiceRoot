package store

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestPublicGamesCursorStrictValidation(t *testing.T) {
	hash := strings.Repeat("a", 64)
	for _, cursor := range []string{
		" ", "gpop2." + hash + ".1", "gpop1.a.1", "gpop1." + strings.Repeat("z", 64) + ".1",
		"gpop1." + strings.ToUpper(hash) + ".1", "gpop1." + hash + ".0", "gpop1." + hash + ".-1",
		"gpop1." + hash + ".01", "gpop1." + hash + ".+1", "gpop1." + hash + ".2147483648",
		"gpop1." + hash + ".1.extra", strings.Repeat("x", 129),
	} {
		_, _, err := decodePublicGamesCursor(cursor)
		require.ErrorIs(t, err, ErrGamesCursorInvalid, cursor)
	}
	got, pos, err := decodePublicGamesCursor("gpop1." + hash + ".123")
	require.NoError(t, err)
	require.Equal(t, hash, got)
	require.Equal(t, 123, pos)
}

func TestPublicGamesPagesRejectOrderOrMembershipChurn(t *testing.T) {
	games := []Game{{ID: uuid.New()}, {ID: uuid.New()}, {ID: uuid.New()}}
	first, err := pagePublicGames(games, ListGamesParams{PageSize: 1})
	require.NoError(t, err)
	require.Len(t, first.Games, 1)
	for _, changed := range [][]Game{
		{games[1], games[0], games[2]}, {games[0], games[2]},
		{games[0], games[1], games[2], {ID: uuid.New()}},
	} {
		page, err := pagePublicGames(changed, ListGamesParams{Cursor: first.NextCursor, PageSize: 1})
		require.ErrorIs(t, err, ErrGamesCursorStale)
		require.Empty(t, page.Games)
	}
	// Only order/membership is bound; metadata and hidden score changes that
	// leave the order intact do not stale an in-flight chain.
	games[1].Name = "updated"
	second, err := pagePublicGames(games, ListGamesParams{Cursor: first.NextCursor, PageSize: 1})
	require.NoError(t, err)
	require.Equal(t, "updated", second.Games[0].Name)
	third, err := pagePublicGames(games, ListGamesParams{Cursor: second.NextCursor, PageSize: 1})
	require.NoError(t, err)
	require.Equal(t, games[2].ID, third.Games[0].ID)
	require.Empty(t, third.NextCursor)
	_, err = pagePublicGames(games, ListGamesParams{Cursor: "gpop1." + publicGamesFingerprint(games) + ".3"})
	require.ErrorIs(t, err, ErrGamesCursorInvalid)
}

func TestPublicGamesPageNormalizationAndEmptyCatalog(t *testing.T) {
	games := make([]Game, 101)
	for i := range games {
		games[i].ID = uuid.New()
	}
	for _, size := range []int32{0, -1, 101} {
		page, err := pagePublicGames(games, ListGamesParams{PageSize: size})
		require.NoError(t, err)
		require.Len(t, page.Games, 50)
	}
	page, err := pagePublicGames(nil, ListGamesParams{})
	require.NoError(t, err)
	require.Empty(t, page.Games)
	require.Empty(t, page.NextCursor)
}
