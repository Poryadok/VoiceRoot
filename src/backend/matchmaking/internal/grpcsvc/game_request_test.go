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

// GC-02: user SubmitGameRequest → pending_moderation (docs/features/game-catalog.md, П.4).
func TestSubmitGameRequest_CreatesPendingModeration(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startDB(t, ctx)
	srv := &MatchmakingGRPC{Games: &store.GameStore{Pool: pool}}
	name := "Apex Legends " + uuid.New().String()[:8]

	resp, err := srv.SubmitGameRequest(ctxWithProfile(uuid.New()), &matchmakingv1.SubmitGameRequestRequest{
		Name:       name,
		ConfigJson: validConfigJSON(),
	})
	require.NoError(t, err)
	require.Equal(t, store.StatusPendingModeration, resp.GetGame().GetStatus())
	require.Equal(t, name, resp.GetGame().GetName())

	list, err := srv.ListGames(ctx, &matchmakingv1.ListGamesRequest{})
	require.NoError(t, err)
	for _, g := range list.GetGameList().GetGames() {
		require.NotEqual(t, name, g.GetName(), "pending games must not appear in public catalog")
	}
}

func TestSubmitGameRequest_DuplicateNameRejected(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startDB(t, ctx)
	srv := &MatchmakingGRPC{Games: &store.GameStore{Pool: pool}}

	_, err := srv.SubmitGameRequest(ctxWithProfile(uuid.New()), &matchmakingv1.SubmitGameRequestRequest{
		Name:       "Dota 2",
		ConfigJson: validConfigJSON(),
	})
	require.Equal(t, codes.AlreadyExists, status.Code(err))
}

func TestSubmitGameRequest_RequiresMode(t *testing.T) {
	t.Parallel()
	srv := &MatchmakingGRPC{Games: &store.GameStore{}}
	_, err := srv.SubmitGameRequest(ctxWithProfile(uuid.New()), &matchmakingv1.SubmitGameRequestRequest{
		Name:       "No Modes",
		ConfigJson: `{"regions":["eu"],"modes":[]}`,
	})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

// GC-03: staff approve pending → active in catalog.
func TestApproveGameRequest_PromotesToActive(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startDB(t, ctx)
	srv := &MatchmakingGRPC{Games: &store.GameStore{Pool: pool}}
	name := "New Game " + uuid.New().String()[:8]

	submitted, err := srv.SubmitGameRequest(ctxWithProfile(uuid.New()), &matchmakingv1.SubmitGameRequestRequest{
		Name:       name,
		ConfigJson: validConfigJSON(),
	})
	require.NoError(t, err)

	staff := ctxWithStaff(uuid.New())
	pending, err := srv.ListGameRequests(staff, &matchmakingv1.ListGameRequestsRequest{})
	require.NoError(t, err)
	found := false
	for _, g := range pending.GetGameList().GetGames() {
		if g.GetId() == submitted.GetGame().GetId() {
			found = true
			break
		}
	}
	require.True(t, found, "staff must see pending request")

	approved, err := srv.ApproveGameRequest(staff, &matchmakingv1.ApproveGameRequestRequest{
		GameId: submitted.GetGame().GetId(),
	})
	require.NoError(t, err)
	require.Equal(t, store.StatusActive, approved.GetGame().GetStatus())

	list, err := srv.ListGames(ctx, &matchmakingv1.ListGamesRequest{})
	require.NoError(t, err)
	names := make([]string, 0, len(list.GetGameList().GetGames()))
	for _, g := range list.GetGameList().GetGames() {
		names = append(names, g.GetName())
	}
	require.Contains(t, names, name)
}

func TestApproveGameRequest_NonStaffDenied(t *testing.T) {
	t.Parallel()
	srv := &MatchmakingGRPC{Games: &store.GameStore{}}
	_, err := srv.ApproveGameRequest(ctxWithProfile(uuid.New()), &matchmakingv1.ApproveGameRequestRequest{
		GameId: uuid.New().String(),
	})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestRejectGameRequest_SetsRejected(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startDB(t, ctx)
	srv := &MatchmakingGRPC{Games: &store.GameStore{Pool: pool}}
	name := "Reject Me " + uuid.New().String()[:8]

	submitted, err := srv.SubmitGameRequest(ctxWithProfile(uuid.New()), &matchmakingv1.SubmitGameRequestRequest{
		Name:       name,
		ConfigJson: validConfigJSON(),
	})
	require.NoError(t, err)

	rejected, err := srv.RejectGameRequest(ctxWithStaff(uuid.New()), &matchmakingv1.RejectGameRequestRequest{
		GameId: submitted.GetGame().GetId(),
	})
	require.NoError(t, err)
	require.Equal(t, store.StatusRejected, rejected.GetGame().GetStatus())
}

func TestListGameRequests_NonStaffDenied(t *testing.T) {
	t.Parallel()
	srv := &MatchmakingGRPC{Games: &store.GameStore{}}
	_, err := srv.ListGameRequests(ctxWithProfile(uuid.New()), &matchmakingv1.ListGameRequestsRequest{})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}
