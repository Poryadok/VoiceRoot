package grpcsvc

import (
	"context"
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

func startDB(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()
	pool := integrationtest.StartPostgres(t, ctx, "matchmakinggrpc", "")
	store.ApplyMatchmakingMigrationsForStoreTest(t, ctx, pool)
	return pool
}

func ctxWithProfile(profileID uuid.UUID) context.Context {
	return metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-voice-profile-id", profileID.String()))
}

func ctxWithProfileAccount(profileID, accountID uuid.UUID) context.Context {
	return metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		"x-voice-profile-id", profileID.String(),
		"x-voice-user-id", accountID.String(),
	))
}

func ctxWithStaff(profileID uuid.UUID) context.Context {
	return metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		"x-voice-profile-id", profileID.String(),
		"x-voice-roles", "staff",
	))
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
	_, err := srv.CreateGame(ctxWithStaff(uuid.New()), &matchmakingv1.CreateGameRequest{
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

// TestUpdateGame_NonStaffDeniedBeforeStoreUse documents the game-catalog rule:
// only platform staff may publish or change catalog entries. A regular
// authenticated profile must be denied before request validation or any store
// mutation is attempted.
func TestUpdateGame_NonStaffDeniedBeforeStoreUse(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		req  *matchmakingv1.UpdateGameRequest
	}{
		{
			name: "invalid game id",
			req:  &matchmakingv1.UpdateGameRequest{GameId: "not-a-uuid"},
		},
		{
			name: "invalid config",
			req: func() *matchmakingv1.UpdateGameRequest {
				configJSON := `{"regions":[],"modes":[]}`
				return &matchmakingv1.UpdateGameRequest{
					GameId:     uuid.New().String(),
					ConfigJson: &configJSON,
				}
			}(),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := &MatchmakingGRPC{}
			_, err := srv.UpdateGame(ctxWithProfile(uuid.New()), tc.req)
			require.Equal(t, codes.PermissionDenied, status.Code(err))
		})
	}
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
	_, err := srv.UpdateGame(ctxWithStaff(uuid.New()), &matchmakingv1.UpdateGameRequest{
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
	created, err := srv.CreateGame(ctxWithStaff(uuid.New()), &matchmakingv1.CreateGameRequest{
		Name:       "Cfg Test",
		ConfigJson: validConfigJSON(),
	})
	require.NoError(t, err)
	bad := `{"regions":[],"modes":[]}`
	_, err = srv.UpdateGame(ctxWithStaff(uuid.New()), &matchmakingv1.UpdateGameRequest{
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
	srv := &MatchmakingGRPC{Games: &store.GameStore{}}

	_, err := srv.GetGame(context.Background(), &matchmakingv1.GetGameRequest{GameId: "not-a-uuid"})
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

	_, err := srv.CreateGame(ctxWithStaff(uuid.New()), &matchmakingv1.CreateGameRequest{
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

	resp, err := srv.CreateGame(ctxWithStaff(uuid.New()), &matchmakingv1.CreateGameRequest{
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

	created, err := srv.CreateGame(ctxWithStaff(profile), &matchmakingv1.CreateGameRequest{
		Name:       "Rename Me",
		ConfigJson: validConfigJSON(),
	})
	require.NoError(t, err)

	newName := "Renamed Arena"
	resp, err := srv.UpdateGame(ctxWithStaff(profile), &matchmakingv1.UpdateGameRequest{
		GameId: created.GetGame().GetId(),
		Name:   &newName,
	})
	require.NoError(t, err)
	require.Equal(t, "Renamed Arena", resp.GetGame().GetName())
}

func TestUpdateGame_NonStaffLeavesConfigUnchanged(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startDB(t, ctx)
	srv := &MatchmakingGRPC{Games: &store.GameStore{Pool: pool}}
	staff := ctxWithStaff(uuid.New())
	created, err := srv.CreateGame(staff, &matchmakingv1.CreateGameRequest{
		Name:       "Protected Catalog Entry",
		ConfigJson: validConfigJSON(),
	})
	require.NoError(t, err)

	updatedConfig := config.MustMarshal(config.GameConfig{
		Regions: []string{"na"},
		Modes: []config.Mode{{
			Name: "3v3", Slots: 6, PartySizeMin: 1, PartySizeMax: 3,
			Roles: []config.Role{{Name: "Support", Required: true}},
			Ranks: []config.Rank{{Name: "Silver", Value: 1}},
		}},
	})
	_, err = srv.UpdateGame(ctxWithProfile(uuid.New()), &matchmakingv1.UpdateGameRequest{
		GameId:     created.GetGame().GetId(),
		ConfigJson: &updatedConfig,
	})
	require.Equal(t, codes.PermissionDenied, status.Code(err))

	stored, err := srv.GetGame(ctx, &matchmakingv1.GetGameRequest{GameId: created.GetGame().GetId()})
	require.NoError(t, err)
	require.Equal(t, created.GetGame().GetConfigJson(), stored.GetGame().GetConfigJson())
}

func TestCreateGame_NonStaffDenied(t *testing.T) {
	t.Parallel()
	srv := &MatchmakingGRPC{Games: &store.GameStore{}}
	_, err := srv.CreateGame(ctxWithProfile(uuid.New()), &matchmakingv1.CreateGameRequest{
		Name:       "X",
		ConfigJson: validConfigJSON(),
	})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
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
