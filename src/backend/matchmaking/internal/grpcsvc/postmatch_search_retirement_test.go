package grpcsvc

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	matchmakingv1 "voice.app/voice/matchmaking/v1"
	"voice/backend/matchmaking/internal/matcher"
	"voice/backend/matchmaking/internal/queue"
	"voice/backend/matchmaking/internal/store"
	searchtimeout "voice/backend/matchmaking/internal/timeout"
)

func completedMatchWithSearchReservations(t *testing.T) (*MatchmakingGRPC, store.Match, map[uuid.UUID]store.SearchSession) {
	t.Helper()
	ctx := context.Background()
	srv := ratingTestServer(t, startDB(t, ctx))
	matchText, a, b := activateDuoMatchViaGRPC(t, ctx, srv)
	matchID := uuid.MustParse(matchText)
	proposals, err := srv.Matches.ListProposals(ctx, matchID)
	require.NoError(t, err)
	original := make(map[uuid.UUID]store.SearchSession)
	for _, proposal := range proposals {
		sess, err := srv.Sessions.Get(ctx, proposal.SearchSessionID)
		require.NoError(t, err)
		original[sess.ProfileID] = sess
		// The matcher removes the FIFO member but retains its profile lock until
		// terminal recovery retires the reservation. Reproduce that real state.
		require.NoError(t, srv.Queue.AcquireLock(ctx, sess.ProfileID, sess.ID))
	}
	require.Len(t, original, 2)
	_, err = srv.BanFromMM(ctxWithProfile(a), &matchmakingv1.BanFromMMRequest{TargetProfileId: b.String()})
	require.NoError(t, err)
	_, err = srv.CompleteMatch(completeMatchContext(a), completeMatchRequest(matchText))
	require.NoError(t, err)
	var earlyReleases int
	require.NoError(t, srv.Matches.Pool.QueryRow(ctx, `SELECT count(*) FROM matchmaking_terminal_search_releases WHERE search_session_id IN (SELECT search_session_id FROM match_proposals WHERE match_id=$1)`, matchID).Scan(&earlyReleases))
	require.Zero(t, earlyReleases, "a non-final leave must not retire the still-active match's reservations")
	final := completeMatchRequest(matchText)
	completed, err := srv.CompleteMatch(completeMatchContext(b), final)
	require.NoError(t, err)
	require.Equal(t, store.MatchStatusCompleted, completed.GetMatch().GetStatus())
	// Exact completion replay must not mint another terminal generation/effect.
	_, err = srv.CompleteMatch(completeMatchContext(b), final)
	require.NoError(t, err)
	match, err := srv.Matches.Get(ctx, matchID)
	require.NoError(t, err)
	return srv, match, original
}

func restartCompletedProfileSearch(t *testing.T, srv *MatchmakingGRPC, match store.Match, profile uuid.UUID) uuid.UUID {
	t.Helper()
	started, err := srv.StartSearch(ctxWithProfileAccount(profile, uuid.New()), &matchmakingv1.StartSearchRequest{
		GameId: match.GameID.String(), Mode: match.Mode, CriteriaJson: `{"region":"eu"}`,
	})
	require.NoError(t, err, "a completed match must not retain this profile's old search reservation")
	require.Equal(t, store.SessionStatusSearching, started.GetSearchSession().GetStatus())
	return uuid.MustParse(started.GetSearchSession().GetId())
}

func TestCompletedMatchBothProfilesCanSearchAgainWithoutMatchingBannedPeer(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	ctx := context.Background()
	srv, match, original := completedMatchWithSearchReservations(t)
	sweeper := &searchtimeout.Sweeper{Sessions: srv.Sessions, Matches: srv.Matches, Queue: srv.Queue}
	require.NoError(t, sweeper.RunOnce(ctx), "the production recovery pass must retire completed reservations")
	var restarted []uuid.UUID
	for profile, old := range original {
		fresh := restartCompletedProfileSearch(t, srv, match, profile)
		require.NotEqual(t, old.ID, fresh)
		restarted = append(restarted, fresh)
	}
	worker := &matcher.Worker{Queue: srv.Queue, Sessions: srv.Sessions, Matches: srv.Matches, Games: srv.Games, Bans: srv.Bans}
	require.NoError(t, worker.RunOnce(ctx))
	for _, id := range restarted {
		sess, err := srv.Sessions.Get(ctx, id)
		require.NoError(t, err)
		require.Equal(t, store.SessionStatusSearching, sess.Status, "both banned peers stay queued concurrently")
		require.Nil(t, sess.MatchID, "the persisted peer ban must prevent their next pairing")
	}
}

func TestCompletedMatchTerminalReleaseIsInvisibleToLegacyRecoveryConsumer(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	ctx := context.Background()
	srv, match, _ := completedMatchWithSearchReservations(t)
	var visible int
	// This is the unfiltered legacy consumer selection at base 5af3e186,
	// matches.go:308-314. That consumer acknowledges matched releases as invalid.
	require.NoError(t, srv.Matches.Pool.QueryRow(ctx, `SELECT count(*) FROM matchmaking_match_recovery_effects WHERE match_id=$1 AND applied_at IS NULL`, match.ID).Scan(&visible))
	require.Zero(t, visible, "legacy consumers must never discover a new terminal-release obligation")
	require.NoError(t, srv.Matches.ApplyPendingRecoveryEffects(ctx, 100, func(context.Context, store.SearchSession, string) error {
		t.Fatal("a legacy recovery consumer must not project terminal-release obligations")
		return nil
	}))
	sweeper := &searchtimeout.Sweeper{Sessions: srv.Sessions, Matches: srv.Matches, Queue: srv.Queue}
	require.NoError(t, sweeper.RunOnce(ctx))
	for _, profile := range match.ProfileIDs() {
		restartCompletedProfileSearch(t, srv, match, profile)
	}
}

func TestCompletedMatchBinaryRollbackAndReupgradeCannotSwallowPendingReleases(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	ctx := context.Background()
	srv, match, _ := completedMatchWithSearchReservations(t)
	// ApplyPendingRecoveryEffects is byte-identical to the base legacy method.
	// Simulate an old-only interval, followed by mixed old/new recovery workers.
	legacy := &store.MatchStore{Pool: srv.Matches.Pool}
	for range 2 {
		require.NoError(t, legacy.ApplyPendingRecoveryEffects(ctx, 100, func(context.Context, store.SearchSession, string) error {
			t.Fatal("old worker discovered a terminal obligation during binary rollback")
			return nil
		}))
	}
	var pending int
	require.NoError(t, srv.Matches.Pool.QueryRow(ctx, `SELECT count(*) FROM matchmaking_terminal_search_releases`).Scan(&pending))
	require.Equal(t, 2, pending, "binary rollback retains every new-produced obligation")
	outage := errors.New("Redis unavailable during re-upgrade")
	require.ErrorIs(t, srv.Matches.ApplyPendingTerminalSearchReleases(ctx, 100, func(context.Context, store.TerminalSearchRelease) error {
		return outage
	}), outage)
	require.NoError(t, srv.Matches.Pool.QueryRow(ctx, `SELECT count(*) FROM matchmaking_terminal_search_releases`).Scan(&pending))
	require.Equal(t, 2, pending, "an unsuccessful re-upgrade projection cannot acknowledge cleanup")
	errCh := make(chan error, 1)
	go func() {
		errCh <- legacy.ApplyPendingRecoveryEffects(ctx, 100, func(context.Context, store.SearchSession, string) error {
			return errors.New("legacy worker discovered terminal routing")
		})
	}()
	resumed := &searchtimeout.Sweeper{Sessions: srv.Sessions, Matches: &store.MatchStore{Pool: srv.Matches.Pool}, Queue: srv.Queue}
	require.NoError(t, resumed.RunOnce(ctx))
	require.NoError(t, <-errCh)
	require.NoError(t, srv.Matches.Pool.QueryRow(ctx, `SELECT count(*) FROM matchmaking_terminal_search_releases`).Scan(&pending))
	require.Zero(t, pending, "successful Redis acknowledgements erase pending routing")
	for _, profile := range match.ProfileIDs() {
		restartCompletedProfileSearch(t, srv, match, profile)
	}
}

func TestCompletedMatchRetirementRetryPreservesNewSearchOwnerAndGeneration(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	ctx := context.Background()
	srv, match, original := completedMatchWithSearchReservations(t)
	interrupted := errors.New("lost projection acknowledgement after Redis success")
	var released store.TerminalSearchRelease
	err := srv.Matches.ApplyPendingTerminalSearchReleases(ctx, 100, func(projectionCtx context.Context, sess store.TerminalSearchRelease) error {
		require.NoError(t, srv.Queue.RecoverRelease(projectionCtx, queue.SearchRecovery{
			SpaceID: sess.SpaceID, GameID: sess.GameID, Mode: sess.Mode, Region: sess.Region,
			SessionID: sess.SessionID, ProfileID: sess.ProfileID, Generation: sess.Generation,
		}))
		released = sess
		return interrupted
	})
	require.ErrorIs(t, err, interrupted)
	var pending int
	require.NoError(t, srv.Matches.Pool.QueryRow(ctx, `SELECT count(*) FROM matchmaking_terminal_search_releases`).Scan(&pending))
	require.Equal(t, 2, pending, "completion replay retains exactly two unacknowledged, durably retryable obligations")
	fresh := restartCompletedProfileSearch(t, srv, match, released.ProfileID)
	// A fresh store/worker replays the old release after a new owner has started.
	resumed := &searchtimeout.Sweeper{
		Sessions: &store.SessionStore{Pool: srv.Sessions.Pool}, Matches: &store.MatchStore{Pool: srv.Matches.Pool}, Queue: srv.Queue,
	}
	require.NoError(t, resumed.RunOnce(ctx))
	owner, err := srv.Queue.Client.Get(ctx, fmt.Sprintf("%s:lock:profile:%s", srv.Queue.Prefix, released.ProfileID)).Result()
	require.NoError(t, err)
	require.Equal(t, fresh.String(), owner, "retrying an old release cannot delete a newer profile reservation")
	queued, err := srv.Queue.ListSessionIDsScoped(ctx, released.SpaceID, match.GameID, match.Mode, "eu", 0)
	require.NoError(t, err)
	require.Contains(t, queued, fresh)
	for profile, old := range original {
		if profile != released.ProfileID {
			restartCompletedProfileSearch(t, srv, match, profile)
		}
		err := srv.Queue.RecoverSearch(ctx, queue.SearchRecovery{
			SpaceID: old.SpaceID, GameID: old.GameID, Mode: old.Mode, Region: "eu", SessionID: old.ID,
			ProfileID: old.ProfileID, CreatedAt: old.CreatedAt, Generation: old.RecoveryGeneration,
		})
		require.ErrorIs(t, err, queue.ErrQueueGeneration, "a delayed older enqueue cannot resurrect a retired session")
		retained, err := srv.Sessions.Get(ctx, old.ID)
		require.NoError(t, err)
		require.Equal(t, store.SessionStatusMatched, retained.Status)
		require.Equal(t, old.MatchID, retained.MatchID, "history linkage must survive reservation cleanup")
		require.Equal(t, old.RecoveryGeneration+1, retained.RecoveryGeneration)
	}
	require.NoError(t, srv.Matches.Pool.QueryRow(ctx, `SELECT count(*) FROM matchmaking_terminal_search_releases`).Scan(&pending))
	require.Zero(t, pending)
}
