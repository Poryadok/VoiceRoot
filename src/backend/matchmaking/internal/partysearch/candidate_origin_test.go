package partysearch

import (
	"context"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"testing"
	admission "voice/backend/matchmaking/internal/mmadmission"
	"voice/backend/matchmaking/internal/store"
)

func TestNativeCandidateV2RejectsPrecutProofAndBindsAdmissionHistory(t *testing.T) {
	pool := partyDB(t)
	ctx := context.Background()
	oldParams := partyParams(t, pool)
	oldSource, err := (&store.PartyStore{Pool: pool}).CreateSourceParty(ctx, oldParams)
	require.NoError(t, err)
	oldStore := &store.MatchStore{Pool: pool}
	oldCandidate, err := oldStore.CaptureProposalCandidate(ctx, partyProposal(oldParams, oldSource.Sessions))
	require.NoError(t, err)
	oldProof := validateNativeProposal(t, pool, oldCandidate)
	binding, receipt, root := historyReceiptFixture(t, pool)
	native := &store.MatchStore{Pool: pool, NativeAdmission: &binding}
	_, err = native.CreateValidatedProposal(ctx, oldCandidate, oldProof)
	require.Error(t, err, "pre-cut validated proof cannot cross native admission without original origin")
	requireProposalStillSearching(t, pool, oldSource)
	_, err = native.CaptureProposalCandidate(ctx, partyProposal(oldParams, oldSource.Sessions))
	require.Error(t, err, "typed historical party without origin is not enrolled")

	params := partyParams(t, pool)
	source, err := (&store.PartyStore{Pool: pool, NativeAdmission: &binding}).CreateSourceParty(ctx, params)
	require.NoError(t, err)
	proposalParams := partyProposal(params, source.Sessions)
	before, err := native.CaptureProposalCandidate(ctx, proposalParams)
	require.NoError(t, err)
	proof := validateNativeProposal(t, pool, before)
	_, err = oldStore.CaptureProposalCandidate(ctx, proposalParams)
	require.Error(t, err, "unbound reader cannot select historical v1 after native cut began")
	nextBinding := advanceOriginHistoryFixture(t, pool, binding, receipt, root)
	native.NativeAdmission = &nextBinding
	_, err = native.CreateValidatedProposal(ctx, before, proof)
	require.Error(t, err, "old current admission stamp cannot reserve after continuous rollout")
	requireProposalStillSearching(t, pool, source)
	fresh, err := native.CaptureProposalCandidate(ctx, proposalParams)
	require.NoError(t, err)
	require.NotEqual(t, before.Hash(), fresh.Hash())
	reserved, err := native.CreateValidatedProposal(ctx, fresh, validateNativeProposal(t, pool, fresh))
	require.NoError(t, err)
	// Explicit local acceptance fixture; actual RPC/dispatch is not exercised.
	_, err = pool.Exec(ctx, `UPDATE match_proposals SET response='accepted' WHERE match_id=$1`, reserved.Match.ID)
	require.NoError(t, err)
	provision, err := native.CaptureProvisionCandidate(ctx, reserved.Match.ID)
	require.NoError(t, err)
	_, err = oldStore.CaptureProvisionCandidate(ctx, reserved.Match.ID)
	require.Error(t, err)
	verify := func() error {
		tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
		require.NoError(t, err)
		defer tx.Rollback(ctx)
		require.NoError(t, admission.Acquire(ctx, tx, &nextBinding))
		return store.VerifyProvisionCandidateTx(ctx, tx, provision)
	}
	require.NoError(t, verify())
	_, err = pool.Exec(ctx, `UPDATE mm_native_admission_head SET continuity_history_root_sha256=$1`, make([]byte, 32))
	require.NoError(t, err)
	require.Error(t, verify(), "provision cut must explicitly include current validated history")
	_, err = native.CaptureProvisionCandidate(ctx, reserved.Match.ID)
	require.Error(t, err)
}
