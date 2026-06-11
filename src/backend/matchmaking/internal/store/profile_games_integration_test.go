package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/matchmaking/internal/config"
	"voice/backend/matchmaking/internal/store"
)

func TestProfileGamesStore_UpsertListDelete(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := store.StartMatchmakingDBForStoreTest(t, ctx)
	store.ApplyMatchmakingMigrationsForStoreTest(t, ctx, pool)

	games := &store.GameStore{Pool: pool}
	profiles := &store.ProfileGamesStore{Pool: pool}
	profileID := uuid.New()
	cfg := config.GameConfig{
		Regions: []string{"eu"},
		Modes: []config.Mode{{
			Name: "5v5", Slots: 10, PartySizeMin: 1, PartySizeMax: 5,
			Roles: []config.Role{{Name: "Carry", Required: true}},
			Ranks: []config.Rank{{Name: "Herald", Value: 0}},
		}},
	}
	game, err := games.Create(ctx, "Profile Test Game", cfg, profileID)
	require.NoError(t, err)

	role := "Carry"
	rank := "Herald"
	_, err = profiles.Upsert(ctx, store.UpsertProfileGameParams{
		ProfileID: profileID,
		GameID:    game.ID,
		Region:    "eu",
		Role:      &role,
		Rank:      &rank,
	})
	require.NoError(t, err)

	time.Sleep(5 * time.Millisecond)
	rank2 := "Ancient"
	_, err = profiles.Upsert(ctx, store.UpsertProfileGameParams{
		ProfileID: profileID,
		GameID:    game.ID,
		Region:    "eu",
		Rank:      &rank2,
	})
	require.NoError(t, err)

	entries, err := profiles.ListByProfile(ctx, profileID)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, "eu", entries[0].Region)
	require.NotNil(t, entries[0].Rank)
	require.Equal(t, "Ancient", *entries[0].Rank)

	err = profiles.Delete(ctx, profileID, game.ID)
	require.NoError(t, err)
	entries, err = profiles.ListByProfile(ctx, profileID)
	require.NoError(t, err)
	require.Empty(t, entries)
}

func TestProfileGamesStore_DeleteNotFound(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := store.StartMatchmakingDBForStoreTest(t, ctx)
	store.ApplyMatchmakingMigrationsForStoreTest(t, ctx, pool)
	profiles := &store.ProfileGamesStore{Pool: pool}
	err := profiles.Delete(ctx, uuid.New(), uuid.New())
	require.ErrorIs(t, err, store.ErrProfileGameEntryNotFound)
}
