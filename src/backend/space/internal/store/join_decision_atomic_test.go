package store

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/space/internal/entrypolicy"
)

func TestJoinAttempt_DecisionHookSharesSubmissionCommit(t *testing.T) {
	for _, mode := range []string{"hook-error", "expired-owner", "expired-collection", "success"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			pool := startJoinAttemptPostgres(t, ctx)
			applySpaceMigrationForStoreTest(t, ctx, pool)
			st := &SpaceStore{Pool: pool}
			space, err := st.CreateSpace(ctx, uuid.New(), "Atomic basis", "", "public")
			require.NoError(t, err)
			applyEntryPolicyMigration(t, st)
			applyJoinAttemptMigration(t, st)
			applyJoinOperationMigration(t, st)
			in := JoinAttemptInput{SpaceID: space.ID, AccountID: uuid.New(), ProfileID: uuid.New()}
			a, err := st.StartJoinAttempt(ctx, in)
			require.NoError(t, err)
			op := JoinOperationInput{SpaceID: space.ID, ActorAccountID: in.AccountID, ActorProfileID: in.ProfileID, OperationID: uuid.New(), Method: JoinOperationSubmit, RequestSHA256: joinDigest("atomic fixture", mode)}
			_, err = st.ReserveJoinOperation(ctx, op, &a.ID)
			require.NoError(t, err)
			hookRan := false
			// A local storage fixture, not a private capability or online allow.
			hook := func(ctx context.Context, scoped *SpaceStore, pending *joinAttemptPrivate) error {
				require.NotNil(t, scoped.tx)
				hookRan = true
				basis := JoinDecisionBasis{Context: JoinDecisionContext{Kind: JoinDecisionSubmitReady, SpaceID: space.ID, AttemptID: a.ID, AttemptBindingSHA256: pending.binding, SubmissionSHA256: pending.submission, ReadyEvidenceSHA256: pending.EvidenceSHA256, ActorAccountID: in.AccountID, ActorProfileID: in.ProfileID, OperationID: op.OperationID, RequestMethod: op.Method, RequestSHA256: op.RequestSHA256}, ApplicantObservationSHA256: bytes.Repeat([]byte{12}, 32)}
				sha, err := basis.SHA256()
				require.NoError(t, err)
				raw, err := json.Marshal(basis)
				require.NoError(t, err)
				_, err = scoped.db().Exec(ctx, `UPDATE space_join_operations SET state='DECIDED',decision_basis=$3,decision_basis_sha256=$4 WHERE actor_profile_id=$1 AND operation_id=$2`, op.ActorProfileID, op.OperationID, raw, sha[:])
				if err != nil {
					return err
				}
				if mode == "hook-error" {
					return ErrJoinAttemptUnavailable
				}
				if mode == "expired-collection" {
					_, err = scoped.db().Exec(ctx, `UPDATE space_join_attempts SET created_at=clock_timestamp()-interval '601 seconds',collection_deadline=clock_timestamp()-interval '1 second' WHERE id=$1`, a.ID)
				}
				return err
			}
			guard := func() error {
				if hookRan && mode == "expired-owner" {
					return ErrJoinAttemptUnavailable
				}
				return nil
			}
			result, err := st.submitJoinAttempt(ctx, in, a.ID, nil, hook, guard)
			require.True(t, hookRan)
			var attemptState, operationState string
			var evidence, basisSHA []byte
			require.NoError(t, pool.QueryRow(ctx, `SELECT state,evidence_sha256 FROM space_join_attempts WHERE id=$1`, a.ID).Scan(&attemptState, &evidence))
			require.NoError(t, pool.QueryRow(ctx, `SELECT state,decision_basis_sha256 FROM space_join_operations WHERE actor_profile_id=$1 AND operation_id=$2`, op.ActorProfileID, op.OperationID).Scan(&operationState, &basisSHA))
			if mode != "success" {
				if mode == "expired-collection" {
					require.ErrorIs(t, err, ErrJoinAttemptDenied)
				} else {
					require.ErrorIs(t, err, ErrJoinAttemptUnavailable)
				}
				require.Equal(t, "COLLECTING", attemptState)
				require.Nil(t, evidence)
				require.Equal(t, "RESERVED", operationState)
				require.Nil(t, basisSHA)
				return
			}
			require.NoError(t, err)
			require.Equal(t, "READY", attemptState)
			require.Equal(t, "DECIDED", operationState)
			require.Len(t, basisSHA, 32)
			again, err := st.submitJoinAttempt(ctx, in, a.ID, nil, func(context.Context, *SpaceStore, *joinAttemptPrivate) error {
				t.Fatal("historical replay reran hook")
				return nil
			}, func() error { t.Fatal("historical replay reran guard"); return nil })
			require.NoError(t, err)
			require.Equal(t, result, again)
		})
	}
}

func TestJoinAttempt_DecisionHookSharesApprovalCommit(t *testing.T) {
	ctx := context.Background()
	pool := startJoinAttemptPostgres(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}
	owner := uuid.New()
	space, err := st.CreateSpace(ctx, owner, "Atomic approval", "", "public")
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
	op := JoinOperationInput{SpaceID: space.ID, ActorAccountID: approval.ActorAccountID, ActorProfileID: approval.ActorProfileID, OperationID: approval.OperationID, Method: JoinOperationApprove, RequestSHA256: joinDigest("atomic approval fixture", "request")}
	_, err = st.ReserveJoinOperation(ctx, op, &a.ID)
	require.NoError(t, err)
	hookRan, expired := false, true
	hook := func(ctx context.Context, scoped *SpaceStore, pending *joinAttemptPrivate) error {
		hookRan = true
		basis := JoinDecisionBasis{Context: parent.Context, ApplicantObservationSHA256: bytes.Repeat([]byte{13}, 32), ReviewerObservationSHA256: bytes.Repeat([]byte{14}, 32)}
		basis.Context.Kind = JoinDecisionApproveReady
		basis.Context.ReadyEvidenceSHA256 = pending.EvidenceSHA256
		basis.Context.ActorAccountID, basis.Context.ActorProfileID = op.ActorAccountID, op.ActorProfileID
		basis.Context.OperationID, basis.Context.RequestMethod, basis.Context.RequestSHA256 = op.OperationID, op.Method, op.RequestSHA256
		basis.Context.SubmitBasisSHA256 = parentSHA
		sha, err := basis.SHA256()
		if err != nil {
			return err
		}
		raw, err := json.Marshal(basis)
		if err != nil {
			return err
		}
		_, err = scoped.db().Exec(ctx, `UPDATE space_join_operations SET state='DECIDED',decision_basis=$3,decision_basis_sha256=$4 WHERE actor_profile_id=$1 AND operation_id=$2`, op.ActorProfileID, op.OperationID, raw, sha[:])
		return err
	}
	guard := func(context.Context, *SpaceStore, JoinAttemptApprovalInput) error {
		if hookRan && expired {
			return ErrJoinAttemptUnavailable
		}
		return nil
	}
	_, err = st.approveJoinAttempt(ctx, approval, guard, hook)
	require.ErrorIs(t, err, ErrJoinAttemptUnavailable)
	var attemptState, operationState, approvalState string
	var basisSHA []byte
	require.NoError(t, pool.QueryRow(ctx, `SELECT state FROM space_join_attempts WHERE id=$1`, a.ID).Scan(&attemptState))
	require.Equal(t, "PENDING", attemptState)
	require.NoError(t, pool.QueryRow(ctx, `SELECT state,decision_basis_sha256 FROM space_join_operations WHERE actor_profile_id=$1 AND operation_id=$2`, op.ActorProfileID, op.OperationID).Scan(&operationState, &basisSHA))
	require.Equal(t, "RESERVED", operationState)
	require.Nil(t, basisSHA)
	require.NoError(t, pool.QueryRow(ctx, `SELECT state FROM space_join_approval_operations WHERE actor_profile_id=$1 AND operation_id=$2`, op.ActorProfileID, op.OperationID).Scan(&approvalState))
	require.Equal(t, "RESERVED", approvalState)
	hookRan, expired = false, false
	approved, err := st.approveJoinAttempt(ctx, approval, guard, hook)
	require.NoError(t, err)
	require.True(t, hookRan)
	require.Equal(t, "READY", approved.State)
	require.NoError(t, pool.QueryRow(ctx, `SELECT state,decision_basis_sha256 FROM space_join_operations WHERE actor_profile_id=$1 AND operation_id=$2`, op.ActorProfileID, op.OperationID).Scan(&operationState, &basisSHA))
	require.Equal(t, "DECIDED", operationState)
	require.Len(t, basisSHA, 32)
	again, err := st.approveJoinAttempt(ctx, approval, nil, func(context.Context, *SpaceStore, *joinAttemptPrivate) error {
		t.Fatal("historical approval reran hook")
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, approved, again)
}
