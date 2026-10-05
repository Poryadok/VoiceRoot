package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestMatchStore_CreateProposalPersistsMatchAndProposals(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := StartMatchmakingDBForStoreTest(t, ctx)
	ApplyMatchmakingMigrationsForStoreTest(t, ctx, pool)

	games := &GameStore{Pool: pool}
	g, err := games.List(ctx, ListGamesParams{PageSize: 1, Status: StatusActive})
	require.NoError(t, err)
	require.NotEmpty(t, g.Games)

	sessions := &SessionStore{Pool: pool}
	matches := &MatchStore{Pool: pool}
	timeout := time.Now().UTC().Add(30 * time.Minute)
	criteria := `{"region":"eu","self":{"role":"Carry","rank":"Herald"}}`

	profileA := uuid.New()
	profileB := uuid.New()
	sessA, err := sessions.Create(ctx, CreateSessionParams{
		ProfileID: profileA,
		GameID:    g.Games[0].ID,
		Mode:      "5v5 Ranked",
		Criteria:  criteria,
		TimeoutAt: timeout,
	})
	require.NoError(t, err)
	sessB, err := sessions.Create(ctx, CreateSessionParams{
		ProfileID: profileB,
		GameID:    g.Games[0].ID,
		Mode:      "5v5 Ranked",
		Criteria:  criteria,
		TimeoutAt: timeout,
	})
	require.NoError(t, err)

	result, err := matches.CreateProposal(ctx, CreateProposalParams{
		GameID: g.Games[0].ID,
		Mode:   "5v5 Ranked",
		Region: "eu",
		Sessions: []ProposalSession{
			{SessionID: sessA.ID, ProfileID: profileA},
			{SessionID: sessB.ID, ProfileID: profileB},
		},
	})
	require.NoError(t, err)
	require.Equal(t, MatchStatusPendingAccept, result.Match.Status)
	require.Equal(t, "eu", result.Match.Region)
	require.Len(t, result.Proposals, 2)

	for _, p := range result.Proposals {
		require.Equal(t, result.Match.ID, p.MatchID)
		require.Equal(t, ProposalResponsePending, p.Response)
	}

	loaded, err := matches.Get(ctx, result.Match.ID)
	require.NoError(t, err)
	require.Equal(t, MatchStatusPendingAccept, loaded.Status)

	proposals, err := matches.ListProposals(ctx, result.Match.ID)
	require.NoError(t, err)
	require.Len(t, proposals, 2)

	updatedA, err := sessions.Get(ctx, sessA.ID)
	require.NoError(t, err)
	require.Equal(t, SessionStatusPendingAccept, updatedA.Status)
	require.NotNil(t, updatedA.MatchID)
	require.Equal(t, result.Match.ID, *updatedA.MatchID)
}

func TestMatchStore_DeclineRecoveryEffectsRemainDurableUntilProjectionSucceeds(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := StartMatchmakingDBForStoreTest(t, ctx)
	ApplyMatchmakingMigrationsForStoreTest(t, ctx, pool)

	games := &GameStore{Pool: pool}
	g, err := games.List(ctx, ListGamesParams{PageSize: 1, Status: StatusActive})
	require.NoError(t, err)
	require.NotEmpty(t, g.Games)

	sessions := &SessionStore{Pool: pool}
	matches := &MatchStore{Pool: pool}
	profileA, profileB := uuid.New(), uuid.New()
	criteria := `{"region":"eu","self":{"role":"Carry","rank":"Herald"}}`
	var sessionA, sessionB SearchSession
	for i, profileID := range []uuid.UUID{profileA, profileB} {
		session, err := sessions.Create(ctx, CreateSessionParams{
			ProfileID: profileID,
			GameID:    g.Games[0].ID,
			Mode:      "5v5 Ranked",
			Criteria:  criteria,
			TimeoutAt: time.Now().UTC().Add(30 * time.Minute),
		})
		require.NoError(t, err)
		if i == 0 {
			sessionA = session
		} else {
			sessionB = session
		}
	}
	created, err := matches.CreateProposal(ctx, CreateProposalParams{
		GameID: g.Games[0].ID,
		Mode:   "5v5 Ranked",
		Region: "eu",
		Sessions: []ProposalSession{
			{SessionID: sessionA.ID, ProfileID: profileA},
			{SessionID: sessionB.ID, ProfileID: profileB},
		},
	})
	require.NoError(t, err)

	_, err = matches.RecordDecline(ctx, created.Match.ID, profileA)
	require.NoError(t, err)
	var pending int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM matchmaking_match_recovery_effects WHERE match_id = $1 AND applied_at IS NULL`, created.Match.ID).Scan(&pending))
	require.Equal(t, 2, pending)

	projectionFailure := errors.New("temporary queue projection failure")
	err = matches.ApplyPendingRecoveryEffects(ctx, 10, func(context.Context, SearchSession, string) error {
		return projectionFailure
	})
	require.ErrorIs(t, err, projectionFailure)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM matchmaking_match_recovery_effects WHERE match_id = $1 AND applied_at IS NULL`, created.Match.ID).Scan(&pending))
	require.Equal(t, 2, pending, "failed external projection must remain retryable")

	actions := make(map[string]int)
	err = matches.ApplyPendingRecoveryEffects(ctx, 10, func(_ context.Context, _ SearchSession, action string) error {
		actions[action]++
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, map[string]int{"enqueue": 1, "release": 1}, actions)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM matchmaking_match_recovery_effects WHERE match_id = $1 AND applied_at IS NULL`, created.Match.ID).Scan(&pending))
	require.Zero(t, pending)
}
