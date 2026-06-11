package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/matchmaking/internal/config"
)

func validTestConfig() config.GameConfig {
	return config.GameConfig{
		Regions: []string{"eu"},
		Modes: []config.Mode{{
			Name:         "5v5 Ranked",
			Slots:        10,
			PartySizeMin: 1,
			PartySizeMax: 5,
			Roles: []config.Role{
				{Name: "Carry", Required: true},
				{Name: "Support", Required: false},
			},
			Ranks: []config.Rank{{Name: "Herald", Value: 0}},
		}},
	}
}

func TestGameStore_CreateAndGet(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := StartMatchmakingDBForStoreTest(t, ctx)
	ApplyMatchmakingMigrationsForStoreTest(t, ctx, pool)
	s := &GameStore{Pool: pool}
	creator := uuid.New()

	created, err := s.Create(ctx, "Test MOBA", validTestConfig(), creator)
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, created.ID)
	require.Equal(t, "active", created.Status)
	require.Equal(t, creator, *created.CreatedBy)
	require.Len(t, created.Config.Modes[0].Roles, 2)

	got, err := s.Get(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, created.ID, got.ID)
	require.Equal(t, "Carry", got.Config.Modes[0].Roles[0].Name)
}

func TestGameStore_ListActiveIncludesSeeds(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := StartMatchmakingDBForStoreTest(t, ctx)
	ApplyMatchmakingMigrationsForStoreTest(t, ctx, pool)
	s := &GameStore{Pool: pool}

	res, err := s.List(ctx, ListGamesParams{PageSize: 10})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(res.Games), 4)
	names := make([]string, len(res.Games))
	for i, g := range res.Games {
		names[i] = g.Name
	}
	require.Contains(t, names, "Dota 2")
}

func TestGameStore_SearchByName(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := StartMatchmakingDBForStoreTest(t, ctx)
	ApplyMatchmakingMigrationsForStoreTest(t, ctx, pool)
	s := &GameStore{Pool: pool}

	games, err := s.Search(ctx, "dota", 10)
	require.NoError(t, err)
	require.Len(t, games, 1)
	require.Equal(t, "Dota 2", games[0].Name)
	require.NotEmpty(t, games[0].Config.Modes[0].Roles)
}

func TestGameStore_UpdateConfig(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := StartMatchmakingDBForStoreTest(t, ctx)
	ApplyMatchmakingMigrationsForStoreTest(t, ctx, pool)
	s := &GameStore{Pool: pool}
	created, err := s.Create(ctx, "Updatable", validTestConfig(), uuid.New())
	require.NoError(t, err)

	cfg := validTestConfig()
	cfg.Modes[0].Roles = append(cfg.Modes[0].Roles, config.Role{Name: "Mid", Required: false})
	updated, err := s.Update(ctx, created.ID, UpdateGameParams{Config: &cfg})
	require.NoError(t, err)
	require.Len(t, updated.Config.Modes[0].Roles, 3)
}

func TestGameStore_ListPagination(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := StartMatchmakingDBForStoreTest(t, ctx)
	ApplyMatchmakingMigrationsForStoreTest(t, ctx, pool)
	s := &GameStore{Pool: pool}
	creator := uuid.New()

	for i := 0; i < 3; i++ {
		cfg := validTestConfig()
		_, err := s.Create(ctx, "Extra Game "+string(rune('A'+i)), cfg, creator)
		require.NoError(t, err)
	}

	page1, err := s.List(ctx, ListGamesParams{PageSize: 2})
	require.NoError(t, err)
	require.Len(t, page1.Games, 2)
	require.NotEmpty(t, page1.NextCursor)

	page2, err := s.List(ctx, ListGamesParams{PageSize: 2, Cursor: page1.NextCursor})
	require.NoError(t, err)
	require.NotEmpty(t, page2.Games)
}

func TestGameStore_GetNotFound(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := StartMatchmakingDBForStoreTest(t, ctx)
	ApplyMatchmakingMigrationsForStoreTest(t, ctx, pool)
	s := &GameStore{Pool: pool}
	_, err := s.Get(ctx, uuid.New())
	require.ErrorIs(t, err, ErrGameNotFound)
}
