package grpcsvc

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"voice/backend/matchmaking/internal/config"
	"voice/backend/matchmaking/internal/store"
	"voice/backend/pkg/integrationtest"

	matchmakingv1 "voice.app/voice/matchmaking/v1"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", ".."))
}

func startDB(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()
	pool := integrationtest.StartPostgres(t, ctx, "matchmakinggrpc", "")
	migrationPath := filepath.Join(repoRoot(t), "src", "backend", "migrations", "matchmaking_db", "000001_init.up.sql")
	sqlBytes, err := os.ReadFile(migrationPath)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(sqlBytes))
	require.NoError(t, err)
	return pool
}

func ctxWithProfile(profileID uuid.UUID) context.Context {
	return metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-voice-profile-id", profileID.String()))
}

func validConfigJSON() string {
	return config.MustMarshal(config.GameConfig{
		Regions: []string{"eu"},
		Modes: []config.Mode{{
			Name: "5v5", Slots: 10, PartySizeMin: 1, PartySizeMax: 5,
			Roles: []config.Role{{Name: "Carry", Required: true}},
			Ranks: []config.Rank{{Name: "Bronze", Value: 0}},
		}},
	})
}

func TestGetGame_StoreUnavailable(t *testing.T) {
	t.Parallel()
	srv := &MatchmakingGRPC{}
	_, err := srv.GetGame(context.Background(), &matchmakingv1.GetGameRequest{GameId: uuid.New().String()})
	require.Equal(t, codes.Unavailable, status.Code(err))
}

func TestSearchGames_StoreUnavailable(t *testing.T) {
	t.Parallel()
	srv := &MatchmakingGRPC{}
	_, err := srv.SearchGames(context.Background(), &matchmakingv1.SearchGamesRequest{Query: "x"})
	require.Equal(t, codes.Unavailable, status.Code(err))
}

func TestCreateGame_EmptyNameRejected(t *testing.T) {
	t.Parallel()
	srv := &MatchmakingGRPC{Games: &store.GameStore{}}
	_, err := srv.CreateGame(ctxWithProfile(uuid.New()), &matchmakingv1.CreateGameRequest{
		Name:       "  ",
		ConfigJson: validConfigJSON(),
	})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestUpdateGame_Unauthenticated(t *testing.T) {
	t.Parallel()
	srv := &MatchmakingGRPC{Games: &store.GameStore{}}
	name := "x"
	_, err := srv.UpdateGame(context.Background(), &matchmakingv1.UpdateGameRequest{
		GameId: uuid.New().String(),
		Name:   &name,
	})
	require.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestListGames_StoreUnavailable(t *testing.T) {
	t.Parallel()
	srv := &MatchmakingGRPC{}
	_, err := srv.ListGames(context.Background(), &matchmakingv1.ListGamesRequest{})
	require.Equal(t, codes.Unavailable, status.Code(err))
}

func TestUpdateGame_NotFound(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startDB(t, ctx)
	srv := &MatchmakingGRPC{Games: &store.GameStore{Pool: pool}}
	name := "missing"
	_, err := srv.UpdateGame(ctxWithProfile(uuid.New()), &matchmakingv1.UpdateGameRequest{
		GameId: uuid.New().String(),
		Name:   &name,
	})
	require.Equal(t, codes.NotFound, status.Code(err))
}

func TestUpdateGame_InvalidConfigRejected(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startDB(t, ctx)
	srv := &MatchmakingGRPC{Games: &store.GameStore{Pool: pool}}
	created, err := srv.CreateGame(ctxWithProfile(uuid.New()), &matchmakingv1.CreateGameRequest{
		Name:       "Cfg Test",
		ConfigJson: validConfigJSON(),
	})
	require.NoError(t, err)
	bad := `{"regions":[],"modes":[]}`
	_, err = srv.UpdateGame(ctxWithProfile(uuid.New()), &matchmakingv1.UpdateGameRequest{
		GameId:     created.GetGame().GetId(),
		ConfigJson: &bad,
	})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestCreateGame_Unauthenticated(t *testing.T) {
	t.Parallel()
	srv := &MatchmakingGRPC{Games: &store.GameStore{}}
	_, err := srv.CreateGame(context.Background(), &matchmakingv1.CreateGameRequest{
		Name:       "X",
		ConfigJson: validConfigJSON(),
	})
	require.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestGetGame_InvalidID(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := startDB(t, ctx)
	srv := &MatchmakingGRPC{Games: &store.GameStore{Pool: pool}}

	_, err := srv.GetGame(ctx, &matchmakingv1.GetGameRequest{GameId: "not-a-uuid"})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestListGames_ReturnsSeededCatalog(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startDB(t, ctx)
	srv := &MatchmakingGRPC{Games: &store.GameStore{Pool: pool}}

	resp, err := srv.ListGames(ctx, &matchmakingv1.ListGamesRequest{})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(resp.GetGameList().GetGames()), 4)
}

func TestGetGame_NotFound(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startDB(t, ctx)
	srv := &MatchmakingGRPC{Games: &store.GameStore{Pool: pool}}

	_, err := srv.GetGame(ctx, &matchmakingv1.GetGameRequest{GameId: uuid.New().String()})
	require.Equal(t, codes.NotFound, status.Code(err))
}

func TestCreateGame_InvalidConfigRejected(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startDB(t, ctx)
	srv := &MatchmakingGRPC{Games: &store.GameStore{Pool: pool}}

	_, err := srv.CreateGame(ctxWithProfile(uuid.New()), &matchmakingv1.CreateGameRequest{
		Name:       "Bad",
		ConfigJson: `{"regions":[],"modes":[]}`,
	})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestCreateGame_PersistsRolesAndRanks(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startDB(t, ctx)
	srv := &MatchmakingGRPC{Games: &store.GameStore{Pool: pool}}

	resp, err := srv.CreateGame(ctxWithProfile(uuid.New()), &matchmakingv1.CreateGameRequest{
		Name:       "Custom Arena",
		ConfigJson: validConfigJSON(),
	})
	require.NoError(t, err)
	require.Equal(t, "Custom Arena", resp.GetGame().GetName())
	require.Contains(t, resp.GetGame().GetConfigJson(), "Carry")
}

func TestUpdateGame_ChangesName(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startDB(t, ctx)
	srv := &MatchmakingGRPC{Games: &store.GameStore{Pool: pool}}
	profile := uuid.New()

	created, err := srv.CreateGame(ctxWithProfile(profile), &matchmakingv1.CreateGameRequest{
		Name:       "Rename Me",
		ConfigJson: validConfigJSON(),
	})
	require.NoError(t, err)

	newName := "Renamed Arena"
	resp, err := srv.UpdateGame(ctxWithProfile(profile), &matchmakingv1.UpdateGameRequest{
		GameId: created.GetGame().GetId(),
		Name:   &newName,
	})
	require.NoError(t, err)
	require.Equal(t, "Renamed Arena", resp.GetGame().GetName())
}

func TestSearchGames_FindsDota(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startDB(t, ctx)
	srv := &MatchmakingGRPC{Games: &store.GameStore{Pool: pool}}

	resp, err := srv.SearchGames(ctx, &matchmakingv1.SearchGamesRequest{Query: "dota"})
	require.NoError(t, err)
	require.Len(t, resp.GetGameList().GetGames(), 1)
	require.Equal(t, "Dota 2", resp.GetGameList().GetGames()[0].GetName())
}
