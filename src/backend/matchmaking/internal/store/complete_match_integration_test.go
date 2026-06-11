package store

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func seedActiveDuoMatch(t *testing.T, ctx context.Context, pool *pgxpool.Pool) (matchID, profileA, profileB uuid.UUID) {
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

	_, err = matches.SetProposalResponse(ctx, result.Match.ID, profileA, ProposalResponseAccepted)
	require.NoError(t, err)
	_, err = matches.SetProposalResponse(ctx, result.Match.ID, profileB, ProposalResponseAccepted)
	require.NoError(t, err)

	active, err := matches.ActivateMatch(ctx, result.Match.ID, "voice-1", "chat-1")
	require.NoError(t, err)
	require.Equal(t, MatchStatusActive, active.Status)
	return result.Match.ID, profileA, profileB
}

func TestMatchStore_CompleteMatchLeaveMarksParticipantLeft(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := StartMatchmakingDBForStoreTest(t, ctx)
	ApplyMatchmakingMigrationsForStoreTest(t, ctx, pool)

	matchID, profileA, _ := seedActiveDuoMatch(t, ctx, pool)
	matches := &MatchStore{Pool: pool}

	updated, err := matches.CompleteMatchLeave(ctx, matchID, profileA)
	require.NoError(t, err)
	require.Equal(t, MatchStatusActive, updated.Status)
	require.True(t, updated.HasLeft(profileA))
}

func TestMatchStore_CompleteMatchLeaveAllLeftSetsCompleted(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := StartMatchmakingDBForStoreTest(t, ctx)
	ApplyMatchmakingMigrationsForStoreTest(t, ctx, pool)

	matchID, profileA, profileB := seedActiveDuoMatch(t, ctx, pool)
	matches := &MatchStore{Pool: pool}

	_, err := matches.CompleteMatchLeave(ctx, matchID, profileA)
	require.NoError(t, err)

	completed, err := matches.CompleteMatchLeave(ctx, matchID, profileB)
	require.NoError(t, err)
	require.Equal(t, MatchStatusCompleted, completed.Status)
	require.NotNil(t, completed.CompletedAt)

	got, err := matches.Get(ctx, matchID)
	require.NoError(t, err)
	require.Equal(t, MatchStatusCompleted, got.Status)
}

func TestMatchStore_CompleteMatchLeaveRejectsNonParticipant(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := StartMatchmakingDBForStoreTest(t, ctx)
	ApplyMatchmakingMigrationsForStoreTest(t, ctx, pool)

	matchID, _, _ := seedActiveDuoMatch(t, ctx, pool)
	matches := &MatchStore{Pool: pool}

	_, err := matches.CompleteMatchLeave(ctx, matchID, uuid.New())
	require.ErrorIs(t, err, ErrNotMatchParticipant)
}
