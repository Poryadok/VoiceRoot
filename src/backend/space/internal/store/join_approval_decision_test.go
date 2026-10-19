package store

import (
	"context"
	"crypto/sha256"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/admissionauthority"
	"voice/backend/space/internal/entrypolicy"
)

func TestJoinAttempt_ApprovalProposalPreservesOriginalSubmit(t *testing.T) {
	ctx := context.Background()
	pool := startJoinAttemptPostgres(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}
	owner := uuid.New()
	space, err := st.CreateSpace(ctx, owner, "Approval proposal", "", "public")
	require.NoError(t, err)
	applyEntryPolicyMigration(t, st)
	applyJoinAttemptMigration(t, st)
	applyJoinOperationMigration(t, st)
	_, err = st.SetEntryPolicy(ctx, SetEntryPolicyInput{SpaceID: space.ID, ActorProfileID: owner, OperationID: uuid.New(), ExpectedVersion: 1, Policy: entrypolicy.Policy{SchemaVersion: 1, RequireManualApproval: true, Questions: []entrypolicy.Question{}}})
	require.NoError(t, err)
	in := JoinAttemptInput{SpaceID: space.ID, AccountID: uuid.New(), ProfileID: uuid.New()}
	a, err := st.StartJoinAttempt(ctx, in)
	require.NoError(t, err)
	a, err = st.SubmitJoinAttempt(ctx, in, a.ID, nil)
	require.NoError(t, err)
	parent, parentSHA := seedJoinDecisionHistory(t, st, in, a)
	selected := a.ID.String()
	req := &spacev1.ApproveJoinRequestRequest{SpaceId: space.ID.String(), RequestId: uuid.NewString(), OperationId: uuid.NewString(), AttemptId: &selected}
	raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(req)
	require.NoError(t, err)
	requestSHA := sha256.Sum256(raw)
	op := JoinOperationInput{SpaceID: space.ID, ActorAccountID: uuid.New(), ActorProfileID: owner, OperationID: uuid.MustParse(req.OperationId), Method: JoinOperationApprove, RequestSHA256: requestSHA[:]}
	_, err = st.ReserveJoinOperation(ctx, op, &a.ID)
	require.NoError(t, err)
	// Internal exact-A fixture: no assertion that RequestId belongs to Q, no
	// private call, and no public handler can use this constructor yet.
	proposal, err := st.prepareJoinApprovalProposal(ctx, op, req, nil)
	require.NoError(t, err)
	_, _, err = proposal.OwnerRequests()
	require.ErrorIs(t, err, ErrJoinAttemptUnavailable, "a data fixture cannot obtain owner requests")
	_, err = proposal.Seal(&admissionauthority.VerifiedSubject{}, &admissionauthority.VerifiedReviewerPermission{})
	require.ErrorIs(t, err, ErrJoinAttemptUnavailable)
	_, err = (&VerifiedJoinApprovalDecision{}).currentApplicant()
	require.ErrorIs(t, err, ErrJoinAttemptUnavailable)
	c := proposal.decisionContext
	require.Equal(t, JoinDecisionApproveReady, c.Kind)
	require.Equal(t, parentSHA, c.SubmitBasisSHA256)
	require.Equal(t, parent.Context.SubmissionSHA256, c.SubmissionSHA256)
	require.Equal(t, parent.Context.AttemptBindingSHA256, c.AttemptBindingSHA256)
	require.Equal(t, op.ActorAccountID, c.ActorAccountID)
	require.Equal(t, op.ActorProfileID, c.ActorProfileID)
	require.Equal(t, op.OperationID, c.OperationID)
	// The prospective evidence must use frozen35's exact original winner tuple.
	approval := JoinAttemptApprovalInput{SpaceID: op.SpaceID, AttemptID: a.ID, ActorAccountID: op.ActorAccountID, ActorProfileID: op.ActorProfileID, OperationID: op.OperationID}
	_, err = st.ReserveJoinAttemptApproval(ctx, approval)
	require.NoError(t, err)
	approved, err := st.ApproveJoinAttempt(ctx, approval, func(context.Context, *SpaceStore, JoinAttemptApprovalInput) error { return nil })
	require.NoError(t, err)
	require.Equal(t, approved.EvidenceSHA256, c.ReadyEvidenceSHA256)
	// A later reviewer/op cannot turn an existing READY decision into a new basis.
	_, err = st.prepareJoinApprovalProposal(ctx, op, req, nil)
	require.ErrorIs(t, err, ErrJoinAttemptUnavailable)
	var state string
	var basis []byte
	require.NoError(t, pool.QueryRow(ctx, `SELECT state,decision_basis_sha256 FROM space_join_operations WHERE actor_profile_id=$1 AND operation_id=$2`, op.ActorProfileID, op.OperationID).Scan(&state, &basis))
	require.Equal(t, "RESERVED", state)
	require.Nil(t, basis, "a data proposal never manufactures an approved basis")
	// A writer hook represents a new decision. If a competing operation has
	// already won, it must roll back instead of committing an unlinked alias.
	loser := approval
	loser.OperationID = uuid.New()
	_, err = st.ReserveJoinAttemptApproval(ctx, loser)
	require.NoError(t, err)
	hookCalls := 0
	_, err = st.approveJoinAttempt(ctx, loser, func(context.Context, *SpaceStore, JoinAttemptApprovalInput) error { return nil }, func(context.Context, *SpaceStore, *joinAttemptPrivate) error { hookCalls++; return nil })
	require.ErrorIs(t, err, ErrJoinAttemptUnavailable)
	require.Zero(t, hookCalls)
	require.NoError(t, pool.QueryRow(ctx, `SELECT state FROM space_join_approval_operations WHERE actor_profile_id=$1 AND operation_id=$2`, loser.ActorProfileID, loser.OperationID).Scan(&state))
	require.Equal(t, "RESERVED", state)
}
