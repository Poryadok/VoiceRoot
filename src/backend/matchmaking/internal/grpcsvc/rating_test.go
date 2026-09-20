package grpcsvc

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/matchmaking/internal/criteria"
	"voice/backend/matchmaking/internal/mmevents"
	"voice/backend/matchmaking/internal/store"

	matchmakingv1 "voice.app/voice/matchmaking/v1"
)

type recordingPlayerBannedPublisher struct {
	mmevents.NoopPublisher
	events []mmevents.PlayerBannedEvent
}

type recordingMatchCompletedPublisher struct {
	mmevents.NoopPublisher
	mu     sync.Mutex
	events []mmevents.MatchCompletedEvent
}

type recordingSquadCleanup struct {
	mu       sync.Mutex
	matchIDs []uuid.UUID
}

func (c *recordingSquadCleanup) Cleanup(_ context.Context, matchID uuid.UUID) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.matchIDs = append(c.matchIDs, matchID)
	return nil
}

func (c *recordingSquadCleanup) MatchIDs() []uuid.UUID {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]uuid.UUID(nil), c.matchIDs...)
}

func (p *recordingMatchCompletedPublisher) PublishMatchCompleted(_ context.Context, event mmevents.MatchCompletedEvent) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.events = append(p.events, event)
	return nil
}

func (p *recordingMatchCompletedPublisher) Events() []mmevents.MatchCompletedEvent {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]mmevents.MatchCompletedEvent(nil), p.events...)
}

func (p *recordingPlayerBannedPublisher) PublishPlayerBanned(_ context.Context, event mmevents.PlayerBannedEvent) error {
	p.events = append(p.events, event)
	return nil
}

func seedPendingDuoMatchForGame(t *testing.T, ctx context.Context, srv *MatchmakingGRPC, gameID, profileB uuid.UUID) (matchID string, profileA uuid.UUID) {
	t.Helper()
	profileA = uuid.New()
	timeout := time.Now().UTC().Add(30 * time.Minute)
	crit := criteria.MustMarshal(criteria.SearchCriteria{Region: "eu"})
	sessA, err := srv.Sessions.Create(ctx, store.CreateSessionParams{
		ProfileID: profileA,
		GameID:    gameID,
		Mode:      "Duo",
		Criteria:  crit,
		TimeoutAt: timeout,
	})
	require.NoError(t, err)
	sessB, err := srv.Sessions.Create(ctx, store.CreateSessionParams{
		ProfileID: profileB,
		GameID:    gameID,
		Mode:      "Duo",
		Criteria:  crit,
		TimeoutAt: timeout,
	})
	require.NoError(t, err)
	result, err := srv.Matches.CreateProposal(ctx, store.CreateProposalParams{
		GameID: gameID,
		Mode:   "Duo",
		Region: "eu",
		Sessions: []store.ProposalSession{
			{SessionID: sessA.ID, ProfileID: profileA},
			{SessionID: sessB.ID, ProfileID: profileB},
		},
	})
	require.NoError(t, err)
	return result.Match.ID.String(), profileA
}

func ratingTestServer(t *testing.T, pool *pgxpool.Pool) *MatchmakingGRPC {
	t.Helper()
	srv := matchTestServer(t, pool, &stubSquadProvisioner{})
	srv.Ratings = &store.RatingStore{Pool: pool}
	srv.Bans = &store.BanStore{Pool: pool}
	return srv
}

func activateDuoMatchViaGRPC(t *testing.T, ctx context.Context, srv *MatchmakingGRPC) (matchID string, profileA, profileB uuid.UUID) {
	t.Helper()
	matchID, profileA, profileB = seedPendingDuoMatch(t, ctx, srv)

	_, err := srv.RespondToMatch(ctxWithProfile(profileA), &matchmakingv1.RespondToMatchRequest{
		MatchId: matchID,
		Accept:  true,
	})
	require.NoError(t, err)
	_, err = srv.RespondToMatch(ctxWithProfile(profileB), &matchmakingv1.RespondToMatchRequest{
		MatchId: matchID,
		Accept:  true,
	})
	require.NoError(t, err)
	return matchID, profileA, profileB
}

func TestCompleteMatch_FirstLeaveKeepsActive(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startDB(t, ctx)
	srv := ratingTestServer(t, pool)
	matchID, profileA, _ := activateDuoMatchViaGRPC(t, ctx, srv)

	resp, err := srv.CompleteMatch(ctxWithProfile(profileA), &matchmakingv1.CompleteMatchRequest{
		MatchId: matchID,
	})
	require.NoError(t, err)
	require.Equal(t, "active", resp.GetMatch().GetStatus())
}

func TestCompleteMatch_AllLeftSetsCompleted(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startDB(t, ctx)
	srv := ratingTestServer(t, pool)
	matchID, profileA, profileB := activateDuoMatchViaGRPC(t, ctx, srv)

	_, err := srv.CompleteMatch(ctxWithProfile(profileA), &matchmakingv1.CompleteMatchRequest{MatchId: matchID})
	require.NoError(t, err)

	resp, err := srv.CompleteMatch(ctxWithProfile(profileB), &matchmakingv1.CompleteMatchRequest{MatchId: matchID})
	require.NoError(t, err)
	require.Equal(t, "completed", resp.GetMatch().GetStatus())
}

func TestCompleteMatch_ConcurrentFinalRetryPublishesOnce(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startDB(t, ctx)
	srv := ratingTestServer(t, pool)
	publisher := &recordingMatchCompletedPublisher{}
	srv.Events = publisher
	cleanup := &recordingSquadCleanup{}
	srv.SquadCleanup = cleanup
	matchID, profileA, profileB := activateDuoMatchViaGRPC(t, ctx, srv)

	_, err := srv.CompleteMatch(ctxWithProfile(profileA), &matchmakingv1.CompleteMatchRequest{MatchId: matchID})
	require.NoError(t, err)

	conn, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer conn.Release()
	tx, err := conn.Begin(ctx)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, "SELECT 1 FROM matches WHERE id = $1 FOR UPDATE", matchID)
	require.NoError(t, err)

	start := make(chan struct{})
	errs := make(chan error, 2)
	request := &matchmakingv1.CompleteMatchRequest{MatchId: matchID}
	for range 2 {
		go func() {
			<-start
			_, callErr := srv.CompleteMatch(ctxWithProfile(profileB), request)
			errs <- callErr
		}()
	}
	close(start)

	deadline := time.Now().Add(5 * time.Second)
	for {
		var waiting int
		err = pool.QueryRow(ctx, `
			SELECT COUNT(*)
			FROM pg_stat_activity
			WHERE datname = current_database()
			  AND wait_event_type = 'Lock'
			  AND query LIKE '%FOR UPDATE%'
		`).Scan(&waiting)
		require.NoError(t, err)
		if waiting >= 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("both final retries did not reach the match row lock")
		}
		time.Sleep(10 * time.Millisecond)
	}
	require.NoError(t, tx.Commit(ctx))

	require.NoError(t, <-errs)
	require.NoError(t, <-errs)
	require.Len(t, publisher.Events(), 1, "duplicate final CompleteMatch retries must emit mm.match_completed once")
	require.Equal(t, []uuid.UUID{uuid.MustParse(matchID)}, cleanup.MatchIDs(), "racing final retries must clean the fixture squad once")
}

// TestCompleteMatch_FinalLeaveCleansFixtureSquadOnce freezes the pre-A2
// provider contract: the durable active-to-completed transition owns a single
// cleanup call. The fixture is intentionally transport- and roster-event-free;
// a later A3 acceptance slice wires the real temporary chat and voice context.
func TestCompleteMatch_FinalLeaveCleansFixtureSquadOnce(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startDB(t, ctx)
	srv := ratingTestServer(t, pool)
	cleanup := &recordingSquadCleanup{}
	srv.SquadCleanup = cleanup
	matchID, profileA, profileB := activateDuoMatchViaGRPC(t, ctx, srv)

	_, err := srv.CompleteMatch(ctxWithProfile(profileA), &matchmakingv1.CompleteMatchRequest{MatchId: matchID})
	require.NoError(t, err)
	_, err = srv.CompleteMatch(ctxWithProfile(profileB), &matchmakingv1.CompleteMatchRequest{MatchId: matchID})
	require.NoError(t, err)
	_, err = srv.CompleteMatch(ctxWithProfile(profileB), &matchmakingv1.CompleteMatchRequest{MatchId: matchID})
	require.NoError(t, err, "a duplicate final leave is an idempotent retry")

	require.Equal(t, []uuid.UUID{uuid.MustParse(matchID)}, cleanup.MatchIDs(), "only the transition that completes the match cleans the fixture squad")
}

func TestRateMatch_PersistsStarsForTeammate(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startDB(t, ctx)
	srv := ratingTestServer(t, pool)
	matchID, profileA, profileB := activateDuoMatchViaGRPC(t, ctx, srv)

	_, err := srv.CompleteMatch(ctxWithProfile(profileA), &matchmakingv1.CompleteMatchRequest{MatchId: matchID})
	require.NoError(t, err)
	_, err = srv.CompleteMatch(ctxWithProfile(profileB), &matchmakingv1.CompleteMatchRequest{MatchId: matchID})
	require.NoError(t, err)

	_, err = srv.RateMatch(ctxWithProfile(profileA), &matchmakingv1.RateMatchRequest{
		MatchId:        matchID,
		RatedProfileId: profileB.String(),
		Stars:          5,
	})
	require.NoError(t, err)
}

func TestRateMatch_AllowsParticipantToRateAfterTheirOwnLeave(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startDB(t, ctx)
	srv := ratingTestServer(t, pool)
	matchID, profileA, profileB := activateDuoMatchViaGRPC(t, ctx, srv)

	left, err := srv.CompleteMatch(ctxWithProfile(profileA), &matchmakingv1.CompleteMatchRequest{
		MatchId: matchID,
	})
	require.NoError(t, err)
	require.Equal(t, store.MatchStatusActive, left.GetMatch().GetStatus())

	_, err = srv.RateMatch(ctxWithProfile(profileA), &matchmakingv1.RateMatchRequest{
		MatchId:        matchID,
		RatedProfileId: profileB.String(),
		Stars:          5,
	})
	require.NoError(t, err, "a participant must be able to rate teammates when they leave the squad")

	var ratingRows, totalRatings int
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM match_ratings
		WHERE match_id = $1 AND rater_profile_id = $2 AND rated_profile_id = $3
	`, matchID, profileA, profileB).Scan(&ratingRows))
	require.Equal(t, 1, ratingRows)
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT total_ratings_received FROM player_ratings WHERE profile_id = $1
	`, profileB).Scan(&totalRatings))
	require.Equal(t, 1, totalRatings)
}

func TestRateMatch_DuplicateRejected(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startDB(t, ctx)
	srv := ratingTestServer(t, pool)
	matchID, profileA, profileB := activateDuoMatchViaGRPC(t, ctx, srv)

	_, err := srv.CompleteMatch(ctxWithProfile(profileA), &matchmakingv1.CompleteMatchRequest{MatchId: matchID})
	require.NoError(t, err)
	_, err = srv.CompleteMatch(ctxWithProfile(profileB), &matchmakingv1.CompleteMatchRequest{MatchId: matchID})
	require.NoError(t, err)

	req := &matchmakingv1.RateMatchRequest{
		MatchId:        matchID,
		RatedProfileId: profileB.String(),
		Stars:          4,
	}
	_, err = srv.RateMatch(ctxWithProfile(profileA), req)
	require.NoError(t, err)

	_, err = srv.RateMatch(ctxWithProfile(profileA), req)
	require.Equal(t, codes.AlreadyExists, status.Code(err))
}

func TestRateMatch_ExplicitSkipDoesNotPersistOrBlockScore(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startDB(t, ctx)
	srv := ratingTestServer(t, pool)
	matchID, profileA, profileB := activateDuoMatchViaGRPC(t, ctx, srv)

	_, err := srv.CompleteMatch(ctxWithProfile(profileA), &matchmakingv1.CompleteMatchRequest{MatchId: matchID})
	require.NoError(t, err)
	_, err = srv.CompleteMatch(ctxWithProfile(profileB), &matchmakingv1.CompleteMatchRequest{MatchId: matchID})
	require.NoError(t, err)

	skip := &matchmakingv1.RateMatchRequest{
		MatchId:        matchID,
		RatedProfileId: profileB.String(),
		Skip:           true,
	}
	_, err = srv.RateMatch(ctxWithProfile(profileA), skip)
	require.NoError(t, err)

	var ratingRows, aggregateRows int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM match_ratings WHERE match_id = $1 AND rater_profile_id = $2 AND rated_profile_id = $3`,
		matchID, profileA, profileB,
	).Scan(&ratingRows))
	require.Zero(t, ratingRows)
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM player_ratings WHERE profile_id = $1`, profileB,
	).Scan(&aggregateRows))
	require.Zero(t, aggregateRows)

	score := &matchmakingv1.RateMatchRequest{
		MatchId:        matchID,
		RatedProfileId: profileB.String(),
		Stars:          4,
	}
	_, err = srv.RateMatch(ctxWithProfile(profileA), score)
	require.NoError(t, err)

	_, err = srv.RateMatch(ctxWithProfile(profileA), score)
	require.Equal(t, codes.AlreadyExists, status.Code(err))
}

func TestRateMatch_ZeroStarsWithoutSkipRejected(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startDB(t, ctx)
	srv := ratingTestServer(t, pool)
	matchID, profileA, profileB := activateDuoMatchViaGRPC(t, ctx, srv)

	_, err := srv.CompleteMatch(ctxWithProfile(profileA), &matchmakingv1.CompleteMatchRequest{MatchId: matchID})
	require.NoError(t, err)
	_, err = srv.CompleteMatch(ctxWithProfile(profileB), &matchmakingv1.CompleteMatchRequest{MatchId: matchID})
	require.NoError(t, err)

	_, err = srv.RateMatch(ctxWithProfile(profileA), &matchmakingv1.RateMatchRequest{
		MatchId:        matchID,
		RatedProfileId: profileB.String(),
	})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestRateMatch_SkipWithScoreRejected(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startDB(t, ctx)
	srv := ratingTestServer(t, pool)
	matchID, profileA, profileB := activateDuoMatchViaGRPC(t, ctx, srv)

	_, err := srv.CompleteMatch(ctxWithProfile(profileA), &matchmakingv1.CompleteMatchRequest{MatchId: matchID})
	require.NoError(t, err)
	_, err = srv.CompleteMatch(ctxWithProfile(profileB), &matchmakingv1.CompleteMatchRequest{MatchId: matchID})
	require.NoError(t, err)

	_, err = srv.RateMatch(ctxWithProfile(profileA), &matchmakingv1.RateMatchRequest{
		MatchId:        matchID,
		RatedProfileId: profileB.String(),
		Stars:          4,
		Skip:           true,
	})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestGetPlayerRating_ReturnsAggregate(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startDB(t, ctx)
	srv := ratingTestServer(t, pool)
	matchID, profileA, profileB := activateDuoMatchViaGRPC(t, ctx, srv)

	_, err := srv.CompleteMatch(ctxWithProfile(profileA), &matchmakingv1.CompleteMatchRequest{MatchId: matchID})
	require.NoError(t, err)
	_, err = srv.CompleteMatch(ctxWithProfile(profileB), &matchmakingv1.CompleteMatchRequest{MatchId: matchID})
	require.NoError(t, err)

	_, err = srv.RateMatch(ctxWithProfile(profileA), &matchmakingv1.RateMatchRequest{
		MatchId:        matchID,
		RatedProfileId: profileB.String(),
		Stars:          5,
	})
	require.NoError(t, err)

	got, err := srv.GetMatch(ctxWithProfile(profileB), &matchmakingv1.GetMatchRequest{MatchId: matchID})
	require.NoError(t, err)
	gameID := got.GetMatch().GetGameId()

	resp, err := srv.GetPlayerRating(ctx, &matchmakingv1.GetPlayerRatingRequest{
		ProfileId: profileB.String(),
		GameId:    gameID,
	})
	require.NoError(t, err)
	require.Equal(t, profileB.String(), resp.GetPlayerRating().GetProfileId())
	require.Equal(t, gameID, resp.GetPlayerRating().GetGameId())
	require.Equal(t, 5.0, resp.GetPlayerRating().GetRatingValue())
	require.Equal(t, int32(1), resp.GetPlayerRating().GetGamesPlayed())
}

func TestGetPlayerRating_GamesPlayedCountsCompletedMatchesNotRatings(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startDB(t, ctx)
	srv := ratingTestServer(t, pool)

	game, err := srv.Games.Create(ctx, "Games played test", duoGameConfig(), uuid.New())
	require.NoError(t, err)
	profileB := uuid.New()
	match1, profileA1 := seedPendingDuoMatchForGame(t, ctx, srv, game.ID, profileB)
	_, err = srv.RespondToMatch(ctxWithProfile(profileA1), &matchmakingv1.RespondToMatchRequest{MatchId: match1, Accept: true})
	require.NoError(t, err)
	_, err = srv.RespondToMatch(ctxWithProfile(profileB), &matchmakingv1.RespondToMatchRequest{MatchId: match1, Accept: true})
	require.NoError(t, err)
	_, err = srv.CompleteMatch(ctxWithProfile(profileA1), &matchmakingv1.CompleteMatchRequest{MatchId: match1})
	require.NoError(t, err)
	_, err = srv.CompleteMatch(ctxWithProfile(profileB), &matchmakingv1.CompleteMatchRequest{MatchId: match1})
	require.NoError(t, err)

	_, err = srv.RateMatch(ctxWithProfile(profileA1), &matchmakingv1.RateMatchRequest{
		MatchId: match1, RatedProfileId: profileB.String(), Stars: 5,
	})
	require.NoError(t, err)

	match2, profileA2 := seedPendingDuoMatchForGame(t, ctx, srv, game.ID, profileB)
	_, err = srv.RespondToMatch(ctxWithProfile(profileA2), &matchmakingv1.RespondToMatchRequest{MatchId: match2, Accept: true})
	require.NoError(t, err)
	_, err = srv.RespondToMatch(ctxWithProfile(profileB), &matchmakingv1.RespondToMatchRequest{MatchId: match2, Accept: true})
	require.NoError(t, err)
	_, err = srv.CompleteMatch(ctxWithProfile(profileA2), &matchmakingv1.CompleteMatchRequest{MatchId: match2})
	require.NoError(t, err)
	_, err = srv.CompleteMatch(ctxWithProfile(profileB), &matchmakingv1.CompleteMatchRequest{MatchId: match2})
	require.NoError(t, err)
	// A repeated leave after completion must not create another completed match.
	_, err = srv.CompleteMatch(ctxWithProfile(profileB), &matchmakingv1.CompleteMatchRequest{MatchId: match2})
	require.NoError(t, err)

	resp, err := srv.GetPlayerRating(ctx, &matchmakingv1.GetPlayerRatingRequest{
		ProfileId: profileB.String(), GameId: game.ID.String(),
	})
	require.NoError(t, err)
	require.Equal(t, 5.0, resp.GetPlayerRating().GetRatingValue())
	require.Equal(t, int32(2), resp.GetPlayerRating().GetGamesPlayed())
}

func TestUnbanFromMM_ClearsPeerBan(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startDB(t, ctx)
	srv := ratingTestServer(t, pool)
	banner := uuid.New()
	target := uuid.New()

	_, err := srv.BanFromMM(ctxWithProfile(banner), &matchmakingv1.BanFromMMRequest{
		TargetProfileId: target.String(),
	})
	require.NoError(t, err)

	_, err = srv.UnbanFromMM(ctxWithProfile(banner), &matchmakingv1.UnbanFromMMRequest{
		TargetProfileId: target.String(),
	})
	require.NoError(t, err)

	statusResp, err := srv.GetMMBanStatus(ctxWithProfile(banner), &matchmakingv1.GetMMBanStatusRequest{
		TargetProfileId: target.String(),
	})
	require.NoError(t, err)
	require.False(t, statusResp.GetMmBanStatus().GetBanned())
}

func TestBanFromMM_UsesTargetProfileID(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startDB(t, ctx)
	srv := ratingTestServer(t, pool)
	banner := uuid.New()
	target := uuid.New()

	_, err := srv.BanFromMM(ctxWithProfile(banner), &matchmakingv1.BanFromMMRequest{
		TargetProfileId: target.String(),
	})
	require.NoError(t, err)

	statusResp, err := srv.GetMMBanStatus(ctxWithProfile(banner), &matchmakingv1.GetMMBanStatusRequest{
		TargetProfileId: target.String(),
	})
	require.NoError(t, err)
	require.True(t, statusResp.GetMmBanStatus().GetBanned())
}

func TestBanFromMM_PublishesOnceAfterNewPeerBan(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startDB(t, ctx)
	srv := ratingTestServer(t, pool)
	publisher := &recordingPlayerBannedPublisher{}
	srv.Events = publisher
	banner := uuid.New()
	target := uuid.New()
	reason := " toxic "
	req := &matchmakingv1.BanFromMMRequest{TargetProfileId: target.String(), Reason: &reason}

	_, err := srv.BanFromMM(ctxWithProfile(banner), req)
	require.NoError(t, err)
	require.Equal(t, []mmevents.PlayerBannedEvent{{
		ProfileID: target.String(),
		Reason:    "toxic",
	}}, publisher.events)

	_, err = srv.BanFromMM(ctxWithProfile(banner), req)
	require.NoError(t, err)
	require.Len(t, publisher.events, 1, "idempotent ban must not create another event")
}

func TestBanFromMM_FailuresDoNotPublish(t *testing.T) {
	t.Parallel()
	banner := uuid.New()
	target := uuid.New()

	t.Run("invalid target", func(t *testing.T) {
		publisher := &recordingPlayerBannedPublisher{}
		srv := &MatchmakingGRPC{Bans: &store.BanStore{}, Events: publisher}
		_, err := srv.BanFromMM(ctxWithProfile(banner), &matchmakingv1.BanFromMMRequest{TargetProfileId: "not-a-uuid"})
		require.Equal(t, codes.InvalidArgument, status.Code(err))
		require.Empty(t, publisher.events)
	})

	t.Run("missing actor", func(t *testing.T) {
		publisher := &recordingPlayerBannedPublisher{}
		srv := &MatchmakingGRPC{Bans: &store.BanStore{}, Events: publisher}
		_, err := srv.BanFromMM(context.Background(), &matchmakingv1.BanFromMMRequest{TargetProfileId: target.String()})
		require.Equal(t, codes.Unauthenticated, status.Code(err))
		require.Empty(t, publisher.events)
	})

	t.Run("store failure", func(t *testing.T) {
		publisher := &recordingPlayerBannedPublisher{}
		srv := &MatchmakingGRPC{Bans: &store.BanStore{}, Events: publisher}
		_, err := srv.BanFromMM(ctxWithProfile(banner), &matchmakingv1.BanFromMMRequest{TargetProfileId: target.String()})
		require.Equal(t, codes.Internal, status.Code(err))
		require.Empty(t, publisher.events)
	})
}

func TestRateMatch_ActiveMatchRejected(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startDB(t, ctx)
	srv := ratingTestServer(t, pool)
	matchID, profileA, profileB := activateDuoMatchViaGRPC(t, ctx, srv)

	_, err := srv.RateMatch(ctxWithProfile(profileA), &matchmakingv1.RateMatchRequest{
		MatchId:        matchID,
		RatedProfileId: profileB.String(),
		Stars:          3,
	})
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}

func TestRateMatch_NotParticipantDenied(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startDB(t, ctx)
	srv := ratingTestServer(t, pool)
	matchID, _, profileB := activateDuoMatchViaGRPC(t, ctx, srv)

	_, err := srv.RateMatch(ctxWithProfile(uuid.New()), &matchmakingv1.RateMatchRequest{
		MatchId:        matchID,
		RatedProfileId: profileB.String(),
		Stars:          3,
	})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestCompleteMatch_NotParticipantDenied(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startDB(t, ctx)
	srv := ratingTestServer(t, pool)
	matchID, _, _ := activateDuoMatchViaGRPC(t, ctx, srv)

	_, err := srv.CompleteMatch(ctxWithProfile(uuid.New()), &matchmakingv1.CompleteMatchRequest{
		MatchId: matchID,
	})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}
