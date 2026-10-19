package store

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	authv1 "voice.app/voice/auth/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/integrationtest"
)

func TestNativeLegacyV2CandidateBindsExactHistoricalDecision(t *testing.T) {
	if testing.Short() {
		t.Skip("requires real PostgreSQL native current40 decision binding")
	}
	ctx := context.Background()
	database := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"))
	database.Run(true, "goto", "34")
	st := &SpaceStore{Pool: database.Pool}
	space, err := st.CreateSpace(ctx, uuid.New(), "Native legacy v2", "", "public")
	require.NoError(t, err)
	database.Run(true, "up")
	in := JoinAttemptInput{SpaceID: space.ID, AccountID: uuid.New(), ProfileID: uuid.New()}
	a, err := st.StartJoinAttempt(ctx, in)
	require.NoError(t, err)
	a, err = st.SubmitJoinAttempt(ctx, in, a.ID, nil)
	require.NoError(t, err)
	var binding, submission []byte
	require.NoError(t, database.Pool.QueryRow(ctx, `SELECT binding_sha256,submission_sha256 FROM space_join_attempts WHERE id=$1`, a.ID).Scan(&binding, &submission))
	// Historical storage fixture only. It grants no current Auth/Role capability.
	basis := JoinLegacyDecisionBasis{Context: JoinLegacyDecisionContext{Kind: JoinLegacyReady, OperationOrigin: JoinOperationClientKey,
		SpaceID: space.ID, AttemptID: a.ID, AttemptBindingSHA256: binding, SubmissionSHA256: submission, ReadyEvidenceSHA256: a.EvidenceSHA256,
		ActorAccountID: in.AccountID, ActorProfileID: in.ProfileID, OperationID: uuid.New(), RequestMethod: spacev1.SpaceService_JoinSpace_FullMethodName,
		RequestSHA256: bytes.Repeat([]byte{1}, 32)}, ApplicantObservationSHA256: bytes.Repeat([]byte{2}, 32)}
	sha := seedLegacyJoinDecisionHistory(t, st, basis)
	input := MemberAdmissionInput{OperationID: uuid.New(), SpaceID: space.ID, AccountID: in.AccountID, ProfileID: in.ProfileID,
		JoinAttempt: &JoinAttemptReference{AttemptID: a.ID, EvidenceSHA256: a.EvidenceSHA256}, DecisionBasisSHA256: sha}
	intent, err := st.PrepareMemberAdmission(ctx, input)
	require.NoError(t, err, "current40 must consume explicit v2 rather than reinterpret it as v1")
	require.Equal(t, sha, intent.Input.DecisionBasisSHA256)
	decision, err := st.loadContinuationDecision(ctx, sha)
	require.NoError(t, err)
	require.Nil(t, decision.basis, "legacy provenance never casts into v1")
	require.NotNil(t, decision.legacyBasis)
	request, err := decision.request(intent, false)
	require.NoError(t, err)
	contextSHA, err := intent.FirstPublicationContextSHA256()
	require.NoError(t, err)
	require.Equal(t, contextSHA[:], request.AdmissionBindingSha256)
	require.Equal(t, authv1.SpaceAdmissionPhase_SPACE_ADMISSION_PHASE_FIRST_PUBLISH, request.Phase)
	require.Equal(t, in.AccountID.String(), request.AccountId)
	require.Equal(t, in.ProfileID.String(), request.ProfileId)
	_, err = decision.request(intent, true)
	require.ErrorIs(t, err, ErrJoinAttemptUnavailable, "automatic ready has no reviewer authority")
	_, err = decision.check(intent)
	require.ErrorIs(t, err, ErrJoinAttemptUnavailable, "historical tagged data cannot manufacture a private owner observation")
	changed := input
	changed.OperationID = uuid.New()
	_, err = st.PrepareMemberAdmission(ctx, changed)
	require.ErrorIs(t, err, ErrJoinAttemptConflict)
	_, err = st.ObserveMemberAdmissionContinuation(ctx, input.OperationID, nil, nil)
	require.ErrorIs(t, err, ErrJoinAttemptUnavailable, "historical provenance alone is not a fresh continuation")
	var active bool
	require.NoError(t, database.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM space_members WHERE space_id=$1 AND profile_id=$2)`, space.ID, in.ProfileID).Scan(&active))
	require.False(t, active)
}
