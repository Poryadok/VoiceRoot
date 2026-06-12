package store

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func TestMatchStore_ListHistoryForProfile_ReturnsActiveAndCompletedOnly(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := StartMatchmakingDBForStoreTest(t, ctx)
	ApplyMatchmakingMigrationsForStoreTest(t, ctx, pool)
	matches := &MatchStore{Pool: pool}

	activeID, profileA, _ := seedActiveDuoMatch(t, ctx, pool)
	pendingID, _, _ := seedPendingDuoMatch(t, ctx, pool)

	err := matches.AbandonMatch(ctx, pendingID)
	require.NoError(t, err)

	res, err := matches.ListHistoryForProfile(ctx, ListMatchHistoryParams{
		ProfileID: profileA,
		PageSize:  50,
	})
	require.NoError(t, err)
	require.Len(t, res.Matches, 1)
	require.Equal(t, activeID, res.Matches[0].ID)
	require.Equal(t, MatchStatusActive, res.Matches[0].Status)
}

func TestMatchStore_ListHistoryForProfile_IncludesCompleted(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := StartMatchmakingDBForStoreTest(t, ctx)
	ApplyMatchmakingMigrationsForStoreTest(t, ctx, pool)
	matches := &MatchStore{Pool: pool}

	matchID, profileA, profileB := seedActiveDuoMatch(t, ctx, pool)
	_, err := matches.CompleteMatchLeave(ctx, matchID, profileA)
	require.NoError(t, err)
	_, err = matches.CompleteMatchLeave(ctx, matchID, profileB)
	require.NoError(t, err)

	res, err := matches.ListHistoryForProfile(ctx, ListMatchHistoryParams{
		ProfileID: profileA,
		PageSize:  50,
	})
	require.NoError(t, err)
	require.Len(t, res.Matches, 1)
	require.Equal(t, MatchStatusCompleted, res.Matches[0].Status)
}

func TestMatchStore_ListHistoryForProfile_ExcludesNonParticipant(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := StartMatchmakingDBForStoreTest(t, ctx)
	ApplyMatchmakingMigrationsForStoreTest(t, ctx, pool)
	matches := &MatchStore{Pool: pool}

	_, _, _ = seedActiveDuoMatch(t, ctx, pool)

	res, err := matches.ListHistoryForProfile(ctx, ListMatchHistoryParams{
		ProfileID: uuid.New(),
		PageSize:  50,
	})
	require.NoError(t, err)
	require.Empty(t, res.Matches)
}

func TestMatchStore_ListHistoryForProfile_CursorPagination(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := StartMatchmakingDBForStoreTest(t, ctx)
	ApplyMatchmakingMigrationsForStoreTest(t, ctx, pool)
	matches := &MatchStore{Pool: pool}

	firstID, profileA, profileB := seedActiveDuoMatch(t, ctx, pool)
	older := time.Now().UTC().Add(-time.Hour)
	_, err := pool.Exec(ctx, `UPDATE matches SET created_at = $2 WHERE id = $1`, firstID, older)
	require.NoError(t, err)
	secondID := seedAnotherActiveDuoMatch(t, ctx, pool, profileA, profileB)

	page1, err := matches.ListHistoryForProfile(ctx, ListMatchHistoryParams{
		ProfileID: profileA,
		PageSize:  1,
	})
	require.NoError(t, err)
	require.Len(t, page1.Matches, 1)
	require.Equal(t, secondID, page1.Matches[0].ID)
	require.NotEmpty(t, page1.NextCursor)

	page2, err := matches.ListHistoryForProfile(ctx, ListMatchHistoryParams{
		ProfileID: profileA,
		PageSize:  1,
		Cursor:    page1.NextCursor,
	})
	require.NoError(t, err)
	require.Len(t, page2.Matches, 1)
	require.Equal(t, firstID, page2.Matches[0].ID)
	require.Empty(t, page2.NextCursor)
}

func seedAnotherActiveDuoMatch(t *testing.T, ctx context.Context, pool *pgxpool.Pool, profileA, profileB uuid.UUID) uuid.UUID {
	t.Helper()
	games := &GameStore{Pool: pool}
	g, err := games.List(ctx, ListGamesParams{PageSize: 1, Status: StatusActive})
	require.NoError(t, err)
	require.NotEmpty(t, g.Games)

	sessions := &SessionStore{Pool: pool}
	matches := &MatchStore{Pool: pool}
	timeout := time.Now().UTC().Add(30 * time.Minute)
	criteria := `{"region":"eu"}`

	sessA, err := sessions.Create(ctx, CreateSessionParams{
		ProfileID: profileA,
		GameID:    g.Games[0].ID,
		Mode:      "Duo",
		Criteria:  criteria,
		TimeoutAt: timeout,
	})
	require.NoError(t, err)
	sessB, err := sessions.Create(ctx, CreateSessionParams{
		ProfileID: profileB,
		GameID:    g.Games[0].ID,
		Mode:      "Duo",
		Criteria:  criteria,
		TimeoutAt: timeout,
	})
	require.NoError(t, err)

	result, err := matches.CreateProposal(ctx, CreateProposalParams{
		GameID: g.Games[0].ID,
		Mode:   "Duo",
		Region: "eu",
		Sessions: []ProposalSession{
			{SessionID: sessA.ID, ProfileID: profileA},
			{SessionID: sessB.ID, ProfileID: profileB},
		},
	})
	require.NoError(t, err)

	_, err = matches.SetProposalResponse(ctx, result.Match.ID, profileA, ProposalResponseAccepted)
	require.NoError(t, err)
	_, err = matches.SetProposalResponse(ctx, result.Match.ID, profileB, ProposalResponseAccepted)
	require.NoError(t, err)

	active, err := matches.ActivateMatch(ctx, result.Match.ID, "voice-2", "chat-2")
	require.NoError(t, err)
	require.Equal(t, MatchStatusActive, active.Status)
	return result.Match.ID
}

func seedPendingDuoMatch(t *testing.T, ctx context.Context, pool *pgxpool.Pool) (matchID, profileA, profileB uuid.UUID) {
	t.Helper()
	games := &GameStore{Pool: pool}
	g, err := games.List(ctx, ListGamesParams{PageSize: 1, Status: StatusActive})
	require.NoError(t, err)
	require.NotEmpty(t, g.Games)

	sessions := &SessionStore{Pool: pool}
	matches := &MatchStore{Pool: pool}
	timeout := time.Now().UTC().Add(30 * time.Minute)
	criteria := `{"region":"eu"}`

	profileA = uuid.New()
	profileB = uuid.New()
	sessA, err := sessions.Create(ctx, CreateSessionParams{
		ProfileID: profileA,
		GameID:    g.Games[0].ID,
		Mode:      "Duo",
		Criteria:  criteria,
		TimeoutAt: timeout,
	})
	require.NoError(t, err)
	sessB, err := sessions.Create(ctx, CreateSessionParams{
		ProfileID: profileB,
		GameID:    g.Games[0].ID,
		Mode:      "Duo",
		Criteria:  criteria,
		TimeoutAt: timeout,
	})
	require.NoError(t, err)

	result, err := matches.CreateProposal(ctx, CreateProposalParams{
		GameID: g.Games[0].ID,
		Mode:   "Duo",
		Region: "eu",
		Sessions: []ProposalSession{
			{SessionID: sessA.ID, ProfileID: profileA},
			{SessionID: sessB.ID, ProfileID: profileB},
		},
	})
	require.NoError(t, err)
	return result.Match.ID, profileA, profileB
}
