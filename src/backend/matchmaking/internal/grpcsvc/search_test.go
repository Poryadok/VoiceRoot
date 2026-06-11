package grpcsvc

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/matchmaking/internal/mmevents"
	"voice/backend/matchmaking/internal/queue"
	"voice/backend/matchmaking/internal/store"

	matchmakingv1 "voice.app/voice/matchmaking/v1"
)

func searchTestServer(t *testing.T, pool *pgxpool.Pool) *MatchmakingGRPC {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		_ = rdb.Close()
		mr.Close()
	})
	return &MatchmakingGRPC{
		Games:    &store.GameStore{Pool: pool},
		Sessions: &store.SessionStore{Pool: pool},
		Queue:    &queue.RedisQueue{Client: rdb, Prefix: "test"},
		Events:   mmevents.NoopPublisher{},
	}
}

func dotaGameID(t *testing.T, srv *MatchmakingGRPC, ctx context.Context) string {
	t.Helper()
	resp, err := srv.ListGames(ctx, &matchmakingv1.ListGamesRequest{})
	require.NoError(t, err)
	for _, g := range resp.GetGameList().GetGames() {
		if g.GetName() == "Dota 2" {
			return g.GetId()
		}
	}
	t.Fatal("Dota 2 not in seed")
	return ""
}

func validDotaCriteriaJSON() string {
	return `{"region":"eu","self":{"role":"Carry","rank":"Herald"},"sought":{"rank_min":"Herald","rank_max":"Guardian"}}`
}

func TestStartSearch_HappyPath(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startDB(t, ctx)
	srv := searchTestServer(t, pool)
	profileID := uuid.New()
	ctx = ctxWithProfile(profileID)

	gameID := dotaGameID(t, srv, ctx)
	resp, err := srv.StartSearch(ctx, &matchmakingv1.StartSearchRequest{
		GameId:       gameID,
		Mode:         "5v5 Ranked",
		CriteriaJson: validDotaCriteriaJSON(),
	})
	require.NoError(t, err)
	require.Equal(t, "searching", resp.GetSearchSession().GetStatus())
	require.Equal(t, profileID.String(), resp.GetSearchSession().GetProfileId())

	statusResp, err := srv.GetSearchStatus(ctx, &matchmakingv1.GetSearchStatusRequest{
		SessionId: resp.GetSearchSession().GetId(),
	})
	require.NoError(t, err)
	require.Equal(t, "searching", statusResp.GetSearchSession().GetStatus())

	_, err = srv.CancelSearch(ctx, &matchmakingv1.CancelSearchRequest{
		SessionId: resp.GetSearchSession().GetId(),
	})
	require.NoError(t, err)
}

func TestStartSearch_InvalidCriteriaRejected(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startDB(t, ctx)
	srv := searchTestServer(t, pool)
	ctx = ctxWithProfile(uuid.New())

	gameID := dotaGameID(t, srv, ctx)
	_, err := srv.StartSearch(ctx, &matchmakingv1.StartSearchRequest{
		GameId:       gameID,
		Mode:         "5v5 Ranked",
		CriteriaJson: `{"region":"na","self":{"role":"Carry","rank":"Herald"}}`,
	})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestStartSearch_DuplicateSearchRejected(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startDB(t, ctx)
	srv := searchTestServer(t, pool)
	profileID := uuid.New()
	ctx = ctxWithProfile(profileID)

	gameID := dotaGameID(t, srv, ctx)
	_, err := srv.StartSearch(ctx, &matchmakingv1.StartSearchRequest{
		GameId:       gameID,
		Mode:         "5v5 Ranked",
		CriteriaJson: validDotaCriteriaJSON(),
	})
	require.NoError(t, err)

	_, err = srv.StartSearch(ctx, &matchmakingv1.StartSearchRequest{
		GameId:       gameID,
		Mode:         "5v5 Ranked",
		CriteriaJson: validDotaCriteriaJSON(),
	})
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}

func TestStartSearch_RedisUnavailable(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startDB(t, ctx)
	badRedis := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	t.Cleanup(func() { _ = badRedis.Close() })
	srv := &MatchmakingGRPC{
		Games:    &store.GameStore{Pool: pool},
		Sessions: &store.SessionStore{Pool: pool},
		Queue:    &queue.RedisQueue{Client: badRedis, Prefix: "test"},
		Events:   mmevents.NoopPublisher{},
	}
	ctx = ctxWithProfile(uuid.New())
	gameID := dotaGameID(t, srv, ctx)
	_, err := srv.StartSearch(ctx, &matchmakingv1.StartSearchRequest{
		GameId:       gameID,
		Mode:         "5v5 Ranked",
		CriteriaJson: validDotaCriteriaJSON(),
	})
	require.Equal(t, codes.Unavailable, status.Code(err))
}

func TestGetSearchStatus_NotOwnerDenied(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startDB(t, ctx)
	srv := searchTestServer(t, pool)
	owner := uuid.New()
	ctxOwner := ctxWithProfile(owner)
	gameID := dotaGameID(t, srv, ctxOwner)
	started, err := srv.StartSearch(ctxOwner, &matchmakingv1.StartSearchRequest{
		GameId:       gameID,
		Mode:         "5v5 Ranked",
		CriteriaJson: validDotaCriteriaJSON(),
	})
	require.NoError(t, err)

	_, err = srv.GetSearchStatus(ctxWithProfile(uuid.New()), &matchmakingv1.GetSearchStatusRequest{
		SessionId: started.GetSearchSession().GetId(),
	})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestStartSearch_QueueUnavailable(t *testing.T) {
	t.Parallel()
	srv := &MatchmakingGRPC{
		Games:    &store.GameStore{},
		Sessions: &store.SessionStore{},
		Queue:    nil,
	}
	_, err := srv.StartSearch(ctxWithProfile(uuid.New()), &matchmakingv1.StartSearchRequest{
		GameId:       uuid.New().String(),
		Mode:         "5v5 Ranked",
		CriteriaJson: validDotaCriteriaJSON(),
	})
	require.Equal(t, codes.Unavailable, status.Code(err))
}
