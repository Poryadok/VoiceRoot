package partysearch

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"testing"
	"voice/backend/matchmaking/internal/partysource"
	"voice/backend/matchmaking/internal/store"
)

func TestResponseCandidateAcceptsPendingWithoutRelaxingProvisioning(t *testing.T) {
	ctx := context.Background()
	pool, st, p, intent := admittedProvisionFixture(t)
	_, err := pool.Exec(ctx, `UPDATE match_proposals SET response='pending' WHERE match_id=$1`, intent.MatchID)
	require.NoError(t, err)
	_, err = st.CaptureProvisionCandidate(ctx, intent.MatchID)
	require.Error(t, err, "pending proposal is never a provisioning candidate")
	params := store.ResponseParams{MatchID: intent.MatchID, AccountID: p.MemberAuthorities[0].AccountID, ProfileID: p.MemberAuthorities[0].ProfileID, OperationID: uuid.New(), SessionEpoch: 99, RequestHash: [32]byte{1}, Accept: true}
	candidate, err := st.CaptureResponseCandidate(ctx, params)
	require.NoError(t, err)
	reader := &partysource.Revalidator{Snapshots: provisionReaderFunc(func(context.Context, partysource.Subject) (partysource.Snapshot, error) {
		require.Zero(t, pool.Stat().AcquiredConns(), "response source RPC is outside SQL")
		return partysource.Snapshot{ProtocolVersion: p.ProtocolVersion, Kind: p.SourceKind, RoomID: p.SourceRoomID, RosterVersion: p.SourceRosterVersion, MemberProfileIDs: p.MemberProfileIDs, MemberAuthorityVersion: p.MemberAuthorityVersion, MemberAuthorities: p.MemberAuthorities, SourceAuthorityHash: p.SourceAuthorityHash}, nil
	})}
	validation, err := reader.ValidateResponseCandidate(ctx, candidate)
	require.NoError(t, err)
	require.NotNil(t, validation.Attempt)
	require.True(t, validation.Attempt.Matches(candidate))
	other := params
	other.OperationID = uuid.New()
	otherCandidate, err := st.CaptureResponseCandidate(ctx, other)
	require.NoError(t, err)
	require.False(t, validation.Attempt.Matches(otherCandidate), "source observation cannot authorize another decision operation")
	unknown, err := (&partysource.Revalidator{Snapshots: provisionReaderFunc(func(context.Context, partysource.Subject) (partysource.Snapshot, error) {
		return partysource.Snapshot{}, partysource.ErrUnknown
	})}).ValidateResponseCandidate(ctx, candidate)
	require.Error(t, err)
	require.Nil(t, unknown.Attempt)
	require.Empty(t, unknown.ChangedPartyIDs)
	require.NotEqual(t, [32]byte{}, candidate.Hash())
	require.Len(t, candidate.Accounts(), 2)
	parties := candidate.Parties()
	parties[0].MemberAuthorities[0].AccountID = uuid.New()
	require.NotEqual(t, parties[0].MemberAuthorities[0].AccountID, candidate.Parties()[0].MemberAuthorities[0].AccountID)
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	require.NoError(t, err)
	require.NoError(t, store.VerifyResponseCandidateTx(ctx, tx, candidate))
	require.NoError(t, tx.Rollback(ctx))
	wrong := params
	wrong.AccountID = uuid.New()
	_, err = st.CaptureResponseCandidate(ctx, wrong)
	require.Error(t, err, "current profile lookup cannot replace admitted account binding")
	_, err = pool.Exec(ctx, `UPDATE match_proposals SET response='accepted' WHERE match_id=$1 AND profile_id=$2`, intent.MatchID, params.ProfileID)
	require.NoError(t, err)
	tx, err = pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	require.NoError(t, err)
	require.Error(t, store.VerifyResponseCandidateTx(ctx, tx, candidate))
	require.NoError(t, tx.Rollback(ctx))
	_, err = pool.Exec(ctx, `UPDATE mm_native_admission_head SET state='CLOSED'`)
	require.NoError(t, err)
	_, err = st.CaptureResponseCandidate(ctx, params)
	require.Error(t, err, "positive response candidate cannot outlive current admission")
}
