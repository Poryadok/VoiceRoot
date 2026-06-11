package grpcsvc

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/matchmaking/internal/config"
	"voice/backend/matchmaking/internal/criteria"
	"voice/backend/matchmaking/internal/mmevents"
	"voice/backend/matchmaking/internal/queue"
	"voice/backend/matchmaking/internal/store"

	matchmakingv1 "voice.app/voice/matchmaking/v1"
)

func duoGameConfig() config.GameConfig {
	return config.GameConfig{
		Regions: []string{"eu"},
		Modes: []config.Mode{{
			Name:          "Duo",
			Slots:         2,
			PartySizeMin:  1,
			PartySizeMax:  1,
			RolesRequired: false,
			RankRequired:  false,
		}},
	}
}

type squadProvisioner interface {
	Provision(context.Context, uuid.UUID, []uuid.UUID) (voiceRoomID, chatID string, err error)
}

type stubSquadProvisioner struct {
	voiceRoomID string
	chatID      string
}

func (s *stubSquadProvisioner) Provision(_ context.Context, _ uuid.UUID, _ []uuid.UUID) (string, string, error) {
	if s.voiceRoomID == "" {
		s.voiceRoomID = "voice-room-1"
	}
	if s.chatID == "" {
		s.chatID = "chat-1"
	}
	return s.voiceRoomID, s.chatID, nil
}

func matchTestServer(t *testing.T, pool *pgxpool.Pool, provisioner squadProvisioner) *MatchmakingGRPC {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		_ = rdb.Close()
		mr.Close()
	})
	return &MatchmakingGRPC{
		Games:        &store.GameStore{Pool: pool},
		Sessions:     &store.SessionStore{Pool: pool},
		Matches:      &store.MatchStore{Pool: pool},
		Queue:        &queue.RedisQueue{Client: rdb, Prefix: "match-test"},
		Events:       mmevents.NoopPublisher{},
		Squad:        provisioner,
	}
}

func seedPendingDuoMatch(t *testing.T, ctx context.Context, srv *MatchmakingGRPC) (matchID string, profileA, profileB uuid.UUID) {
	t.Helper()
	game, err := srv.Games.Create(ctx, "Respond test", duoGameConfig(), uuid.New())
	require.NoError(t, err)
	timeout := time.Now().UTC().Add(30 * time.Minute)
	crit := criteria.MustMarshal(criteria.SearchCriteria{Region: "eu"})
	profileA = uuid.New()
	profileB = uuid.New()
	sessA, err := srv.Sessions.Create(ctx, store.CreateSessionParams{
		ProfileID: profileA,
		GameID:    game.ID,
		Mode:      "Duo",
		Criteria:  crit,
		TimeoutAt: timeout,
	})
	require.NoError(t, err)
	sessB, err := srv.Sessions.Create(ctx, store.CreateSessionParams{
		ProfileID: profileB,
		GameID:    game.ID,
		Mode:      "Duo",
		Criteria:  crit,
		TimeoutAt: timeout,
	})
	require.NoError(t, err)
	result, err := srv.Matches.CreateProposal(ctx, store.CreateProposalParams{
		GameID: game.ID,
		Mode:   "Duo",
		Region: "eu",
		Sessions: []store.ProposalSession{
			{SessionID: sessA.ID, ProfileID: profileA},
			{SessionID: sessB.ID, ProfileID: profileB},
		},
	})
	require.NoError(t, err)
	return result.Match.ID.String(), profileA, profileB
}

func TestRespondToMatch_AcceptAllActivatesMatch(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startDB(t, ctx)
	provisioner := &stubSquadProvisioner{}
	srv := matchTestServer(t, pool, provisioner)
	matchID, profileA, profileB := seedPendingDuoMatch(t, ctx, srv)

	ctxA := ctxWithProfile(profileA)
	ctxB := ctxWithProfile(profileB)

	respA, err := srv.RespondToMatch(ctxA, &matchmakingv1.RespondToMatchRequest{
		MatchId: matchID,
		Accept:  true,
	})
	require.NoError(t, err)
	require.Equal(t, "pending_accept", respA.GetMatch().GetStatus())

	respB, err := srv.RespondToMatch(ctxB, &matchmakingv1.RespondToMatchRequest{
		MatchId: matchID,
		Accept:  true,
	})
	require.NoError(t, err)
	require.Equal(t, "active", respB.GetMatch().GetStatus())
	require.NotEmpty(t, respB.GetMatch().GetVoiceRoomId())
	require.NotEmpty(t, respB.GetMatch().GetChatId())
	require.Equal(t, "matched", respB.GetSearchSession().GetStatus())

	got, err := srv.GetMatch(ctxA, &matchmakingv1.GetMatchRequest{MatchId: matchID})
	require.NoError(t, err)
	require.Equal(t, "active", got.GetMatch().GetStatus())
	require.Len(t, got.GetMatch().GetProfileIds(), 2)
}

func TestRespondToMatch_DeclineResetsOwnSearch(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startDB(t, ctx)
	srv := matchTestServer(t, pool, &stubSquadProvisioner{})
	matchID, profileA, _ := seedPendingDuoMatch(t, ctx, srv)

	resp, err := srv.RespondToMatch(ctxWithProfile(profileA), &matchmakingv1.RespondToMatchRequest{
		MatchId: matchID,
		Accept:  false,
	})
	require.NoError(t, err)
	require.Equal(t, "searching", resp.GetSearchSession().GetStatus())
	require.Empty(t, resp.GetSearchSession().GetMatchId())

	statusResp, err := srv.GetSearchStatus(ctxWithProfile(profileA), &matchmakingv1.GetSearchStatusRequest{
		SessionId: resp.GetSearchSession().GetId(),
	})
	require.NoError(t, err)
	require.Equal(t, "searching", statusResp.GetSearchSession().GetStatus())
	require.Empty(t, statusResp.GetSearchSession().GetMatchId())
}

func TestGetMatch_NotParticipantDenied(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startDB(t, ctx)
	srv := matchTestServer(t, pool, &stubSquadProvisioner{})
	matchID, _, _ := seedPendingDuoMatch(t, ctx, srv)

	_, err := srv.GetMatch(ctxWithProfile(uuid.New()), &matchmakingv1.GetMatchRequest{MatchId: matchID})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}
