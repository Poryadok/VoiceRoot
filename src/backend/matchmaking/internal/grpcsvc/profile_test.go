package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/matchmaking/internal/store"

	matchmakingv1 "voice.app/voice/matchmaking/v1"
)

func newProfileTestServer(t *testing.T, ctx context.Context) (*MatchmakingGRPC, uuid.UUID) {
	t.Helper()
	pool := startDB(t, ctx)
	profileID := uuid.New()
	return &MatchmakingGRPC{
		Games:        &store.GameStore{Pool: pool},
		ProfileGames: &store.ProfileGamesStore{Pool: pool},
	}, profileID
}

func TestGetMyPlayerProfile_StoreUnavailable(t *testing.T) {
	t.Parallel()
	srv := &MatchmakingGRPC{}
	_, err := srv.GetMyPlayerProfile(ctxWithProfile(uuid.New()), &matchmakingv1.GetMyPlayerProfileRequest{})
	require.Equal(t, codes.Unavailable, status.Code(err))
}

func TestGetMyPlayerProfile_Unauthenticated(t *testing.T) {
	t.Parallel()
	srv := &MatchmakingGRPC{ProfileGames: &store.ProfileGamesStore{}}
	_, err := srv.GetMyPlayerProfile(context.Background(), &matchmakingv1.GetMyPlayerProfileRequest{})
	require.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestGetPlayerProfile_InvalidProfileID(t *testing.T) {
	t.Parallel()
	srv := &MatchmakingGRPC{ProfileGames: &store.ProfileGamesStore{}}
	_, err := srv.GetPlayerProfile(ctxWithProfile(uuid.New()), &matchmakingv1.GetPlayerProfileRequest{ProfileId: "bad"})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestUpsertPlayerGameEntry_ValidatesRegion(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	srv, profileID := newProfileTestServer(t, ctx)

	list, err := srv.ListGames(ctx, &matchmakingv1.ListGamesRequest{})
	require.NoError(t, err)
	require.NotEmpty(t, list.GetGameList().GetGames())
	gameID := list.GetGameList().GetGames()[0].GetId()

	_, err = srv.UpsertPlayerGameEntry(ctxWithProfile(profileID), &matchmakingv1.UpsertPlayerGameEntryRequest{
		GameId: gameID,
		Region: "invalid-region",
	})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestGetPlayerProfile_ReturnsEntries(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	srv, ownerID := newProfileTestServer(t, ctx)
	viewerID := uuid.New()

	search, err := srv.SearchGames(ctx, &matchmakingv1.SearchGamesRequest{Query: "dota"})
	require.NoError(t, err)
	gameID := search.GetGameList().GetGames()[0].GetId()
	role := "Carry"
	_, err = srv.UpsertPlayerGameEntry(ctxWithProfile(ownerID), &matchmakingv1.UpsertPlayerGameEntryRequest{
		GameId: gameID,
		Region: "eu",
		Role:   &role,
	})
	require.NoError(t, err)

	resp, err := srv.GetPlayerProfile(ctxWithProfile(viewerID), &matchmakingv1.GetPlayerProfileRequest{
		ProfileId: ownerID.String(),
	})
	require.NoError(t, err)
	require.Len(t, resp.GetEntries(), 1)
}

func TestDeletePlayerGameEntry_RemovesEntry(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	srv, profileID := newProfileTestServer(t, ctx)

	search, err := srv.SearchGames(ctx, &matchmakingv1.SearchGamesRequest{Query: "dota"})
	require.NoError(t, err)
	gameID := search.GetGameList().GetGames()[0].GetId()

	_, err = srv.UpsertPlayerGameEntry(ctxWithProfile(profileID), &matchmakingv1.UpsertPlayerGameEntryRequest{
		GameId: gameID,
		Region: "eu",
	})
	require.NoError(t, err)

	_, err = srv.DeletePlayerGameEntry(ctxWithProfile(profileID), &matchmakingv1.DeletePlayerGameEntryRequest{
		GameId: gameID,
	})
	require.NoError(t, err)

	my, err := srv.GetMyPlayerProfile(ctxWithProfile(profileID), &matchmakingv1.GetMyPlayerProfileRequest{})
	require.NoError(t, err)
	require.Empty(t, my.GetEntries())
}

func TestUpsertPlayerGameEntry_PersistsDotaProfile(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	srv, profileID := newProfileTestServer(t, ctx)

	search, err := srv.SearchGames(ctx, &matchmakingv1.SearchGamesRequest{Query: "dota"})
	require.NoError(t, err)
	require.Len(t, search.GetGameList().GetGames(), 1)
	gameID := search.GetGameList().GetGames()[0].GetId()

	role := "Carry"
	rank := "Herald"
	resp, err := srv.UpsertPlayerGameEntry(ctxWithProfile(profileID), &matchmakingv1.UpsertPlayerGameEntryRequest{
		GameId: gameID,
		Region: "eu",
		Role:   &role,
		Rank:   &rank,
	})
	require.NoError(t, err)
	require.Equal(t, gameID, resp.GetEntry().GetGameId())
	require.Equal(t, "eu", resp.GetEntry().GetRegion())
	require.Equal(t, "Carry", resp.GetEntry().GetRole())
	require.Equal(t, "Herald", resp.GetEntry().GetRank())

	my, err := srv.GetMyPlayerProfile(ctxWithProfile(profileID), &matchmakingv1.GetMyPlayerProfileRequest{})
	require.NoError(t, err)
	require.Len(t, my.GetEntries(), 1)
}

func TestUpsertPlayerGameEntry_ArchivedGameNotFound(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	srv, profileID := newProfileTestServer(t, ctx)

	created, err := srv.CreateGame(ctxWithProfile(profileID), &matchmakingv1.CreateGameRequest{
		Name:       "Archive Test",
		ConfigJson: validConfigJSON(),
	})
	require.NoError(t, err)

	archived := store.StatusArchived
	_, err = srv.UpdateGame(ctxWithProfile(profileID), &matchmakingv1.UpdateGameRequest{
		GameId: created.GetGame().GetId(),
		Status: &archived,
	})
	require.NoError(t, err)

	_, err = srv.UpsertPlayerGameEntry(ctxWithProfile(profileID), &matchmakingv1.UpsertPlayerGameEntryRequest{
		GameId: created.GetGame().GetId(),
		Region: "eu",
	})
	require.Equal(t, codes.NotFound, status.Code(err))
}

func TestDeletePlayerGameEntry_NotFound(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	srv, profileID := newProfileTestServer(t, ctx)
	_, err := srv.DeletePlayerGameEntry(ctxWithProfile(profileID), &matchmakingv1.DeletePlayerGameEntryRequest{
		GameId: uuid.New().String(),
	})
	require.Equal(t, codes.NotFound, status.Code(err))
}
