package partysearch

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/matchmaking/internal/partysource"
	"voice/backend/matchmaking/internal/store"
)

type proposalSnapshotFunc func(context.Context, partysource.Subject) (partysource.Snapshot, error)

func (f proposalSnapshotFunc) Snapshot(ctx context.Context, s partysource.Subject) (partysource.Snapshot, error) {
	return f(ctx, s)
}

func TestProposalValidationUnknownNeverProducesPartialChangeOrReusableProof(t *testing.T) {
	pool := partyDB(t)
	ctx := context.Background()
	params := partyParams(t, pool)
	parties := &store.PartyStore{Pool: pool}
	first, err := parties.CreateSourceParty(ctx, params)
	require.NoError(t, err)
	secondParams := partyParams(t, pool)
	secondParams.GameID, secondParams.Mode = params.GameID, params.Mode
	second, err := parties.CreateSourceParty(ctx, secondParams)
	require.NoError(t, err)
	proposal := store.CreateProposalParams{GameID: params.GameID, Mode: params.Mode, Region: "eu"}
	for _, session := range append(first.Sessions, second.Sessions...) {
		proposal.Sessions = append(proposal.Sessions, store.ProposalSession{SessionID: session.ID, ProfileID: session.ProfileID, PartyID: session.PartyID})
	}
	matches := &store.MatchStore{Pool: pool}
	candidate, err := matches.CaptureProposalCandidate(ctx, proposal)
	require.NoError(t, err)
	captured := candidate.Parties()
	var changed, unavailable uuid.UUID
	reads := 0
	validator := &partysource.Revalidator{Snapshots: proposalSnapshotFunc(func(_ context.Context, subject partysource.Subject) (partysource.Snapshot, error) {
		require.Zero(t, pool.Stat().AcquiredConns(), "protected RPC must be outside local SQL transactions")
		reads++
		for _, p := range captured {
			if p.LeaderProfileID != subject.ProfileID {
				continue
			}
			require.Equal(t, p.InitiatorAccountID, subject.AccountID)
			require.Equal(t, p.SearchOperationID, subject.SearchOperationID)
			require.Equal(t, p.InitiatorSessionEpoch, subject.SessionEpoch)
			if p.ID == unavailable {
				return partysource.Snapshot{}, partysource.ErrUnknown
			}
			snapshot := partysource.Snapshot{ProtocolVersion: p.ProtocolVersion, Kind: p.SourceKind, RoomID: p.SourceRoomID, RosterVersion: p.SourceRosterVersion, MemberProfileIDs: p.MemberProfileIDs, MemberAuthorityVersion: p.MemberAuthorityVersion, MemberAuthorities: p.MemberAuthorities, SourceAuthorityHash: p.SourceAuthorityHash}
			if p.ID == changed {
				snapshot.SourceAuthorityHash = [32]byte{9}
			}
			return snapshot, nil
		}
		return partysource.Snapshot{}, partysource.ErrUnknown
	})}
	t.Run("current is bound to one exact local cut", func(t *testing.T) {
		result, validateErr := validator.ValidateProposalCandidate(ctx, candidate)
		require.NoError(t, validateErr)
		require.NotNil(t, result.Attempt)
		require.True(t, result.Attempt.Matches(candidate))
		require.Empty(t, result.ChangedPartyIDs)
		require.Equal(t, 2, reads)
		_, updateErr := pool.Exec(ctx, `UPDATE search_sessions SET updated_at=updated_at+interval '1 millisecond' WHERE id=$1`, first.Sessions[0].ID)
		require.NoError(t, updateErr)
		fresh, captureErr := matches.CaptureProposalCandidate(ctx, proposal)
		require.NoError(t, captureErr)
		require.False(t, result.Attempt.Matches(fresh), "source proof cannot authorize another local cut")
	})
	t.Run("changed plus unknown has no partial reset set", func(t *testing.T) {
		changed, unavailable = captured[0].ID, captured[1].ID
		result, validateErr := validator.ValidateProposalCandidate(ctx, candidate)
		require.ErrorIs(t, validateErr, partysource.ErrUnknown)
		require.Nil(t, result.Attempt)
		require.Empty(t, result.ChangedPartyIDs)
	})
	t.Run("definitive mismatch identifies only affected whole party", func(t *testing.T) {
		unavailable = uuid.Nil
		result, validateErr := validator.ValidateProposalCandidate(ctx, candidate)
		require.NoError(t, validateErr)
		require.Nil(t, result.Attempt)
		require.Equal(t, []uuid.UUID{changed}, result.ChangedPartyIDs)
	})
	t.Run("missing reader cannot mint attempt", func(t *testing.T) {
		result, validateErr := (&partysource.Revalidator{}).ValidateProposalCandidate(ctx, candidate)
		require.ErrorIs(t, validateErr, partysource.ErrUnknown)
		require.Nil(t, result.Attempt)
	})
}
