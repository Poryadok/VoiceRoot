package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/memberadmission"
)

func TestMemberAdmissionBasisCompositionPinsOnlyWithNativeIntent(t *testing.T) {
	ctx := context.Background()
	st := nativeCreatorFixture(t)
	create, _, _ := signedNativeCreator(t, st, &spacev1.CreateSpaceRequest{Name: "Native basis composition", Visibility: "public"})
	space, err := create()
	require.NoError(t, err)
	applyJoinAttemptMigration(t, st)
	applyJoinOperationMigration(t, st)
	input := JoinAttemptInput{SpaceID: space.ID, AccountID: uuid.New(), ProfileID: uuid.New()}
	a, err := st.StartJoinAttempt(ctx, input)
	require.NoError(t, err)
	a, err = st.SubmitJoinAttempt(ctx, input, a.ID, nil)
	require.NoError(t, err)
	// Historical SQL fixture only. No verified live owner decision is minted.
	_, basis := seedJoinDecisionHistory(t, st, input, a)
	in := MemberAdmissionInput{OperationID: uuid.New(), SpaceID: space.ID, AccountID: input.AccountID, ProfileID: input.ProfileID, JoinAttempt: &JoinAttemptReference{AttemptID: a.ID, EvidenceSHA256: a.EvidenceSHA256}, DecisionBasisSHA256: basis}
	_, err = st.Pool.Exec(ctx, `CREATE FUNCTION reject_basis_native_intent() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected native intent write failure'; END $$; CREATE TRIGGER reject_basis_native_intent BEFORE INSERT ON space_member_admission_intents FOR EACH ROW EXECUTE FUNCTION reject_basis_native_intent()`)
	require.NoError(t, err)
	_, err = st.PrepareMemberAdmission(ctx, in)
	require.Error(t, err)
	var state string
	var attemptPin, basisPin *uuid.UUID
	var heads, intents int
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT state,publication_operation_id FROM space_join_attempts WHERE id=$1`, a.ID).Scan(&state, &attemptPin))
	require.Equal(t, "READY", state)
	require.Nil(t, attemptPin, "failed native intent must roll back35 CLAIMED")
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT publication_operation_id FROM space_join_operations WHERE decision_basis_sha256=$1`, basis).Scan(&basisPin))
	require.Nil(t, basisPin, "failed native intent must roll back36 sole-P pin")
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM space_ordinary_member_heads WHERE space_id=$1 AND profile_id=$2),(SELECT count(*) FROM space_member_admission_intents WHERE operation_id=$3)`, space.ID, input.ProfileID, in.OperationID).Scan(&heads, &intents))
	require.Zero(t, heads)
	require.Zero(t, intents)
	_, err = st.Pool.Exec(ctx, `DROP TRIGGER reject_basis_native_intent ON space_member_admission_intents`)
	require.NoError(t, err)
	i, err := st.PrepareMemberAdmission(ctx, in)
	require.NoError(t, err)
	require.Equal(t, in, i.Input)
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT state,publication_operation_id FROM space_join_attempts WHERE id=$1`, a.ID).Scan(&state, &attemptPin))
	require.Equal(t, "CLAIMED", state)
	require.Equal(t, &in.OperationID, attemptPin)
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT publication_operation_id FROM space_join_operations WHERE decision_basis_sha256=$1`, basis).Scan(&basisPin))
	require.Equal(t, &in.OperationID, basisPin)
	again, err := st.PrepareMemberAdmission(ctx, in)
	require.NoError(t, err)
	require.Equal(t, i, again)
	changed := cloneMemberAdmissionInput(in)
	changed.OperationID = uuid.New()
	_, err = st.PrepareMemberAdmission(ctx, changed)
	require.ErrorIs(t, err, ErrJoinAttemptConflict, "one historical basis cannot select another P")
	role := uuid.New()
	receipt, err := memberadmission.EnsureReceiptFingerprint(i.EnsureBinding(), role.String())
	require.NoError(t, err)
	called := false
	_, err = st.PublishMemberAdmission(ctx, in.OperationID, role, receipt[:], func(context.Context, *SpaceStore, *MemberAdmissionIntent) error { called = true; return nil })
	require.ErrorIs(t, err, ErrJoinAttemptUnavailable, "history and generic callback cannot substitute for a sealed continuation")
	require.False(t, called)
	var members, events int
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM space_members WHERE space_id=$1 AND profile_id=$2),(SELECT count(*) FROM space_member_admission_outbox WHERE operation_id=$3)`, space.ID, input.ProfileID, in.OperationID).Scan(&members, &events))
	require.Zero(t, members)
	require.Zero(t, events)
}
