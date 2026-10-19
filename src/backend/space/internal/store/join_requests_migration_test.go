package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
	commonv1 "voice.app/voice/common/v1"
	"voice/backend/space/internal/entrypolicy"
)

func TestJoinAttempt_DowngradeRefusesPrivateEvidenceAndPreservesIt(t *testing.T) {
	for _, populated := range []bool{false, true} {
		name := "empty"
		if populated {
			name = "private attempt"
		}
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			pool := startJoinAttemptPostgres(t, ctx)
			applySpaceMigrationForStoreTest(t, ctx, pool)
			st := &SpaceStore{Pool: pool}
			applyEntryPolicyMigration(t, st)
			applyJoinAttemptMigration(t, st)
			var a *JoinAttempt
			var before string
			if populated {
				space, err := st.CreateSpace(ctx, uuid.New(), "Downgrade", "", "public")
				require.NoError(t, err)
				a, err = st.StartJoinAttempt(ctx, JoinAttemptInput{SpaceID: space.ID, AccountID: uuid.New(), ProfileID: uuid.New()})
				require.NoError(t, err)
				require.NoError(t, pool.QueryRow(ctx, `SELECT md5(to_jsonb(a)::text) FROM space_join_attempts a WHERE id=$1`, a.ID).Scan(&before))
			}
			down, err := os.ReadFile(filepath.Join(repoRoot(t), "src/backend/migrations/space_db/000035_join_attempts.down.sql"))
			require.NoError(t, err)
			conn, err := pool.Acquire(ctx)
			require.NoError(t, err)
			_, err = conn.Exec(ctx, string(down))
			if populated {
				require.Error(t, err)
				var refusal *pgconn.PgError
				require.ErrorAs(t, err, &refusal)
				require.Equal(t, "P0001", refusal.Code)
				_, rollbackErr := conn.Exec(ctx, "ROLLBACK")
				require.NoError(t, rollbackErr)
			} else {
				require.NoError(t, err)
			}
			conn.Release()
			var exists bool
			require.NoError(t, pool.QueryRow(ctx, `SELECT to_regclass('space_join_attempts') IS NOT NULL`).Scan(&exists))
			require.Equal(t, populated, exists)
			if populated {
				var after string
				require.NoError(t, pool.QueryRow(ctx, `SELECT md5(to_jsonb(a)::text) FROM space_join_attempts a WHERE id=$1`, a.ID).Scan(&after))
				require.Equal(t, before, after)
			}
		})
	}
}

func TestJoinAttempt_TerminalPurgeErasesPrivateJournalAtomically(t *testing.T) {
	ctx := context.Background()
	pool := startJoinAttemptPostgres(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}
	for _, name := range []string{"000014_ownership_outbox_delivery.up.sql", "000015_audit_ledger.up.sql", "000022_lifecycle_runtime.up.sql", "000023_lifecycle_evidence_retention.up.sql", "000025_tree_audit_actions.up.sql", "000027_audit_ingestion.up.sql", "000029_audit_receipt_minimization.up.sql"} {
		raw, err := os.ReadFile(filepath.Join(repoRoot(t), "src/backend/migrations/space_db", name))
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(raw))
		require.NoError(t, err)
	}
	applyEntryPolicyMigration(t, st)
	applyJoinAttemptMigration(t, st)
	owner, account := uuid.New(), uuid.New()
	space, err := st.CreateSpace(ctx, owner, "R23 durable lifecycle", "", "public")
	require.NoError(t, err)
	question := uuid.New()
	_, err = st.SetEntryPolicy(ctx, SetEntryPolicyInput{SpaceID: space.ID, ActorProfileID: owner, OperationID: uuid.New(), ExpectedVersion: 1, Policy: entrypolicy.Policy{SchemaVersion: 1, RequireManualApproval: true, Questions: []entrypolicy.Question{{ID: question, Prompt: "2x2", Kind: "exact_text", ExpectedAnswer: "4"}}}})
	require.NoError(t, err)
	in := JoinAttemptInput{SpaceID: space.ID, AccountID: uuid.New(), ProfileID: uuid.New()}
	a, err := st.StartJoinAttempt(ctx, in)
	require.NoError(t, err)
	_, err = st.SubmitJoinAttempt(ctx, in, a.ID, []entrypolicy.Answer{{QuestionID: question, Value: "4"}})
	require.NoError(t, err)
	approval := JoinAttemptApprovalInput{SpaceID: space.ID, AttemptID: a.ID, ActorAccountID: account, ActorProfileID: owner, OperationID: uuid.New()}
	_, err = st.ReserveJoinAttemptApproval(ctx, approval)
	require.NoError(t, err)
	_, err = st.ApproveJoinAttempt(ctx, approval, func(context.Context, *SpaceStore, JoinAttemptApprovalInput) error { return nil })
	require.NoError(t, err)
	other, err := st.CreateSpace(ctx, uuid.New(), "Other", "", "public")
	require.NoError(t, err)
	otherAttempt, err := st.StartJoinAttempt(ctx, JoinAttemptInput{SpaceID: other.ID, AccountID: uuid.New(), ProfileID: uuid.New()})
	require.NoError(t, err)
	op := uuid.New()
	request := lifecycleScheduleRequest(space.ID, op)
	_, err = st.ReserveLifecycleSchedule(ctx, account, owner, 7, request)
	require.NoError(t, err)
	_, confirmation, proofDigest := lifecycleScheduleRequestEvidence(t, request)
	proof, proofHash := lifecycleAuthReceiptEvidence(t, account, owner, space.ID, op, 7, confirmation, proofDigest)
	_, err = st.RecordLifecycleDeletionProofReceipt(ctx, owner, op, proof, proofHash)
	require.NoError(t, err)
	aggregate, err := st.BeginLifecycleFreeze(ctx, space.ID, lifecycleManifestFixture())
	require.NoError(t, err)
	recordLifecycleFenceBarrier(t, aggregate, space.ID, op, 1, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN)
	require.NoError(t, st.PersistLifecycle(ctx, aggregate))
	_, _, err = st.CompleteLifecycleSchedule(ctx, space.ID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE space_lifecycle_aggregates SET scheduled_at=scheduled_at-interval '8 days',purge_after=purge_after-interval '8 days' WHERE space_id=$1`, space.ID)
	require.NoError(t, err)
	aggregate, err = st.DecideLifecycleRecovery(ctx, space.ID, 2)
	require.NoError(t, err)
	recordLifecycleFenceBarrier(t, aggregate, space.ID, op, 2, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED)
	require.NoError(t, aggregate.BeginPurging())
	require.NoError(t, st.PersistLifecycle(ctx, aggregate))
	snapshot := aggregate.Snapshot()
	require.NoError(t, aggregate.RecordRoleRetirementReceipt(lifecycleRoleRetirementReceipt(t, space.ID, op, 2, snapshot.PurgeDecidedAt)))
	for _, id := range lifecycleTestParticipants[1:] {
		require.NoError(t, aggregate.RecordPurgeReceipt(lifecyclePurgeReceipt(t, space.ID, op, id, 2, snapshot.PurgeDecidedAt)))
	}
	require.NoError(t, st.PersistLifecycle(ctx, aggregate))
	_, err = pool.Exec(ctx, `DELETE FROM spaces WHERE id=$1`, space.ID)
	require.Error(t, err, "ordinary delete cannot erase private evidence before terminal P3")
	_, err = pool.Exec(ctx, `CREATE FUNCTION fail_join_purge_test() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'fixture rollback'; END $$; CREATE TRIGGER fail_join_purge_test BEFORE DELETE ON spaces FOR EACH ROW EXECUTE FUNCTION fail_join_purge_test()`)
	require.NoError(t, err)
	_, err = st.FinalizeLifecyclePurge(ctx, space.ID, testLifecycleHasher{})
	require.Error(t, err)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM space_join_attempts WHERE id=$1`, a.ID).Scan(&count))
	require.Equal(t, 1, count)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM space_join_approval_operations WHERE attempt_id=$1`, a.ID).Scan(&count))
	require.Equal(t, 1, count)
	_, err = pool.Exec(ctx, `DROP TRIGGER fail_join_purge_test ON spaces; DROP FUNCTION fail_join_purge_test()`)
	require.NoError(t, err)
	_, err = st.FinalizeLifecyclePurge(ctx, space.ID, testLifecycleHasher{})
	require.NoError(t, err)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM space_join_attempts WHERE id=$1`, a.ID).Scan(&count))
	require.Zero(t, count)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM space_join_approval_operations WHERE attempt_id=$1`, a.ID).Scan(&count))
	require.Zero(t, count)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM space_join_attempts WHERE id=$1`, otherAttempt.ID).Scan(&count))
	require.Equal(t, 1, count)
}
