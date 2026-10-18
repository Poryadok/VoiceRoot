package partysearch

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"sort"
	"testing"
	"voice/backend/matchmaking/internal/partysource"
	"voice/backend/matchmaking/internal/store"
)

func TestProvisionCutBindsWholePersistedPartyAndLocalChanges(t *testing.T) {
	ctx := context.Background()
	pool := partyDB(t)
	p := partyParams(t, pool)
	party, err := (&store.PartyStore{Pool: pool}).CreateSourceParty(ctx, p)
	require.NoError(t, err)
	sessions := []store.ProposalSession{}
	for _, s := range party.Sessions {
		sessions = append(sessions, store.ProposalSession{SessionID: s.ID, ProfileID: s.ProfileID, PartyID: s.PartyID})
	}
	matches := &store.MatchStore{Pool: pool}
	proposal, err := matches.CreateProposal(ctx, store.CreateProposalParams{GameID: p.GameID, Mode: p.Mode, Region: "eu", Sessions: sessions})
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE match_proposals SET response='accepted' WHERE match_id=$1`, proposal.Match.ID)
	require.NoError(t, err)
	cut, err := matches.CaptureProvisionCandidate(ctx, proposal.Match.ID)
	require.NoError(t, err)
	require.Equal(t, proposal.Match.ID, cut.MatchID)
	require.NotEqual(t, [32]byte{}, cut.Hash())
	require.Len(t, cut.Parties(), 1)
	require.Equal(t, party.Party.SearchOperationID, cut.Parties()[0].SearchOperationID)
	accounts := []uuid.UUID{p.MemberAuthorities[0].AccountID, p.MemberAuthorities[1].AccountID}
	sort.Slice(accounts, func(i, j int) bool { return accounts[i].String() < accounts[j].String() })
	require.Equal(t, accounts, cut.Accounts())
	copyAccounts := cut.Accounts()
	copyAccounts[0] = uuid.Nil
	require.Equal(t, accounts, cut.Accounts())
	copyParties := cut.Parties()
	copyParties[0].MemberAuthorities[0].AccountID = uuid.Nil
	require.Equal(t, p.MemberAuthorities, cut.Parties()[0].MemberAuthorities)
	validator := &partysource.Revalidator{Snapshots: provisionReaderFunc(func(_ context.Context, s partysource.Subject) (partysource.Snapshot, error) {
		require.Equal(t, p.InitiatorAccountID, s.AccountID)
		return partysource.Snapshot{ProtocolVersion: 1, Kind: p.SourceKind, RoomID: p.SourceRoomID, RosterVersion: p.SourceRosterVersion, MemberProfileIDs: p.MemberProfileIDs, MemberAuthorityVersion: p.MemberAuthorityVersion, MemberAuthorities: p.MemberAuthorities, SourceAuthorityHash: p.SourceAuthorityHash}, nil
	})}
	validated, err := validator.ValidateCapturedParties(ctx, cut)
	require.NoError(t, err)
	require.NotNil(t, validated.Attempt)
	require.True(t, validated.Attempt.Matches(cut))
	require.Empty(t, validated.ChangedPartyIDs)
	validator.Snapshots = provisionReaderFunc(func(context.Context, partysource.Subject) (partysource.Snapshot, error) {
		return partysource.Snapshot{}, partysource.ErrIneligible
	})
	changed, err := validator.ValidateCapturedParties(ctx, cut)
	require.NoError(t, err)
	require.Nil(t, changed.Attempt)
	require.Equal(t, []uuid.UUID{party.Party.ID}, changed.ChangedPartyIDs)
	validator.Snapshots = provisionReaderFunc(func(context.Context, partysource.Subject) (partysource.Snapshot, error) {
		return partysource.Snapshot{}, partysource.ErrUnknown
	})
	unknown, err := validator.ValidateCapturedParties(ctx, cut)
	require.Error(t, err)
	require.Nil(t, unknown.Attempt)
	require.Empty(t, unknown.ChangedPartyIDs)
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	require.NoError(t, store.VerifyProvisionCandidateTx(ctx, tx, cut))
	require.NoError(t, tx.Rollback(ctx))
	_, err = pool.Exec(ctx, `UPDATE search_sessions SET recovery_generation=recovery_generation+1 WHERE id=$1`, party.Sessions[0].ID)
	require.NoError(t, err)
	tx, err = pool.Begin(ctx)
	require.NoError(t, err)
	require.ErrorIs(t, store.VerifyProvisionCandidateTx(ctx, tx, cut), store.ErrProvisionCandidateChanged)
	require.NoError(t, tx.Rollback(ctx))
	fresh, err := matches.CaptureProvisionCandidate(ctx, proposal.Match.ID)
	require.NoError(t, err)
	require.NotEqual(t, cut.Hash(), fresh.Hash())
	require.False(t, validated.Attempt.Matches(fresh))
	_, err = pool.Exec(ctx, `UPDATE search_sessions SET mode='mismatched-source-mode' WHERE id=$1`, party.Sessions[0].ID)
	require.NoError(t, err)
	_, err = matches.CaptureProvisionCandidate(ctx, proposal.Match.ID)
	require.Error(t, err, "captured party must not authorize a mismatched session/match mode")
	_, err = pool.Exec(ctx, `UPDATE search_sessions SET mode=$2 WHERE id=$1`, party.Sessions[0].ID, p.Mode)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE parties SET disbanded_at=clock_timestamp() WHERE id=$1`, party.Party.ID)
	require.NoError(t, err)
	_, err = matches.CaptureProvisionCandidate(ctx, proposal.Match.ID)
	require.Error(t, err)
}

type provisionReaderFunc func(context.Context, partysource.Subject) (partysource.Snapshot, error)

func (f provisionReaderFunc) Snapshot(ctx context.Context, s partysource.Subject) (partysource.Snapshot, error) {
	return f(ctx, s)
}

func TestProvisionValidationUnknownDoesNotReturnPartialCancellation(t *testing.T) {
	ctx := context.Background()
	pool := partyDB(t)
	parties := &store.PartyStore{Pool: pool}
	var sessions []store.ProposalSession
	var game uuid.UUID
	var mode string
	for i := 0; i < 2; i++ {
		p := partyParams(t, pool)
		game = p.GameID
		mode = p.Mode
		created, err := parties.CreateSourceParty(ctx, p)
		require.NoError(t, err)
		for _, s := range created.Sessions {
			sessions = append(sessions, store.ProposalSession{SessionID: s.ID, ProfileID: s.ProfileID, PartyID: s.PartyID})
		}
	}
	matches := &store.MatchStore{Pool: pool}
	match, err := matches.CreateProposal(ctx, store.CreateProposalParams{GameID: game, Mode: mode, Sessions: sessions})
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE match_proposals SET response='accepted' WHERE match_id=$1`, match.Match.ID)
	require.NoError(t, err)
	cut, err := matches.CaptureProvisionCandidate(ctx, match.Match.ID)
	require.NoError(t, err)
	require.Len(t, cut.Parties(), 2)
	calls := 0
	first := cut.Parties()[0].SearchOperationID
	validator := &partysource.Revalidator{Snapshots: provisionReaderFunc(func(_ context.Context, s partysource.Subject) (partysource.Snapshot, error) {
		calls++
		if s.SearchOperationID == first {
			return partysource.Snapshot{}, partysource.ErrIneligible
		}
		return partysource.Snapshot{}, partysource.ErrUnknown
	})}
	result, err := validator.ValidateCapturedParties(ctx, cut)
	require.Error(t, err)
	require.Nil(t, result.Attempt)
	require.Empty(t, result.ChangedPartyIDs)
	require.Equal(t, 2, calls)
	validator.Snapshots = provisionReaderFunc(func(context.Context, partysource.Subject) (partysource.Snapshot, error) {
		return partysource.Snapshot{}, partysource.ErrIneligible
	})
	result, err = validator.ValidateCapturedParties(ctx, cut)
	require.NoError(t, err)
	require.Nil(t, result.Attempt)
	require.Equal(t, []uuid.UUID{cut.Parties()[0].ID, cut.Parties()[1].ID}, result.ChangedPartyIDs)
}
func TestProvisionCutNeverInventsCaptureForLegacySession(t *testing.T) {
	ctx := context.Background()
	pool := partyDB(t)
	p := partyParams(t, pool)
	session, err := (&store.SessionStore{Pool: pool}).Create(ctx, store.CreateSessionParams{ProfileID: p.InitiatorProfileID, GameID: p.GameID, Mode: p.Mode, Criteria: p.Criteria, TimeoutAt: p.TimeoutAt})
	require.NoError(t, err)
	matches := &store.MatchStore{Pool: pool}
	proposal, err := matches.CreateProposal(ctx, store.CreateProposalParams{GameID: p.GameID, Mode: p.Mode, Sessions: []store.ProposalSession{{SessionID: session.ID, ProfileID: session.ProfileID}}})
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE match_proposals SET response='accepted' WHERE match_id=$1`, proposal.Match.ID)
	require.NoError(t, err)
	_, err = matches.CaptureProvisionCandidate(ctx, proposal.Match.ID)
	require.Error(t, err)
	_, err = matches.CaptureProvisionCandidate(ctx, uuid.Nil)
	require.Error(t, err)
}
