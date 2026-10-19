package partysearch

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"voice/backend/matchmaking/internal/store"
)

// Only the admitted pre-cut legacy API is tested here. These rows cannot mint
// a ProposalCandidate or qualify as Voice/LFP source authority.
func TestLegacyProposalWorksWithoutNativeSourceSchema(t *testing.T) {
	pool := partyDBThrough(t, "000018")
	ctx := context.Background()
	var nativeColumns bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_name='parties' AND column_name='voice_snapshot_protocol_version')`).Scan(&nativeColumns))
	require.False(t, nativeColumns)
	p := partyParams(t, pool)
	party, err := (&store.PartyStore{Pool: pool}).Create(ctx, store.CreatePartyParams{LeaderProfileID: p.InitiatorProfileID, MemberProfileIDs: p.MemberProfileIDs, GameID: p.GameID, Mode: p.Mode, Criteria: p.Criteria})
	require.NoError(t, err)
	var sessions []store.SearchSession
	for _, profile := range party.MemberProfileIDs {
		session, createErr := (&store.SessionStore{Pool: pool}).Create(ctx, store.CreateSessionParams{ProfileID: profile, PartyID: &party.ID, GameID: p.GameID, Mode: p.Mode, Criteria: p.Criteria, TimeoutAt: p.TimeoutAt})
		require.NoError(t, createErr)
		sessions = append(sessions, session)
	}
	result, err := (&store.MatchStore{Pool: pool}).CreateProposal(ctx, partyProposal(p, sessions))
	require.NoError(t, err)
	require.Len(t, result.Proposals, len(sessions))
}
