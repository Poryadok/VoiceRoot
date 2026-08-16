package grpcsvc

import (
	"context"
	"encoding/json"
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

func TestRespondToMatch_DeclineCancelsDeclinerContinuesOtherSolo(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startDB(t, ctx)
	srv := matchTestServer(t, pool, &stubSquadProvisioner{})
	matchID, profileA, profileB := seedPendingDuoMatch(t, ctx, srv)

	resp, err := srv.RespondToMatch(ctxWithProfile(profileA), &matchmakingv1.RespondToMatchRequest{
		MatchId: matchID,
		Accept:  false,
	})
	require.NoError(t, err)
	require.Equal(t, "cancelled", resp.GetSearchSession().GetStatus())
	require.Equal(t, "abandoned", resp.GetMatch().GetStatus())

	declinerSess, err := srv.Sessions.Get(ctx, uuid.MustParse(resp.GetSearchSession().GetId()))
	require.NoError(t, err)
	require.Equal(t, store.SessionStatusCancelled, declinerSess.Status)

	// Other solo continues searching and is re-queued.
	proposals, err := srv.Matches.ListProposals(ctx, uuid.MustParse(matchID))
	require.NoError(t, err)
	var otherSessionID uuid.UUID
	for _, p := range proposals {
		if p.ProfileID == profileB {
			otherSessionID = p.SearchSessionID
			break
		}
	}
	require.NotEqual(t, uuid.Nil, otherSessionID)
	otherSess, err := srv.Sessions.Get(ctx, otherSessionID)
	require.NoError(t, err)
	require.Equal(t, store.SessionStatusSearching, otherSess.Status)
	require.Nil(t, otherSess.MatchID)

	queued, err := srv.Queue.ListSessionIDs(ctx, otherSess.GameID, otherSess.Mode, "eu", 0)
	require.NoError(t, err)
	require.Contains(t, queued, otherSessionID)
	require.NotContains(t, queued, declinerSess.ID)
}

func seedPendingCrossPartyMatch(t *testing.T, ctx context.Context, srv *MatchmakingGRPC) (
	matchID string,
	partyA1, partyA2, partyB1, partyB2 uuid.UUID,
	partyA, partyB uuid.UUID,
) {
	t.Helper()
	game, err := srv.Games.Create(ctx, "Cross-party decline", config.GameConfig{
		Regions: []string{"eu"},
		Modes: []config.Mode{{
			Name:          "2v2",
			Slots:         4,
			PartySizeMin:  1,
			PartySizeMax:  2,
			RolesRequired: false,
			RankRequired:  false,
		}},
	}, uuid.New())
	require.NoError(t, err)

	partyA = uuid.New()
	partyB = uuid.New()
	partyA1 = uuid.New()
	partyA2 = uuid.New()
	partyB1 = uuid.New()
	partyB2 = uuid.New()
	timeout := time.Now().UTC().Add(30 * time.Minute)
	crit := criteria.MustMarshal(criteria.SearchCriteria{Region: "eu"})

	membersA, err := json.Marshal([]string{partyA1.String(), partyA2.String()})
	require.NoError(t, err)
	membersB, err := json.Marshal([]string{partyB1.String(), partyB2.String()})
	require.NoError(t, err)
	_, err = srv.Sessions.Pool.Exec(ctx, `
		INSERT INTO parties (id, leader_profile_id, member_profile_ids, game_id, mode, criteria)
		VALUES
			($1, $2, $3::jsonb, $4, '2v2', $5::jsonb),
			($6, $7, $8::jsonb, $4, '2v2', $5::jsonb)
	`, partyA, partyA1, string(membersA), game.ID, crit,
		partyB, partyB1, string(membersB))
	require.NoError(t, err)

	mkSess := func(profileID, partyID uuid.UUID) store.SearchSession {
		t.Helper()
		pid := partyID
		sess, err := srv.Sessions.Create(ctx, store.CreateSessionParams{
			ProfileID: profileID,
			PartyID:   &pid,
			GameID:    game.ID,
			Mode:      "2v2",
			Criteria:  crit,
			TimeoutAt: timeout,
		})
		require.NoError(t, err)
		return sess
	}

	sessA1 := mkSess(partyA1, partyA)
	sessA2 := mkSess(partyA2, partyA)
	sessB1 := mkSess(partyB1, partyB)
	sessB2 := mkSess(partyB2, partyB)

	result, err := srv.Matches.CreateProposal(ctx, store.CreateProposalParams{
		GameID: game.ID,
		Mode:   "2v2",
		Region: "eu",
		Sessions: []store.ProposalSession{
			{SessionID: sessA1.ID, ProfileID: partyA1, PartyID: &partyA},
			{SessionID: sessA2.ID, ProfileID: partyA2, PartyID: &partyA},
			{SessionID: sessB1.ID, ProfileID: partyB1, PartyID: &partyB},
			{SessionID: sessB2.ID, ProfileID: partyB2, PartyID: &partyB},
		},
	})
	require.NoError(t, err)
	return result.Match.ID.String(), partyA1, partyA2, partyB1, partyB2, partyA, partyB
}

func sessionStatusByProfile(t *testing.T, ctx context.Context, srv *MatchmakingGRPC, matchID string, profileID uuid.UUID) store.SearchSession {
	t.Helper()
	proposals, err := srv.Matches.ListProposals(ctx, uuid.MustParse(matchID))
	require.NoError(t, err)
	for _, p := range proposals {
		if p.ProfileID == profileID {
			sess, err := srv.Sessions.Get(ctx, p.SearchSessionID)
			require.NoError(t, err)
			return sess
		}
	}
	t.Fatalf("no proposal for profile %s", profileID)
	return store.SearchSession{}
}

func TestRespondToMatch_ForeignPartyDecline_AcceptorsContinue(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startDB(t, ctx)
	srv := matchTestServer(t, pool, &stubSquadProvisioner{})
	matchID, a1, a2, b1, b2, _, _ := seedPendingCrossPartyMatch(t, ctx, srv)

	// Party A fully accepts first.
	_, err := srv.RespondToMatch(ctxWithProfile(a1), &matchmakingv1.RespondToMatchRequest{MatchId: matchID, Accept: true})
	require.NoError(t, err)
	_, err = srv.RespondToMatch(ctxWithProfile(a2), &matchmakingv1.RespondToMatchRequest{MatchId: matchID, Accept: true})
	require.NoError(t, err)

	// Foreign party B declines.
	resp, err := srv.RespondToMatch(ctxWithProfile(b1), &matchmakingv1.RespondToMatchRequest{MatchId: matchID, Accept: false})
	require.NoError(t, err)
	require.Equal(t, "cancelled", resp.GetSearchSession().GetStatus())
	require.Equal(t, "abandoned", resp.GetMatch().GetStatus())

	require.Equal(t, store.SessionStatusSearching, sessionStatusByProfile(t, ctx, srv, matchID, a1).Status)
	require.Equal(t, store.SessionStatusSearching, sessionStatusByProfile(t, ctx, srv, matchID, a2).Status)
	require.Equal(t, store.SessionStatusCancelled, sessionStatusByProfile(t, ctx, srv, matchID, b1).Status)
	require.Equal(t, store.SessionStatusCancelled, sessionStatusByProfile(t, ctx, srv, matchID, b2).Status)

	a1Sess := sessionStatusByProfile(t, ctx, srv, matchID, a1)
	queued, err := srv.Queue.ListSessionIDs(ctx, a1Sess.GameID, a1Sess.Mode, "eu", 0)
	require.NoError(t, err)
	require.Contains(t, queued, a1Sess.ID)
	require.Contains(t, queued, sessionStatusByProfile(t, ctx, srv, matchID, a2).ID)
	require.NotContains(t, queued, sessionStatusByProfile(t, ctx, srv, matchID, b1).ID)
	require.NotContains(t, queued, sessionStatusByProfile(t, ctx, srv, matchID, b2).ID)
}

func TestRespondToMatch_OwnPartyDecline_ResetsWholeParty(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startDB(t, ctx)
	srv := matchTestServer(t, pool, &stubSquadProvisioner{})
	matchID, a1, a2, b1, b2, _, _ := seedPendingCrossPartyMatch(t, ctx, srv)

	// One member of party A declines → whole party A cancelled; party B continues.
	resp, err := srv.RespondToMatch(ctxWithProfile(a1), &matchmakingv1.RespondToMatchRequest{MatchId: matchID, Accept: false})
	require.NoError(t, err)
	require.Equal(t, "cancelled", resp.GetSearchSession().GetStatus())
	require.Equal(t, "abandoned", resp.GetMatch().GetStatus())

	require.Equal(t, store.SessionStatusCancelled, sessionStatusByProfile(t, ctx, srv, matchID, a1).Status)
	require.Equal(t, store.SessionStatusCancelled, sessionStatusByProfile(t, ctx, srv, matchID, a2).Status)
	require.Equal(t, store.SessionStatusSearching, sessionStatusByProfile(t, ctx, srv, matchID, b1).Status)
	require.Equal(t, store.SessionStatusSearching, sessionStatusByProfile(t, ctx, srv, matchID, b2).Status)

	b1Sess := sessionStatusByProfile(t, ctx, srv, matchID, b1)
	queued, err := srv.Queue.ListSessionIDs(ctx, b1Sess.GameID, b1Sess.Mode, "eu", 0)
	require.NoError(t, err)
	require.Contains(t, queued, b1Sess.ID)
	require.Contains(t, queued, sessionStatusByProfile(t, ctx, srv, matchID, b2).ID)
	require.NotContains(t, queued, sessionStatusByProfile(t, ctx, srv, matchID, a1).ID)
	require.NotContains(t, queued, sessionStatusByProfile(t, ctx, srv, matchID, a2).ID)
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
