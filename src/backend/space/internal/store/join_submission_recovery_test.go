package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/space/internal/entrypolicy"
)

func TestJoinAttempt_SubmissionRecoveryPreservesHistoricalOriginal(t *testing.T) {
	for _, manual := range []bool{false, true} {
		t.Run(map[bool]string{false: "ready", true: "pending"}[manual], func(t *testing.T) {
			ctx := context.Background()
			pool := startJoinAttemptPostgres(t, ctx)
			applySpaceMigrationForStoreTest(t, ctx, pool)
			st := &SpaceStore{Pool: pool}
			owner := uuid.New()
			space, err := st.CreateSpace(ctx, owner, "Historical decision", "", "public")
			require.NoError(t, err)
			applyEntryPolicyMigration(t, st)
			applyJoinAttemptMigration(t, st)
			applyJoinOperationMigration(t, st)
			if manual {
				_, err = st.SetEntryPolicy(ctx, SetEntryPolicyInput{SpaceID: space.ID, ActorProfileID: owner, OperationID: uuid.New(), ExpectedVersion: 1, Policy: entrypolicy.Policy{SchemaVersion: 1, RequireManualApproval: true, Questions: []entrypolicy.Question{}}})
				require.NoError(t, err)
			}
			in := JoinAttemptInput{SpaceID: space.ID, AccountID: uuid.New(), ProfileID: uuid.New()}
			a, err := st.StartJoinAttempt(ctx, in)
			require.NoError(t, err)
			a, err = st.SubmitJoinAttempt(ctx, in, a.ID, nil)
			require.NoError(t, err)
			basis, sha := seedJoinDecisionHistory(t, st, in, a)
			c := basis.Context
			op := JoinOperationInput{SpaceID: c.SpaceID, ActorAccountID: c.ActorAccountID, ActorProfileID: c.ActorProfileID, OperationID: c.OperationID, Method: c.RequestMethod, RequestSHA256: c.RequestSHA256}
			// Local denial after a completed decision must not rewrite its history or
			// create a new operation/proof during recovery. Native guards still deny.
			_, err = pool.Exec(ctx, `UPDATE space_join_attempts SET state='INVALIDATED' WHERE id=$1`, a.ID)
			require.NoError(t, err)
			recovered, err := st.loadJoinSubmissionRecovery(ctx, op)
			require.NoError(t, err)
			require.Equal(t, a.ID, recovered.AttemptID)
			require.Equal(t, c.Kind, recovered.DecisionKind)
			require.Equal(t, sha, recovered.DecisionBasisSHA256)
			require.Nil(t, recovered.PublicationOperationID)
			recovered.DecisionBasisSHA256[0] ^= 1
			again, err := st.loadJoinSubmissionRecovery(ctx, op)
			require.NoError(t, err)
			require.Equal(t, sha, again.DecisionBasisSHA256)
			p := uuid.New()
			if !manual {
				// Storage-only pin fixture: an orphan36 P cannot be recovered, and
				// its historical label cannot stand in for an actual native result.
				_, err = pool.Exec(ctx, `UPDATE space_join_operations SET publication_operation_id=$3,publication_state='PREPARING' WHERE actor_profile_id=$1 AND operation_id=$2`, op.ActorProfileID, op.OperationID, p)
				require.NoError(t, err)
				_, err = st.loadJoinSubmissionRecovery(ctx, op)
				require.ErrorIs(t, err, ErrJoinAttemptUnavailable)
				_, err = pool.Exec(ctx, `UPDATE space_join_attempts SET publication_operation_id=$2 WHERE id=$1`, a.ID, p)
				require.NoError(t, err)
				pinned, err := st.loadJoinSubmissionRecovery(ctx, op)
				require.NoError(t, err)
				require.Equal(t, &p, pinned.PublicationOperationID)
				*pinned.PublicationOperationID = uuid.New()
				pinned, err = st.loadJoinSubmissionRecovery(ctx, op)
				require.NoError(t, err)
				require.Equal(t, &p, pinned.PublicationOperationID)
				_, err = pool.Exec(ctx, `UPDATE space_join_operations SET publication_state='PUBLISHED' WHERE actor_profile_id=$1 AND operation_id=$2`, op.ActorProfileID, op.OperationID)
				require.NoError(t, err)
				_, err = st.loadJoinSubmissionRecovery(ctx, op)
				require.ErrorIs(t, err, ErrJoinAttemptUnavailable)
				_, err = pool.Exec(ctx, `UPDATE space_join_operations SET publication_state='PREPARING' WHERE actor_profile_id=$1 AND operation_id=$2`, op.ActorProfileID, op.OperationID)
				require.NoError(t, err)
			} else {
				// A later approval may own P. Original SUBMIT_PENDING recovery
				// must never adopt that P or its approval evidence by inspecting A.
				_, err = pool.Exec(ctx, `UPDATE space_join_attempts SET publication_operation_id=$2 WHERE id=$1`, a.ID, p)
				require.NoError(t, err)
				pending, err := st.loadJoinSubmissionRecovery(ctx, op)
				require.NoError(t, err)
				require.Nil(t, pending.PublicationOperationID)
			}
			changed := op
			changed.RequestSHA256 = joinDigest("different body", true)
			_, err = st.loadJoinSubmissionRecovery(ctx, changed)
			require.ErrorIs(t, err, ErrJoinAttemptConflict)
			changed = op
			changed.ActorAccountID = uuid.New()
			_, err = st.loadJoinSubmissionRecovery(ctx, changed)
			require.ErrorIs(t, err, ErrJoinAttemptConflict)
			var state string
			require.NoError(t, pool.QueryRow(ctx, `SELECT state FROM space_join_attempts WHERE id=$1`, a.ID).Scan(&state))
			require.Equal(t, "INVALIDATED", state)
		})
	}
}
