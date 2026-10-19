package store

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"voice/backend/space/internal/entrypolicy"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
	spacev1 "voice.app/voice/space/v1"
)

func TestJoinAttempt_LegacyBasisDowngradePreservesConcurrentProvenance(t *testing.T) {
	for _, mode := range []string{"empty_rc", "v2_rc", "old_rr"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			pool := startJoinAttemptPostgres(t, ctx)
			applySpaceMigrationForStoreTest(t, ctx, pool)
			st := &SpaceStore{Pool: pool}
			space, err := st.CreateSpace(ctx, uuid.New(), "Downgrade provenance", "", "public")
			require.NoError(t, err)
			applyEntryPolicyMigration(t, st)
			applyJoinAttemptMigration(t, st)
			applyJoinOperationMigration(t, st)
			applyJoinV2Migration(t, st)
			conn, err := pool.Acquire(ctx)
			require.NoError(t, err)
			defer conn.Release()
			if mode == "old_rr" {
				_, err = conn.Exec(ctx, "BEGIN ISOLATION LEVEL REPEATABLE READ")
				require.NoError(t, err)
				var count int
				require.NoError(t, conn.QueryRow(ctx, `SELECT count(*) FROM space_join_operations`).Scan(&count))
				require.Zero(t, count, "establish empty snapshot before another connection commits its first v2 reservation")
			}
			op := uuid.New()
			if mode != "empty_rc" {
				_, err = pool.Exec(ctx, `INSERT INTO space_join_operations(actor_profile_id,operation_id,actor_account_id,space_id,request_method,request_sha256,basis_version,operation_origin) VALUES($1,$2,$3,$4,$5,$6,2,2)`, uuid.New(), op, uuid.New(), space.ID, spacev1.SpaceService_JoinSpace_FullMethodName, bytes.Repeat([]byte{1}, 32))
				require.NoError(t, err)
			}
			raw, err := os.ReadFile(filepath.Join(repoRoot(t), "src/backend/migrations/space_db/000040_join_decision_basis_v2.down.sql"))
			require.NoError(t, err)
			_, err = conn.Exec(ctx, string(raw))
			if mode == "empty_rc" {
				require.NoError(t, err)
			} else {
				var refusal *pgconn.PgError
				require.ErrorAs(t, err, &refusal)
				require.Equal(t, "55000", refusal.Code)
				_, err = conn.Exec(ctx, "ROLLBACK")
				require.NoError(t, err)
				var version int16
				require.NoError(t, pool.QueryRow(ctx, `SELECT basis_version FROM space_join_operations WHERE operation_id=$1`, op).Scan(&version))
				require.EqualValues(t, 2, version)
			}
			var exists bool
			require.NoError(t, pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_name='space_join_operations' AND column_name='basis_version')`).Scan(&exists))
			require.Equal(t, mode != "empty_rc", exists)
			// The migration's transaction must not remain open on the pooled connection.
			_, err = conn.Exec(ctx, "ROLLBACK")
			require.NoError(t, err)
		})
	}
}

func seedLegacyJoinDecisionHistory(t *testing.T, st *SpaceStore, basis JoinLegacyDecisionBasis) []byte {
	t.Helper()
	sha, err := basis.SHA256()
	require.NoError(t, err)
	raw, err := json.Marshal(basis)
	require.NoError(t, err)
	c := basis.Context
	_, err = st.Pool.Exec(context.Background(), `INSERT INTO space_join_operations(actor_profile_id,operation_id,actor_account_id,space_id,request_method,request_sha256,selected_attempt_id,state,decision_basis,decision_basis_sha256,basis_version,operation_origin) VALUES($1,$2,$3,$4,$5,$6,$7,'DECIDED',$8,$9,2,$10)`, c.ActorProfileID, c.OperationID, c.ActorAccountID, c.SpaceID, c.RequestMethod, c.RequestSHA256, c.AttemptID, raw, sha[:], c.OperationOrigin)
	require.NoError(t, err)
	return sha[:]
}

func TestJoinAttempt_LegacyApprovalRequiresOriginalLegacyParent(t *testing.T) {
	ctx := context.Background()
	pool := startJoinAttemptPostgres(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}
	owner := uuid.New()
	space, err := st.CreateSpace(ctx, owner, "Legacy manual basis", "", "public")
	require.NoError(t, err)
	applyEntryPolicyMigration(t, st)
	applyJoinAttemptMigration(t, st)
	applyJoinOperationMigration(t, st)
	applyJoinV2Migration(t, st)
	_, err = st.SetEntryPolicy(ctx, SetEntryPolicyInput{SpaceID: space.ID, ActorProfileID: owner, OperationID: uuid.New(), ExpectedVersion: 1, Policy: entrypolicy.Policy{SchemaVersion: 1, RequireManualApproval: true, Questions: []entrypolicy.Question{}}})
	require.NoError(t, err)
	in := JoinAttemptInput{SpaceID: space.ID, AccountID: uuid.New(), ProfileID: uuid.New()}
	a, err := st.StartJoinAttempt(ctx, in)
	require.NoError(t, err)
	a, err = st.SubmitJoinAttempt(ctx, in, a.ID, nil)
	require.NoError(t, err)
	var binding, submission []byte
	require.NoError(t, pool.QueryRow(ctx, `SELECT binding_sha256,submission_sha256 FROM space_join_attempts WHERE id=$1`, a.ID).Scan(&binding, &submission))
	parent := JoinLegacyDecisionBasis{Context: JoinLegacyDecisionContext{Kind: JoinLegacyPending, OperationOrigin: JoinOperationServerNoKey, SpaceID: space.ID, AttemptID: a.ID, AttemptBindingSHA256: binding, SubmissionSHA256: submission, ActorAccountID: in.AccountID, ActorProfileID: in.ProfileID, OperationID: uuid.New(), RequestMethod: spacev1.SpaceService_JoinByInvite_FullMethodName, RequestSHA256: bytes.Repeat([]byte{1}, 32)}, ApplicantObservationSHA256: bytes.Repeat([]byte{2}, 32)}
	parentSHA := seedLegacyJoinDecisionHistory(t, st, parent)
	approval := JoinAttemptApprovalInput{SpaceID: space.ID, AttemptID: a.ID, ActorAccountID: uuid.New(), ActorProfileID: owner, OperationID: uuid.New()}
	_, err = st.ReserveJoinAttemptApproval(ctx, approval)
	require.NoError(t, err)
	// Historical storage fixture, not a production authorization capability.
	a, err = st.ApproveJoinAttempt(ctx, approval, func(context.Context, *SpaceStore, JoinAttemptApprovalInput) error { return nil })
	require.NoError(t, err)
	basis := JoinLegacyDecisionBasis{Context: parent.Context, ApplicantObservationSHA256: bytes.Repeat([]byte{3}, 32), ReviewerObservationSHA256: bytes.Repeat([]byte{4}, 32)}
	basis.Context.Kind, basis.Context.OperationOrigin = JoinApproveLegacyReady, JoinOperationClientKey
	basis.Context.ActorAccountID, basis.Context.ActorProfileID, basis.Context.OperationID = approval.ActorAccountID, approval.ActorProfileID, approval.OperationID
	basis.Context.RequestMethod, basis.Context.RequestSHA256 = JoinOperationApprove, bytes.Repeat([]byte{5}, 32)
	basis.Context.ReadyEvidenceSHA256, basis.Context.PendingBasisSHA256 = a.EvidenceSHA256, parentSHA
	sha := seedLegacyJoinDecisionHistory(t, st, basis)
	loaded, err := st.LoadVersionedJoinDecisionBasis(ctx, sha)
	require.NoError(t, err)
	actual, err := loaded.LegacyV2()
	require.NoError(t, err)
	require.Equal(t, basis, actual)
	ref := JoinAttemptReference{AttemptID: a.ID, EvidenceSHA256: a.EvidenceSHA256}
	p := uuid.New()
	verify := func() error {
		return st.withOwnershipScope(ctx, []uuid.UUID{space.ID}, func(local *SpaceStore) error {
			return local.VerifyVersionedJoinDecisionBasis(ctx, in, ref, sha, a.PolicyVersion, p)
		})
	}
	_, err = pool.Exec(ctx, `UPDATE space_join_operations SET state='RESERVED',decision_basis=NULL,decision_basis_sha256=NULL WHERE decision_basis_sha256=$1`, parentSHA)
	require.NoError(t, err)
	_, err = st.LoadVersionedJoinDecisionBasis(ctx, sha)
	require.ErrorIs(t, err, ErrJoinAttemptUnavailable)
	require.ErrorIs(t, verify(), ErrJoinAttemptUnavailable)
	_, err = pool.Exec(ctx, `DELETE FROM space_join_operations WHERE actor_profile_id=$1 AND operation_id=$2`, parent.Context.ActorProfileID, parent.Context.OperationID)
	require.NoError(t, err)
	require.Equal(t, parentSHA, seedLegacyJoinDecisionHistory(t, st, parent))
	_, err = pool.Exec(ctx, `UPDATE space_join_attempts SET approved_account_id=$2 WHERE id=$1`, a.ID, uuid.New())
	require.NoError(t, err)
	require.ErrorIs(t, verify(), ErrJoinAttemptUnavailable)
	_, err = pool.Exec(ctx, `UPDATE space_join_attempts SET approved_account_id=$2 WHERE id=$1`, a.ID, approval.ActorAccountID)
	require.NoError(t, err)
	require.NoError(t, verify())
	// A well-formed v1 pending row cannot be silently reinterpreted as v2.
	foreignInput := JoinAttemptInput{SpaceID: space.ID, AccountID: uuid.New(), ProfileID: uuid.New()}
	foreignA, err := st.StartJoinAttempt(ctx, foreignInput)
	require.NoError(t, err)
	foreignA, err = st.SubmitJoinAttempt(ctx, foreignInput, foreignA.ID, nil)
	require.NoError(t, err)
	_, foreignSHA := seedJoinDecisionHistory(t, st, foreignInput, foreignA)
	basis.Context.PendingBasisSHA256 = foreignSHA
	raw, err := json.Marshal(basis)
	require.NoError(t, err)
	wrongSHA, err := basis.SHA256()
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE space_join_operations SET decision_basis=$2,decision_basis_sha256=$3 WHERE decision_basis_sha256=$1`, sha, raw, wrongSHA[:])
	require.NoError(t, err)
	_, err = st.LoadVersionedJoinDecisionBasis(ctx, wrongSHA[:])
	require.ErrorIs(t, err, ErrJoinAttemptUnavailable)
}

func applyJoinV2Migration(t *testing.T, st *SpaceStore) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), "src/backend/migrations/space_db/000040_join_decision_basis_v2.up.sql"))
	require.NoError(t, err)
	_, err = st.Pool.Exec(context.Background(), string(raw))
	require.NoError(t, err)
}

// Historical rows explicitly seed storage integrity; they never mint an ingress
// or owner capability. Public decision writers must supply those independently.
func TestJoinAttempt_VersionedHistoricalLoaderPreservesV1AndSeparatesLegacy(t *testing.T) {
	ctx := context.Background()
	pool := startJoinAttemptPostgres(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}
	space, err := st.CreateSpace(ctx, uuid.New(), "Versioned basis", "", "public")
	require.NoError(t, err)
	applyEntryPolicyMigration(t, st)
	applyJoinAttemptMigration(t, st)
	applyJoinOperationMigration(t, st)
	in := JoinAttemptInput{SpaceID: space.ID, AccountID: uuid.New(), ProfileID: uuid.New()}
	a, err := st.StartJoinAttempt(ctx, in)
	require.NoError(t, err)
	a, err = st.SubmitJoinAttempt(ctx, in, a.ID, nil)
	require.NoError(t, err)
	v1, v1SHA := seedJoinDecisionHistory(t, st, in, a)
	var before []byte
	require.NoError(t, pool.QueryRow(ctx, `SELECT decision_basis::text::bytea FROM space_join_operations WHERE decision_basis_sha256=$1`, v1SHA).Scan(&before))
	applyJoinV2Migration(t, st)
	var after []byte
	require.NoError(t, pool.QueryRow(ctx, `SELECT decision_basis::text::bytea FROM space_join_operations WHERE decision_basis_sha256=$1`, v1SHA).Scan(&after))
	require.Equal(t, before, after)
	legacyInput := JoinAttemptInput{SpaceID: space.ID, AccountID: uuid.New(), ProfileID: uuid.New()}
	legacyA, err := st.StartJoinAttempt(ctx, legacyInput)
	require.NoError(t, err)
	legacyA, err = st.SubmitJoinAttempt(ctx, legacyInput, legacyA.ID, nil)
	require.NoError(t, err)
	var binding, submission []byte
	require.NoError(t, pool.QueryRow(ctx, `SELECT binding_sha256,submission_sha256 FROM space_join_attempts WHERE id=$1`, legacyA.ID).Scan(&binding, &submission))
	basis := JoinLegacyDecisionBasis{Context: JoinLegacyDecisionContext{Kind: JoinLegacyReady, OperationOrigin: JoinOperationServerNoKey, SpaceID: space.ID, AttemptID: legacyA.ID, AttemptBindingSHA256: binding, SubmissionSHA256: submission, ReadyEvidenceSHA256: legacyA.EvidenceSHA256, ActorAccountID: legacyInput.AccountID, ActorProfileID: legacyInput.ProfileID, OperationID: uuid.New(), RequestMethod: spacev1.SpaceService_JoinSpace_FullMethodName, RequestSHA256: bytes.Repeat([]byte{7}, 32)}, ApplicantObservationSHA256: bytes.Repeat([]byte{8}, 32)}
	sha, err := basis.SHA256()
	require.NoError(t, err)
	raw, err := json.Marshal(basis)
	require.NoError(t, err)
	c := basis.Context
	_, err = pool.Exec(ctx, `INSERT INTO space_join_operations(actor_profile_id,operation_id,actor_account_id,space_id,request_method,request_sha256,selected_attempt_id,state,decision_basis,decision_basis_sha256,basis_version,operation_origin) VALUES($1,$2,$3,$4,$5,$6,$7,'DECIDED',$8,$9,2,2)`, c.ActorProfileID, c.OperationID, c.ActorAccountID, c.SpaceID, c.RequestMethod, c.RequestSHA256, c.AttemptID, raw, sha[:])
	require.NoError(t, err)
	loaded, err := st.LoadVersionedJoinDecisionBasis(ctx, sha[:])
	require.NoError(t, err)
	require.EqualValues(t, 2, loaded.Version())
	_, err = loaded.V1()
	require.ErrorIs(t, err, ErrJoinAttemptUnavailable)
	view, err := loaded.LegacyV2()
	require.NoError(t, err)
	require.Equal(t, basis, view)
	view.Context.RequestSHA256[0] ^= 1
	second, err := loaded.LegacyV2()
	require.NoError(t, err)
	require.Equal(t, basis, second, "accessor must copy private historical bytes")
	_, err = st.LoadJoinDecisionBasis(ctx, sha[:])
	require.ErrorIs(t, err, ErrJoinAttemptUnavailable, "v2 never casts into v1")
	old, err := st.LoadVersionedJoinDecisionBasis(ctx, v1SHA)
	require.NoError(t, err)
	require.EqualValues(t, 1, old.Version())
	oldView, err := old.V1()
	require.NoError(t, err)
	require.Equal(t, v1, oldView)
	_, err = old.LegacyV2()
	require.ErrorIs(t, err, ErrJoinAttemptUnavailable)
	ref := JoinAttemptReference{AttemptID: legacyA.ID, EvidenceSHA256: legacyA.EvidenceSHA256}
	publication := uuid.New()
	require.ErrorIs(t, st.VerifyVersionedJoinDecisionBasis(ctx, legacyInput, ref, sha[:], legacyA.PolicyVersion, publication), ErrJoinAttemptUnavailable)
	verify := func(p uuid.UUID) error {
		return st.withOwnershipScope(ctx, []uuid.UUID{space.ID}, func(local *SpaceStore) error {
			return local.VerifyVersionedJoinDecisionBasis(ctx, legacyInput, ref, sha[:], legacyA.PolicyVersion, p)
		})
	}
	require.NoError(t, verify(publication))
	require.NoError(t, verify(publication))
	require.ErrorIs(t, verify(uuid.New()), ErrJoinAttemptConflict)
	var pinned uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT publication_operation_id FROM space_join_operations WHERE decision_basis_sha256=$1`, sha[:]).Scan(&pinned))
	require.Equal(t, publication, pinned)
	// These markers are storage-only assertions. Do not commit them without the
	// actual native member/head/count/use/outcome transaction owned by186.
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)
	require.NoError(t, lockAndCheckOwnershipScope(ctx, tx, []uuid.UUID{space.ID}))
	local := *st
	local.tx = tx
	require.ErrorIs(t, local.MarkVersionedJoinDecisionBasisPublished(ctx, ref, sha[:], publication), ErrJoinAttemptUnavailable)
	require.NoError(t, local.MarkJoinAttemptPublished(ctx, ref, publication))
	require.NoError(t, local.MarkVersionedJoinDecisionBasisPublished(ctx, ref, sha[:], publication))
	require.NoError(t, tx.Rollback(ctx))
	oldRef := JoinAttemptReference{AttemptID: a.ID, EvidenceSHA256: a.EvidenceSHA256}
	require.NoError(t, st.withOwnershipScope(ctx, []uuid.UUID{space.ID}, func(local *SpaceStore) error {
		return local.VerifyVersionedJoinDecisionBasis(ctx, in, oldRef, v1SHA, a.PolicyVersion, uuid.New())
	}))
	var empty ValidatedJoinDecisionBasis
	_, err = empty.V1()
	require.ErrorIs(t, err, ErrJoinAttemptUnavailable)
	_, err = empty.LegacyV2()
	require.ErrorIs(t, err, ErrJoinAttemptUnavailable)
}
