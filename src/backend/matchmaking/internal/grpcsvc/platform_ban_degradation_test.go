package grpcsvc

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/matchmaking/internal/mmevents"
	"voice/backend/matchmaking/internal/queue"
	"voice/backend/matchmaking/internal/store"

	matchmakingv1 "voice.app/voice/matchmaking/v1"
)

// TestStartSearch_BansNotConfigured documents fail-closed StartSearch when BanStore
// is unwired (moderation mm_ban cannot be enforced).
func TestStartSearch_BansNotConfigured(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startDB(t, ctx)
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		_ = rdb.Close()
		mr.Close()
	})
	srv := &MatchmakingGRPC{
		Games:    &store.GameStore{Pool: pool},
		Sessions: &store.SessionStore{Pool: pool},
		Queue:    &queue.RedisQueue{Client: rdb, Prefix: "ban-deg"},
		Bans:     nil,
		Events:   mmevents.NoopPublisher{},
	}
	gameID := dotaGameID(t, srv, ctxWithProfile(uuid.New()))
	_, err := srv.StartSearch(ctxWithProfileAccount(uuid.New(), uuid.New()), &matchmakingv1.StartSearchRequest{
		GameId:       gameID,
		Mode:         "5v5 Ranked",
		CriteriaJson: validDotaCriteriaJSON(),
	})
	require.Error(t, err)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}

// TestStartSearch_MissingAccountID documents that wired BanStore must not skip the
// platform ban check when x-voice-user-id is absent.
func TestStartSearch_MissingAccountID(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startDB(t, ctx)
	srv := searchTestServer(t, pool)
	gameID := dotaGameID(t, srv, ctxWithProfile(uuid.New()))
	_, err := srv.StartSearch(ctxWithProfile(uuid.New()), &matchmakingv1.StartSearchRequest{
		GameId:       gameID,
		Mode:         "5v5 Ranked",
		CriteriaJson: validDotaCriteriaJSON(),
	})
	require.Error(t, err)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
}
