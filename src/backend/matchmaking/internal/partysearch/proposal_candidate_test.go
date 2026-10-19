package partysearch

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"voice/backend/matchmaking/internal/store"
)

func TestProposalCandidateBindsPrivateWholePartyAndFreshLocalCut(t *testing.T) {
	pool := partyDB(t)
	ctx := context.Background()
	params := partyParams(t, pool)
	created, err := (&store.PartyStore{Pool: pool}).CreateSourceParty(ctx, params)
	require.NoError(t, err)
	proposal := store.CreateProposalParams{GameID: params.GameID, Mode: params.Mode, Region: "eu"}
	for _, session := range created.Sessions {
		proposal.Sessions = append(proposal.Sessions, store.ProposalSession{SessionID: session.ID, ProfileID: session.ProfileID, PartyID: session.PartyID})
	}
	matches := &store.MatchStore{Pool: pool}
	t.Run("complete private cut and changed session", func(t *testing.T) {
		candidate, captureErr := matches.CaptureProposalCandidate(ctx, proposal)
		require.NoError(t, captureErr)
		require.NotEqual(t, [32]byte{}, candidate.Hash())
		require.Len(t, candidate.Accounts(), 2)
		require.Len(t, candidate.Parties(), 1)
		require.Zero(t, pool.Stat().AcquiredConns(), "no local transaction may remain open across protected snapshot RPC")
		tx, beginErr := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
		require.NoError(t, beginErr)
		require.NoError(t, store.VerifyProposalCandidateTx(ctx, tx, candidate))
		require.NoError(t, tx.Rollback(ctx))
		_, updateErr := pool.Exec(ctx, `UPDATE search_sessions SET criteria='{"region":"us"}'::jsonb WHERE id=$1`, created.Sessions[0].ID)
		require.NoError(t, updateErr)
		tx, beginErr = pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
		require.NoError(t, beginErr)
		require.ErrorIs(t, store.VerifyProposalCandidateTx(ctx, tx, candidate), store.ErrProposalCandidateChanged)
		require.NoError(t, tx.Rollback(ctx))
		_, updateErr = pool.Exec(ctx, `UPDATE search_sessions SET criteria=$2::jsonb WHERE id=$1`, created.Sessions[0].ID, params.Criteria)
		require.NoError(t, updateErr)
	})
	t.Run("partial party is not a source cut", func(t *testing.T) {
		partial := proposal
		partial.Sessions = proposal.Sessions[:1]
		_, captureErr := matches.CaptureProposalCandidate(ctx, partial)
		require.ErrorIs(t, captureErr, store.ErrSourcePartyUnavailable)
	})
	t.Run("unqualified NULL source is never SOLO", func(t *testing.T) {
		legacy, createErr := (&store.SessionStore{Pool: pool}).Create(ctx, store.CreateSessionParams{ProfileID: uuid.New(), GameID: params.GameID, Mode: params.Mode, Criteria: params.Criteria, TimeoutAt: params.TimeoutAt})
		require.NoError(t, createErr)
		_, captureErr := matches.CaptureProposalCandidate(ctx, store.CreateProposalParams{GameID: params.GameID, Mode: params.Mode, Region: "eu", Sessions: []store.ProposalSession{{SessionID: legacy.ID, ProfileID: legacy.ProfileID}}})
		require.ErrorIs(t, captureErr, store.ErrSourcePartyUnavailable)
	})
	require.Zero(t, pool.Stat().AcquiredConns())
}
