package store

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"voice/backend/space/internal/entrypolicy"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// This fixture seeds historical rows to test integrity and atomicity. It does
// not mint a private request/owner capability or prove live RPC authorization.
func seedJoinDecisionHistory(t *testing.T, st *SpaceStore, in JoinAttemptInput, a *JoinAttempt) (JoinDecisionBasis, []byte) {
	t.Helper()
	ctx := context.Background()
	op := JoinOperationInput{SpaceID: in.SpaceID, ActorAccountID: in.AccountID, ActorProfileID: in.ProfileID, OperationID: uuid.New(), Method: JoinOperationSubmit, RequestSHA256: joinDigest("history fixture", "request")}
	_, err := st.ReserveJoinOperation(ctx, op, &a.ID)
	require.NoError(t, err)
	var binding, submission []byte
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT binding_sha256,submission_sha256 FROM space_join_attempts WHERE id=$1`, a.ID).Scan(&binding, &submission))
	kind := JoinDecisionSubmitReady
	if a.State == "PENDING" {
		kind = JoinDecisionSubmitPending
	}
	basis := JoinDecisionBasis{Context: JoinDecisionContext{Kind: kind, SpaceID: in.SpaceID, AttemptID: a.ID, AttemptBindingSHA256: binding, SubmissionSHA256: submission, ReadyEvidenceSHA256: a.EvidenceSHA256, ActorAccountID: in.AccountID, ActorProfileID: in.ProfileID, OperationID: op.OperationID, RequestMethod: op.Method, RequestSHA256: op.RequestSHA256}, ApplicantObservationSHA256: bytes.Repeat([]byte{9}, 32)}
	return basis, persistJoinDecisionHistory(t, st, op, basis)
}

func persistJoinDecisionHistory(t *testing.T, st *SpaceStore, op JoinOperationInput, basis JoinDecisionBasis) []byte {
	t.Helper()
	digest, err := basis.SHA256()
	require.NoError(t, err)
	raw, err := json.Marshal(basis)
	require.NoError(t, err)
	_, err = st.Pool.Exec(context.Background(), `UPDATE space_join_operations SET state='DECIDED',decision_basis=$3,decision_basis_sha256=$4 WHERE actor_profile_id=$1 AND operation_id=$2`, op.ActorProfileID, op.OperationID, raw, digest[:])
	require.NoError(t, err)
	return digest[:]
}

func TestJoinAttempt_DecisionBasisManualRequiresOriginalChain(t *testing.T) {
	ctx := context.Background()
	pool := startJoinAttemptPostgres(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}
	owner := uuid.New()
	space, err := st.CreateSpace(ctx, owner, "Manual basis", "", "public")
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
	approval := JoinAttemptApprovalInput{SpaceID: space.ID, AttemptID: a.ID, ActorAccountID: uuid.New(), ActorProfileID: owner, OperationID: uuid.New()}
	_, err = st.ReserveJoinAttemptApproval(ctx, approval)
	require.NoError(t, err)
	// Local storage fixture only; this is not a production owner capability.
	a, err = st.ApproveJoinAttempt(ctx, approval, func(context.Context, *SpaceStore, JoinAttemptApprovalInput) error { return nil })
	require.NoError(t, err)
	op := JoinOperationInput{SpaceID: space.ID, ActorAccountID: approval.ActorAccountID, ActorProfileID: approval.ActorProfileID, OperationID: approval.OperationID, Method: JoinOperationApprove, RequestSHA256: joinDigest("manual fixture", "approval")}
	_, err = st.ReserveJoinOperation(ctx, op, &a.ID)
	require.NoError(t, err)
	basis := JoinDecisionBasis{Context: parent.Context, ApplicantObservationSHA256: bytes.Repeat([]byte{10}, 32), ReviewerObservationSHA256: bytes.Repeat([]byte{11}, 32)}
	basis.Context.Kind = JoinDecisionApproveReady
	basis.Context.ReadyEvidenceSHA256 = a.EvidenceSHA256
	basis.Context.ActorAccountID, basis.Context.ActorProfileID = op.ActorAccountID, op.ActorProfileID
	basis.Context.OperationID, basis.Context.RequestMethod, basis.Context.RequestSHA256 = op.OperationID, op.Method, op.RequestSHA256
	basis.Context.SubmitBasisSHA256 = parentSHA
	sha := persistJoinDecisionHistory(t, st, op, basis)
	ref := JoinAttemptReference{AttemptID: a.ID, EvidenceSHA256: a.EvidenceSHA256}
	p := uuid.New()
	verify := func() error {
		return st.withOwnershipScope(ctx, []uuid.UUID{space.ID}, func(scoped *SpaceStore) error {
			return scoped.VerifyJoinDecisionBasis(ctx, in, ref, sha, a.PolicyVersion, p)
		})
	}
	// A complete approval record alone cannot replace its original submission.
	_, err = pool.Exec(ctx, `UPDATE space_join_operations SET state='RESERVED',decision_basis=NULL,decision_basis_sha256=NULL WHERE decision_basis_sha256=$1`, parentSHA)
	require.NoError(t, err)
	require.ErrorIs(t, verify(), ErrJoinAttemptUnavailable)
	var state string
	var publication *uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT state,publication_operation_id FROM space_join_attempts WHERE id=$1`, a.ID).Scan(&state, &publication))
	require.Equal(t, "READY", state)
	require.Nil(t, publication)
	parentOp := JoinOperationInput{SpaceID: parent.Context.SpaceID, ActorAccountID: parent.Context.ActorAccountID, ActorProfileID: parent.Context.ActorProfileID, OperationID: parent.Context.OperationID, Method: parent.Context.RequestMethod, RequestSHA256: parent.Context.RequestSHA256}
	require.Equal(t, parentSHA, persistJoinDecisionHistory(t, st, parentOp, parent))
	// A different current reviewer cannot substitute for the saved winner.
	_, err = pool.Exec(ctx, `UPDATE space_join_attempts SET approved_account_id=$2 WHERE id=$1`, a.ID, uuid.New())
	require.NoError(t, err)
	require.ErrorIs(t, verify(), ErrJoinAttemptUnavailable)
	_, err = pool.Exec(ctx, `UPDATE space_join_attempts SET approved_account_id=$2 WHERE id=$1`, a.ID, approval.ActorAccountID)
	require.NoError(t, err)
	require.NoError(t, verify())
}

func TestJoinAttempt_DecisionBasisExactHistoricalPin(t *testing.T) {
	ctx := context.Background()
	pool := startJoinAttemptPostgres(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}
	space, err := st.CreateSpace(ctx, uuid.New(), "Basis pin", "", "public")
	require.NoError(t, err)
	applyEntryPolicyMigration(t, st)
	applyJoinAttemptMigration(t, st)
	applyJoinOperationMigration(t, st)
	in := JoinAttemptInput{SpaceID: space.ID, AccountID: uuid.New(), ProfileID: uuid.New()}
	a, err := st.StartJoinAttempt(ctx, in)
	require.NoError(t, err)
	a, err = st.SubmitJoinAttempt(ctx, in, a.ID, nil)
	require.NoError(t, err)
	basis, sha := seedJoinDecisionHistory(t, st, in, a)
	loaded, err := st.LoadJoinDecisionBasis(ctx, sha)
	require.NoError(t, err)
	require.Equal(t, basis, *loaded)
	ref := JoinAttemptReference{AttemptID: a.ID, EvidenceSHA256: a.EvidenceSHA256}
	p := uuid.New()
	require.ErrorIs(t, st.VerifyJoinDecisionBasis(ctx, in, ref, sha, a.PolicyVersion, p), ErrJoinAttemptUnavailable)
	require.NoError(t, st.withOwnershipScope(ctx, []uuid.UUID{space.ID}, func(scoped *SpaceStore) error {
		return scoped.VerifyJoinDecisionBasis(ctx, in, ref, sha, a.PolicyVersion, p)
	}))
	require.NoError(t, st.withOwnershipScope(ctx, []uuid.UUID{space.ID}, func(scoped *SpaceStore) error {
		return scoped.VerifyJoinDecisionBasis(ctx, in, ref, sha, a.PolicyVersion, p)
	}))
	err = st.withOwnershipScope(ctx, []uuid.UUID{space.ID}, func(scoped *SpaceStore) error {
		return scoped.VerifyJoinDecisionBasis(ctx, in, ref, sha, a.PolicyVersion, uuid.New())
	})
	require.ErrorIs(t, err, ErrJoinAttemptConflict)
	var actual uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT publication_operation_id FROM space_join_operations WHERE decision_basis_sha256=$1`, sha).Scan(&actual))
	require.Equal(t, p, actual)
	require.ErrorIs(t, st.MarkJoinDecisionBasisPublished(ctx, ref, sha, p), ErrJoinAttemptUnavailable)
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()
	require.NoError(t, lockAndCheckOwnershipScope(ctx, tx, []uuid.UUID{space.ID}))
	scoped := *st
	scoped.tx = tx
	require.ErrorIs(t, scoped.MarkJoinDecisionBasisPublished(ctx, ref, sha, p), ErrJoinAttemptUnavailable, "basis cannot publish before the exact35 marker")
	require.NoError(t, scoped.MarkJoinAttemptPublished(ctx, ref, p))
	require.ErrorIs(t, scoped.MarkJoinDecisionBasisPublished(ctx, ref, sha, uuid.New()), ErrJoinAttemptUnavailable)
	require.NoError(t, scoped.MarkJoinDecisionBasisPublished(ctx, ref, sha, p))
	var publicationState string
	require.NoError(t, tx.QueryRow(ctx, `SELECT publication_state FROM space_join_operations WHERE decision_basis_sha256=$1`, sha).Scan(&publicationState))
	require.Equal(t, "PUBLISHED", publicationState)
	// Storage-only fixture: never commit a publication marker without the
	// actual native member/head/count/use/outcome transaction supplied by186.
	require.NoError(t, tx.Rollback(ctx))
	require.NoError(t, pool.QueryRow(ctx, `SELECT publication_state FROM space_join_operations WHERE decision_basis_sha256=$1`, sha).Scan(&publicationState))
	require.Equal(t, "PREPARING", publicationState)
	// A malicious/corrupt private row is not repaired from its requested digest.
	_, err = pool.Exec(ctx, `UPDATE space_join_operations SET decision_basis=jsonb_set(decision_basis,'{context,request_method}','"/changed"') WHERE decision_basis_sha256=$1`, sha)
	require.NoError(t, err)
	_, err = st.LoadJoinDecisionBasis(ctx, sha)
	require.ErrorIs(t, err, ErrJoinAttemptUnavailable)
}
