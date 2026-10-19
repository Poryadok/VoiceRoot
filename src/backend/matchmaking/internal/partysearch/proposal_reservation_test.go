package partysearch

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"voice/backend/matchmaking/internal/partysource"
	"voice/backend/matchmaking/internal/store"
)

type nativeProposalFixture struct {
	params    store.CreateSourcePartyParams
	search    store.SourcePartySearch
	candidate store.ProposalCandidate
	proof     *partysource.ValidatedProposalAttempt
}

func prepareNativeProposal(t *testing.T, pool *pgxpool.Pool) nativeProposalFixture {
	t.Helper()
	ctx := context.Background()
	p := partyParams(t, pool)
	created, err := (&store.PartyStore{Pool: pool}).CreateSourceParty(ctx, p)
	require.NoError(t, err)
	candidate, err := (&store.MatchStore{Pool: pool}).CaptureProposalCandidate(ctx, partyProposal(p, created.Sessions))
	require.NoError(t, err)
	return nativeProposalFixture{p, created, candidate, validateNativeProposal(t, pool, candidate)}
}

func validateNativeProposal(t *testing.T, pool *pgxpool.Pool, candidate store.ProposalCandidate) *partysource.ValidatedProposalAttempt {
	t.Helper()
	parties := candidate.Parties()
	validator := &partysource.Revalidator{Snapshots: proposalSnapshotFunc(func(_ context.Context, subject partysource.Subject) (partysource.Snapshot, error) {
		require.Zero(t, pool.Stat().AcquiredConns())
		for _, party := range parties {
			if subject.ProfileID != party.LeaderProfileID {
				continue
			}
			require.Equal(t, party.InitiatorAccountID, subject.AccountID)
			require.Equal(t, party.SearchOperationID, subject.SearchOperationID)
			require.Equal(t, party.InitiatorSessionEpoch, subject.SessionEpoch)
			return partysource.Snapshot{ProtocolVersion: party.ProtocolVersion, Kind: party.SourceKind, RoomID: party.SourceRoomID, RosterVersion: party.SourceRosterVersion, MemberProfileIDs: party.MemberProfileIDs, MemberAuthorityVersion: party.MemberAuthorityVersion, MemberAuthorities: party.MemberAuthorities, SourceAuthorityHash: party.SourceAuthorityHash}, nil
		}
		return partysource.Snapshot{}, partysource.ErrUnknown
	})}
	validation, err := validator.ValidateProposalCandidate(context.Background(), candidate)
	require.NoError(t, err)
	require.NotNil(t, validation.Attempt)
	return validation.Attempt
}

func requireProposalStillSearching(t *testing.T, pool *pgxpool.Pool, search store.SourcePartySearch) {
	t.Helper()
	for _, session := range search.Sessions {
		current, err := (&store.SessionStore{Pool: pool}).Get(context.Background(), session.ID)
		require.NoError(t, err)
		require.Equal(t, store.SessionStatusSearching, current.Status)
		require.Nil(t, current.MatchID)
		require.Equal(t, session.RecoveryGeneration, current.RecoveryGeneration)
	}
}

func TestValidatedProposalReservationRequiresAuthorityAndFreshLockedCut(t *testing.T) {
	pool := partyDB(t)
	ctx := context.Background()
	matches := &store.MatchStore{Pool: pool}
	t.Run("public native bypass refused", func(t *testing.T) {
		f := prepareNativeProposal(t, pool)
		_, err := matches.CreateProposal(ctx, partyProposal(f.params, f.search.Sessions))
		require.ErrorIs(t, err, store.ErrSourcePartyUnavailable)
		requireProposalStillSearching(t, pool, f.search)
	})
	t.Run("missing immediate proof refused", func(t *testing.T) {
		f := prepareNativeProposal(t, pool)
		_, err := matches.CreateValidatedProposal(ctx, f.candidate, nil)
		require.ErrorIs(t, err, store.ErrSourcePartyUnavailable)
		requireProposalStillSearching(t, pool, f.search)
	})
	t.Run("noninitiating member ban refused", func(t *testing.T) {
		f := prepareNativeProposal(t, pool)
		account := f.params.MemberAuthorities[1].AccountID
		require.NoError(t, (&store.BanStore{Pool: pool}).InsertPlatformMMBan(ctx, store.InsertPlatformMMBanParams{AccountID: account, BannedByProfileID: uuid.New()}))
		_, err := matches.CreateValidatedProposal(ctx, f.candidate, f.proof)
		require.ErrorIs(t, err, store.ErrPlatformBanDenied)
		require.NotContains(t, err.Error(), account.String())
		requireProposalStillSearching(t, pool, f.search)
	})
	t.Run("proof cannot be borrowed for another candidate", func(t *testing.T) {
		a, b := prepareNativeProposal(t, pool), prepareNativeProposal(t, pool)
		_, err := matches.CreateValidatedProposal(ctx, b.candidate, a.proof)
		require.ErrorIs(t, err, store.ErrSourcePartyUnavailable)
		requireProposalStillSearching(t, pool, b.search)
	})
	t.Run("metadata changed after remote validation refused", func(t *testing.T) {
		f := prepareNativeProposal(t, pool)
		_, err := pool.Exec(ctx, `UPDATE search_sessions SET updated_at=updated_at+interval '1 millisecond' WHERE id=$1`, f.search.Sessions[0].ID)
		require.NoError(t, err)
		_, err = matches.CreateValidatedProposal(ctx, f.candidate, f.proof)
		require.ErrorIs(t, err, store.ErrProposalCandidateChanged)
		requireProposalStillSearching(t, pool, f.search)
	})
	t.Run("valid reservation advances every member exactly once", func(t *testing.T) {
		f := prepareNativeProposal(t, pool)
		result, err := matches.CreateValidatedProposal(ctx, f.candidate, f.proof)
		require.NoError(t, err)
		require.Len(t, result.Proposals, len(f.search.Sessions))
		for _, session := range f.search.Sessions {
			current, getErr := (&store.SessionStore{Pool: pool}).Get(ctx, session.ID)
			require.NoError(t, getErr)
			require.Equal(t, store.SessionStatusPendingAccept, current.Status)
			require.Equal(t, &result.Match.ID, current.MatchID)
			require.Equal(t, session.RecoveryGeneration+1, current.RecoveryGeneration)
		}
	})
	t.Run("fresh verification follows an actually held session lock", func(t *testing.T) {
		f := prepareNativeProposal(t, pool)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		writer, err := pool.Begin(ctx)
		require.NoError(t, err)
		defer writer.Rollback(context.Background())
		_, err = writer.Exec(ctx, `UPDATE search_sessions SET updated_at=updated_at+interval '2 milliseconds' WHERE id=$1`, f.search.Sessions[0].ID)
		require.NoError(t, err)
		done := make(chan error, 1)
		go func() { _, err := matches.CreateValidatedProposal(ctx, f.candidate, f.proof); done <- err }()
		require.Eventually(t, func() bool {
			select {
			case err := <-done:
				t.Fatalf("reservation bypassed held session row: %v", err)
			default:
			}
			var waiting bool
			err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE '%search_sessions%' AND query LIKE '%FOR UPDATE%')`).Scan(&waiting)
			return err == nil && waiting
		}, 3*time.Second, 10*time.Millisecond)
		require.NoError(t, writer.Commit(ctx))
		require.ErrorIs(t, <-done, store.ErrProposalCandidateChanged)
		requireProposalStillSearching(t, pool, f.search)
	})
	require.Zero(t, pool.Stat().AcquiredConns())
}
