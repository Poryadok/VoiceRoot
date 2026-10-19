package partysearch

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"voice/backend/matchmaking/internal/partysource"
	"voice/backend/matchmaking/internal/squad"
	"voice/backend/matchmaking/internal/store"
)

func TestProvisionGateBanEvidenceResetsOnlyAffectedCapturedParty(t *testing.T) {
	ctx := context.Background()
	pool := partyDB(t)
	st := &store.MatchStore{Pool: pool}
	var sessions []store.ProposalSession
	params := map[uuid.UUID]store.CreateSourcePartyParams{}
	var game uuid.UUID
	var mode string
	for range 2 {
		p := partyParams(t, pool)
		game, mode = p.GameID, p.Mode
		created, err := (&store.PartyStore{Pool: pool}).CreateSourceParty(ctx, p)
		require.NoError(t, err)
		params[p.SearchOperationID] = p
		for _, s := range created.Sessions {
			sessions = append(sessions, store.ProposalSession{SessionID: s.ID, ProfileID: s.ProfileID, PartyID: s.PartyID})
		}
	}
	proposal, err := st.CreateProposal(ctx, store.CreateProposalParams{GameID: game, Mode: mode, Region: "eu", Sessions: sessions})
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE match_proposals SET response='accepted' WHERE match_id=$1`, proposal.Match.ID)
	require.NoError(t, err)
	gate := &squad.PartySourceGate{Store: st, Validator: &partysource.Revalidator{Snapshots: provisionReaderFunc(func(_ context.Context, subject partysource.Subject) (partysource.Snapshot, error) {
		p := params[subject.SearchOperationID]
		return partysource.Snapshot{ProtocolVersion: p.ProtocolVersion, Kind: p.SourceKind, RoomID: p.SourceRoomID, RosterVersion: p.SourceRosterVersion,
			MemberProfileIDs: p.MemberProfileIDs, MemberAuthorityVersion: p.MemberAuthorityVersion, MemberAuthorities: p.MemberAuthorities, SourceAuthorityHash: p.SourceAuthorityHash}, nil
	})}}
	candidate, proof, err := gate.ValidateAttempt(ctx, proposal.Match.ID)
	require.NoError(t, err)
	require.True(t, proof.Matches(candidate))
	require.ErrorIs(t, gate.ResetDeniedParties(ctx, candidate, store.ErrPlatformBanDenied), squad.ErrPartyProvisionUnknown)
	parties := candidate.Parties()
	deniedAccount := parties[0].MemberAuthorities[1].AccountID
	require.NoError(t, (&store.BanStore{Pool: pool}).InsertPlatformMMBan(ctx, store.InsertPlatformMMBanParams{AccountID: deniedAccount, Reason: "guard test", BannedByProfileID: uuid.New()}))
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	require.NoError(t, err)
	guard, err := store.AcquirePlatformBanGuard(ctx, tx, candidate.Accounts())
	require.NoError(t, err)
	denial := guard.Check(ctx)
	require.ErrorIs(t, denial, store.ErrPlatformBanDenied)
	require.NoError(t, tx.Rollback(ctx))
	require.ErrorIs(t, gate.ResetDeniedParties(ctx, candidate, denial), squad.ErrPartyProvisionChanged)
	for i, party := range parties {
		var closed bool
		require.NoError(t, pool.QueryRow(ctx, `SELECT disbanded_at IS NOT NULL FROM parties WHERE id=$1`, party.ID).Scan(&closed))
		require.Equal(t, i == 0, closed)
		var statuses []string
		rows, err := pool.Query(ctx, `SELECT status FROM search_sessions WHERE party_id=$1`, party.ID)
		require.NoError(t, err)
		for rows.Next() {
			var status string
			require.NoError(t, rows.Scan(&status))
			statuses = append(statuses, status)
		}
		require.NoError(t, rows.Err())
		rows.Close()
		want := store.SessionStatusSearching
		if i == 0 {
			want = store.SessionStatusCancelled
		}
		require.Equal(t, []string{want, want}, statuses)
	}
}

func TestProvisionPartyGateFreshCutUnknownAndAtomicChangedReset(t *testing.T) {
	ctx := context.Background()
	pool := partyDB(t)
	st := &store.MatchStore{Pool: pool}
	var sessions []store.ProposalSession
	params := map[uuid.UUID]store.CreateSourcePartyParams{}
	var game uuid.UUID
	var mode string
	for range 2 {
		p := partyParams(t, pool)
		game, mode = p.GameID, p.Mode
		created, err := (&store.PartyStore{Pool: pool}).CreateSourceParty(ctx, p)
		require.NoError(t, err)
		params[p.SearchOperationID] = p
		for _, s := range created.Sessions {
			sessions = append(sessions, store.ProposalSession{SessionID: s.ID, ProfileID: s.ProfileID, PartyID: s.PartyID})
		}
	}
	proposal, err := st.CreateProposal(ctx, store.CreateProposalParams{GameID: game, Mode: mode, Region: "eu", Sessions: sessions})
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE match_proposals SET response='accepted' WHERE match_id=$1`, proposal.Match.ID)
	require.NoError(t, err)
	reads := 0
	validator := &partysource.Revalidator{Snapshots: provisionReaderFunc(func(_ context.Context, subject partysource.Subject) (partysource.Snapshot, error) {
		reads++
		p := params[subject.SearchOperationID]
		return partysource.Snapshot{ProtocolVersion: p.ProtocolVersion, Kind: p.SourceKind, RoomID: p.SourceRoomID, RosterVersion: p.SourceRosterVersion,
			MemberProfileIDs: p.MemberProfileIDs, MemberAuthorityVersion: p.MemberAuthorityVersion, MemberAuthorities: p.MemberAuthorities, SourceAuthorityHash: p.SourceAuthorityHash}, nil
	})}
	gate := &squad.PartySourceGate{Store: st, Validator: validator}
	candidate, attempt, err := gate.ValidateAttempt(ctx, proposal.Match.ID)
	require.NoError(t, err)
	require.NotNil(t, attempt)
	require.True(t, attempt.Matches(candidate))
	require.Equal(t, proposal.Match.ID, candidate.MatchID)
	require.NotEqual(t, [32]byte{}, candidate.Hash())
	_, err = gate.ValidateProvision(ctx, proposal.Match.ID)
	require.NoError(t, err)
	require.Equal(t, 4, reads, "every gate obtains all fresh snapshots")
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	require.NoError(t, store.VerifyProvisionCandidateTx(ctx, tx, candidate))
	require.NoError(t, tx.Rollback(ctx))
	assertPending := func() {
		t.Helper()
		match, err := st.Get(ctx, proposal.Match.ID)
		require.NoError(t, err)
		require.Equal(t, store.MatchStatusPendingAccept, match.Status)
		for _, s := range sessions {
			got, err := (&store.SessionStore{Pool: pool}).Get(ctx, s.SessionID)
			require.NoError(t, err)
			require.Equal(t, store.SessionStatusPendingAccept, got.Status)
		}
	}
	first := candidate.Parties()[0].SearchOperationID
	validator.Snapshots = provisionReaderFunc(func(_ context.Context, subject partysource.Subject) (partysource.Snapshot, error) {
		if subject.SearchOperationID == first {
			return partysource.Snapshot{}, partysource.ErrIneligible
		}
		return partysource.Snapshot{}, partysource.ErrUnknown
	})
	unknown, err := gate.ValidateProvision(ctx, proposal.Match.ID)
	require.ErrorIs(t, err, squad.ErrPartyProvisionUnknown)
	require.Zero(t, unknown.Hash())
	assertPending()
	gate.Validator = provisionValidatorFunc(func(context.Context, store.ProvisionCandidate) (partysource.ProvisionValidation, error) {
		return partysource.ProvisionValidation{ChangedPartyIDs: []uuid.UUID{candidate.Parties()[0].ID}}, partysource.ErrUnknown
	})
	_, err = gate.ValidateProvision(ctx, proposal.Match.ID)
	require.ErrorIs(t, err, squad.ErrPartyProvisionUnknown, "an authority error overrides any partial changed IDs")
	assertPending()
	gate.Validator = validator
	missing := &squad.PartySourceGate{Store: st}
	_, err = missing.ValidateProvision(ctx, proposal.Match.ID)
	require.ErrorIs(t, err, squad.ErrPartyProvisionUnknown)
	assertPending()
	validator.Snapshots = provisionReaderFunc(func(context.Context, partysource.Subject) (partysource.Snapshot, error) {
		return partysource.Snapshot{}, partysource.ErrIneligible
	})
	_, err = pool.Exec(ctx, `ALTER TABLE matchmaking_voice_roster_cancellation_outbox ADD CONSTRAINT gate_reject_cancel CHECK(false)`)
	require.NoError(t, err)
	_, err = gate.ValidateProvision(ctx, proposal.Match.ID)
	require.ErrorIs(t, err, squad.ErrPartyProvisionUnknown, "failed reset is not completed cancellation")
	assertPending()
	_, err = pool.Exec(ctx, `ALTER TABLE matchmaking_voice_roster_cancellation_outbox DROP CONSTRAINT gate_reject_cancel`)
	require.NoError(t, err)
	changed, err := gate.ValidateProvision(ctx, proposal.Match.ID)
	require.ErrorIs(t, err, squad.ErrPartyProvisionChanged)
	require.Zero(t, changed.Hash())
	match, err := st.Get(ctx, proposal.Match.ID)
	require.NoError(t, err)
	require.Equal(t, store.MatchStatusAbandoned, match.Status)
	for _, s := range sessions {
		got, err := (&store.SessionStore{Pool: pool}).Get(ctx, s.SessionID)
		require.NoError(t, err)
		require.Equal(t, store.SessionStatusCancelled, got.Status)
	}
}

type provisionValidatorFunc func(context.Context, store.ProvisionCandidate) (partysource.ProvisionValidation, error)

func (f provisionValidatorFunc) ValidateCapturedParties(ctx context.Context, candidate store.ProvisionCandidate) (partysource.ProvisionValidation, error) {
	return f(ctx, candidate)
}
