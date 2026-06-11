package store

import (
	"context"
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
