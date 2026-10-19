package store

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"voice/backend/space/internal/entrypolicy"
)

type joinGroupFixture struct {
	ctx                            context.Context
	store                          *SpaceStore
	space, owner, account, profile uuid.UUID
}

func newJoinGroupFixture(t *testing.T) joinGroupFixture {
	t.Helper()
	ctx := context.Background()
	pool := startJoinAttemptPostgres(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}
	owner := uuid.New()
	space, err := st.CreateSpace(ctx, owner, "Exact review group", "", "public")
	require.NoError(t, err)
	applyEntryPolicyMigration(t, st)
	// Source order matters:32 must not require future35/36 tables to exist.
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), "src/backend/migrations/space_db/000037_join_request_groups_forward.up.sql"))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(raw))
	require.NoError(t, err)
	applyJoinAttemptMigration(t, st)
	applyJoinOperationMigration(t, st)
	_, err = st.SetEntryPolicy(ctx, SetEntryPolicyInput{SpaceID: space.ID, ActorProfileID: owner, OperationID: uuid.New(), ExpectedVersion: 1, Policy: entrypolicy.Policy{SchemaVersion: 1, RequireManualApproval: true, Questions: []entrypolicy.Question{}}})
	require.NoError(t, err)
	return joinGroupFixture{ctx: ctx, store: st, space: space.ID, owner: owner, account: uuid.New(), profile: uuid.New()}
}

func (f joinGroupFixture) child(t *testing.T) (*JoinAttempt, *JoinRequestGroup) {
	t.Helper()
	inv, err := f.store.CreateInvite(f.ctx, CreateInviteInput{SpaceID: f.space, CreatorProfileID: f.owner})
	require.NoError(t, err)
	in := JoinAttemptInput{SpaceID: f.space, AccountID: f.account, ProfileID: f.profile, InviteID: &inv.ID}
	var attempt *JoinAttempt
	var group *JoinRequestGroup
	err = f.store.withOwnershipScope(f.ctx, []uuid.UUID{f.space}, func(scoped *SpaceStore) error {
		var err error
		attempt, err = scoped.StartJoinAttempt(f.ctx, in)
		if err != nil {
			return err
		}
		attempt, err = scoped.SubmitJoinAttempt(f.ctx, in, attempt.ID, nil)
		if err != nil {
			return err
		}
		group, err = scoped.AttachJoinRequestAttempt(f.ctx, attempt.ID)
		return err
	})
	require.NoError(t, err, "successful pending submission must atomically expose its real parent Q")
	return attempt, group
}

func (f joinGroupFixture) operation() JoinOperationInput {
	return JoinOperationInput{SpaceID: f.space, ActorAccountID: uuid.New(), ActorProfileID: f.owner, OperationID: uuid.New(), Method: JoinOperationApprove, RequestSHA256: joinDigest("group-test", uuid.New())}
}

func TestJoinRequestGroupPinsOneChildBeforeOwnersAndRetainsItAcrossRestart(t *testing.T) {
	f := newJoinGroupFixture(t)
	a, q := f.child(t)
	require.NotEqual(t, a.ID, q.ID, "Q is a real server group, never inferred Q=A")
	op := f.operation()
	selected, err := f.store.ResolveJoinRequestApproval(f.ctx, op, q.ID, nil)
	require.NoError(t, err)
	require.Equal(t, a.ID, *selected.AttemptID)
	b, otherGroup := f.child(t)
	require.Equal(t, q.ID, otherGroup.ID, "different invites retain distinct A within the same exact cohort")
	require.NotEqual(t, a.ID, b.ID)
	restarted, err := pgxpool.NewWithConfig(f.ctx, f.store.Pool.Config())
	require.NoError(t, err)
	defer restarted.Close()
	selected, err = (&SpaceStore{Pool: restarted}).ResolveJoinRequestApproval(f.ctx, op, q.ID, nil)
	require.NoError(t, err)
	require.Equal(t, a.ID, *selected.AttemptID, "B arriving after reservation never changes saved A")
	_, err = f.store.ResolveJoinRequestApproval(f.ctx, f.operation(), q.ID, nil)
	require.ErrorIs(t, err, ErrJoinRequestAmbiguous)
	_, err = f.store.ResolveJoinRequestApproval(f.ctx, op, q.ID, &b.ID)
	require.ErrorIs(t, err, ErrJoinAttemptConflict)
	explicit, err := f.store.ResolveJoinRequestApproval(f.ctx, f.operation(), q.ID, &b.ID)
	require.NoError(t, err)
	require.Equal(t, b.ID, *explicit.AttemptID)
	var members, approvals int
	require.NoError(t, f.store.Pool.QueryRow(f.ctx, `SELECT (SELECT count(*) FROM space_members WHERE space_id=$1 AND profile_id=$2),(SELECT count(*) FROM space_join_approval_operations)`, f.space, f.profile).Scan(&members, &approvals))
	require.Zero(t, members)
	require.Zero(t, approvals, "group selection itself is not a review approval")
}

func TestJoinRequestGroupScopedVerificationAndAttachRollBackWithOwnerTransaction(t *testing.T) {
	f := newJoinGroupFixture(t)
	a, q := f.child(t)
	op := f.operation()
	_, err := f.store.ResolveJoinRequestApproval(f.ctx, op, q.ID, &a.ID)
	require.NoError(t, err)
	require.ErrorIs(t, f.store.VerifyJoinRequestApprovalSelection(f.ctx, op, q.ID, a.ID), ErrJoinAttemptUnavailable)
	require.NoError(t, f.store.withOwnershipScope(f.ctx, []uuid.UUID{f.space}, func(scoped *SpaceStore) error {
		return scoped.VerifyJoinRequestApprovalSelection(f.ctx, op, q.ID, a.ID)
	}))
	forced := errors.New("force outer rollback")
	var rolledBack uuid.UUID
	err = f.store.withOwnershipScope(f.ctx, []uuid.UUID{f.space}, func(scoped *SpaceStore) error {
		inv, err := scoped.CreateInvite(f.ctx, CreateInviteInput{SpaceID: f.space, CreatorProfileID: f.owner})
		if err != nil {
			return err
		}
		in := JoinAttemptInput{SpaceID: f.space, AccountID: uuid.New(), ProfileID: uuid.New(), InviteID: &inv.ID}
		attempt, err := scoped.StartJoinAttempt(f.ctx, in)
		if err != nil {
			return err
		}
		attempt, err = scoped.SubmitJoinAttempt(f.ctx, in, attempt.ID, nil)
		if err != nil {
			return err
		}
		group, err := scoped.AttachJoinRequestAttempt(f.ctx, attempt.ID)
		if err != nil {
			return err
		}
		rolledBack = group.ID
		return forced
	})
	require.ErrorIs(t, err, forced)
	var exists bool
	require.NoError(t, f.store.Pool.QueryRow(f.ctx, `SELECT EXISTS(SELECT 1 FROM space_join_request_groups WHERE id=$1)`, rolledBack).Scan(&exists))
	require.False(t, exists, "pending A and its Q association cannot survive a rolled-back submit")
}

func TestJoinRequestGroupCohortsAndPublicRequestBindingCannotBeSubstituted(t *testing.T) {
	f := newJoinGroupFixture(t)
	a, q := f.child(t)
	foreign := f
	foreign.account = uuid.New()
	foreign.profile = uuid.New()
	other, otherQ := foreign.child(t)
	require.NotEqual(t, q.ID, otherQ.ID)
	_, err := f.store.ResolveJoinRequestApproval(f.ctx, f.operation(), q.ID, &other.ID)
	require.ErrorIs(t, err, ErrJoinAttemptDenied, "explicit A must belong to the named Q")
	_, err = f.store.ResolveJoinRequestApproval(f.ctx, f.operation(), a.ID, &a.ID)
	require.ErrorIs(t, err, ErrJoinAttemptDenied, "an attempt UUID is not a Q alias")
	wrongSpace := f.operation()
	wrongSpace.SpaceID = uuid.New()
	_, err = f.store.ResolveJoinRequestApproval(f.ctx, wrongSpace, q.ID, &a.ID)
	require.ErrorIs(t, err, ErrJoinAttemptDenied)
	op := f.operation()
	_, err = f.store.ResolveJoinRequestApproval(f.ctx, op, q.ID, &a.ID)
	require.NoError(t, err)
	_, err = f.store.ResolveJoinRequestApproval(f.ctx, op, otherQ.ID, &other.ID)
	require.ErrorIs(t, err, ErrJoinAttemptConflict, "even an internal inconsistent body hash cannot change saved Q")
	_, err = f.store.ResolveJoinRequestApproval(f.ctx, op, q.ID, nil)
	require.ErrorIs(t, err, ErrJoinAttemptConflict, "explicit selector presence is part of immutable identity")
	unmapped := f.operation()
	_, err = f.store.ReserveJoinOperation(f.ctx, unmapped, &a.ID)
	require.NoError(t, err)
	_, err = f.store.ResolveJoinRequestApproval(f.ctx, unmapped, q.ID, &a.ID)
	require.ErrorIs(t, err, ErrJoinAttemptUnavailable, "old36 reservation without a Q journal is not retroactively attached")
	policy := entrypolicy.Policy{SchemaVersion: 1, Questions: []entrypolicy.Question{}}
	_, err = f.store.SetEntryPolicy(f.ctx, SetEntryPolicyInput{SpaceID: f.space, ActorProfileID: f.owner, OperationID: uuid.New(), ExpectedVersion: q.PolicyVersion, Policy: policy})
	require.NoError(t, err)
	policy.RequireManualApproval = true
	_, err = f.store.SetEntryPolicy(f.ctx, SetEntryPolicyInput{SpaceID: f.space, ActorProfileID: f.owner, OperationID: uuid.New(), ExpectedVersion: q.PolicyVersion + 1, Policy: policy})
	require.NoError(t, err)
	later, laterQ := f.child(t)
	require.NotEqual(t, q.ID, laterQ.ID, "policy ABA never reuses a parent review case")
	_, err = f.store.ResolveJoinRequestApproval(f.ctx, f.operation(), q.ID, &later.ID)
	require.ErrorIs(t, err, ErrJoinAttemptDenied)
}

func TestJoinRequestGroupConcurrentOperationHasOneImmutableWinner(t *testing.T) {
	f := newJoinGroupFixture(t)
	a, q := f.child(t)
	b, _ := f.child(t)
	base := f.operation()
	start := make(chan struct{})
	type result struct {
		attempt uuid.UUID
		err     error
	}
	results := make(chan result, 2)
	var workers sync.WaitGroup
	for _, id := range []uuid.UUID{a.ID, b.ID} {
		workers.Add(1)
		go func(id uuid.UUID) {
			defer workers.Done()
			op := base
			op.RequestSHA256 = joinDigest("actual differing public body", id)
			<-start
			saved, err := f.store.ResolveJoinRequestApproval(f.ctx, op, q.ID, &id)
			if err == nil {
				results <- result{attempt: *saved.AttemptID}
				return
			}
			results <- result{err: err}
		}(id)
	}
	close(start)
	workers.Wait()
	close(results)
	success, conflict := 0, 0
	var winner uuid.UUID
	for result := range results {
		if result.err == nil {
			success++
			winner = result.attempt
		} else {
			require.ErrorIs(t, result.err, ErrJoinAttemptConflict)
			conflict++
		}
	}
	require.Equal(t, 1, success)
	require.Equal(t, 1, conflict)
	var public, selection uuid.UUID
	require.NoError(t, f.store.Pool.QueryRow(f.ctx, `SELECT o.selected_attempt_id,s.selected_attempt_id FROM space_join_operations o JOIN space_join_request_selections s USING(actor_profile_id,operation_id) WHERE o.actor_profile_id=$1 AND o.operation_id=$2`, base.ActorProfileID, base.OperationID).Scan(&public, &selection))
	require.Equal(t, winner, public)
	require.Equal(t, winner, selection)
}

func TestJoinRequestGroupSelectionInsertFailureRollsBackBothJournals(t *testing.T) {
	f := newJoinGroupFixture(t)
	a, q := f.child(t)
	_, err := f.store.Pool.Exec(f.ctx, `CREATE FUNCTION test_group_selection_failure() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'forced selection failure'; END $$; CREATE TRIGGER test_group_selection_failure BEFORE INSERT ON space_join_request_selections FOR EACH ROW EXECUTE FUNCTION test_group_selection_failure()`)
	require.NoError(t, err)
	op := f.operation()
	_, err = f.store.ResolveJoinRequestApproval(f.ctx, op, q.ID, &a.ID)
	require.Error(t, err)
	var operations, selections int
	require.NoError(t, f.store.Pool.QueryRow(f.ctx, `SELECT (SELECT count(*) FROM space_join_operations),(SELECT count(*) FROM space_join_request_selections)`).Scan(&operations, &selections))
	require.Zero(t, operations)
	require.Zero(t, selections)
	_, err = f.store.Pool.Exec(f.ctx, `DROP TRIGGER test_group_selection_failure ON space_join_request_selections`)
	require.NoError(t, err)
	_, err = f.store.ResolveJoinRequestApproval(f.ctx, op, q.ID, &a.ID)
	require.NoError(t, err, "same operation is retryable because the entire reservation rolled back")
}

func TestJoinRequestGroupInconsistentChildIsNeverFilteredIntoAnotherSelection(t *testing.T) {
	f := newJoinGroupFixture(t)
	a, q := f.child(t)
	b, _ := f.child(t)
	_, err := f.store.Pool.Exec(f.ctx, `UPDATE space_join_attempts SET account_id=$2 WHERE id=$1`, a.ID, uuid.New())
	require.NoError(t, err, "negative corruption fixture is not an authorized source transition")
	_, err = f.store.ResolveJoinRequestApproval(f.ctx, f.operation(), q.ID, &b.ID)
	require.ErrorIs(t, err, ErrJoinAttemptUnavailable, "unknown child identity does not authorize a fallback to B")
}

func TestJoinRequestGroupSQLMappingsCannotBeRewrittenOrDeletedWhileLive(t *testing.T) {
	f := newJoinGroupFixture(t)
	a, q := f.child(t)
	op := f.operation()
	_, err := f.store.ResolveJoinRequestApproval(f.ctx, op, q.ID, &a.ID)
	require.NoError(t, err)
	_, err = f.store.Pool.Exec(f.ctx, `UPDATE space_join_request_groups SET profile_id=$2 WHERE id=$1`, q.ID, uuid.New())
	require.Error(t, err, "a live Q cohort cannot be rewritten after selection")
	_, err = f.store.Pool.Exec(f.ctx, `UPDATE space_join_request_selections SET request_sha256=$3 WHERE actor_profile_id=$1 AND operation_id=$2`, op.ActorProfileID, op.OperationID, joinDigest("changed selection body", uuid.New()))
	require.Error(t, err)
	for _, table := range []string{"space_join_request_selections", "space_join_request_attempts", "space_join_request_groups"} {
		_, err = f.store.Pool.Exec(f.ctx, `DELETE FROM `+table)
		require.Error(t, err, "live selection history is not an independently deletable cache")
		_, err = f.store.Pool.Exec(f.ctx, `TRUNCATE `+table+` CASCADE`)
		require.Error(t, err)
	}
}

func TestJoinRequestGroupLookupReadsOnlyExistingExactAssociation(t *testing.T) {
	f := newJoinGroupFixture(t)
	a, q := f.child(t)
	got, err := f.store.LookupJoinRequestGroupForAttempt(f.ctx, f.space, a.ID)
	require.NoError(t, err)
	require.Equal(t, *q, *got)
	_, err = f.store.LookupJoinRequestGroupForAttempt(f.ctx, f.space, q.ID)
	require.ErrorIs(t, err, ErrJoinAttemptDenied, "Q is never an alias for A")
	_, err = f.store.LookupJoinRequestGroupForAttempt(f.ctx, f.space, uuid.New())
	require.ErrorIs(t, err, ErrJoinAttemptDenied)
	var groups, children, selections, operations int
	require.NoError(t, f.store.Pool.QueryRow(f.ctx, `SELECT (SELECT count(*) FROM space_join_request_groups),(SELECT count(*) FROM space_join_request_attempts),(SELECT count(*) FROM space_join_request_selections),(SELECT count(*) FROM space_join_operations)`).Scan(&groups, &children, &selections, &operations))
	require.Equal(t, 1, groups)
	require.Equal(t, 1, children)
	require.Zero(t, selections)
	require.Zero(t, operations)
}

func TestJoinRequestGroupFreezeRejectsNewSelectionAndFinalVerification(t *testing.T) {
	f := newJoinGroupFixture(t)
	a, q := f.child(t)
	op := f.operation()
	_, err := f.store.ResolveJoinRequestApproval(f.ctx, op, q.ID, &a.ID)
	require.NoError(t, err)
	_, err = f.store.Pool.Exec(f.ctx, `INSERT INTO space_lifecycle_aggregates(space_id,deletion_operation_id,phase,generation) VALUES($1,$2,'FREEZE_PENDING',1)`, f.space, uuid.New())
	require.NoError(t, err)
	_, err = f.store.ResolveJoinRequestApproval(f.ctx, f.operation(), q.ID, &a.ID)
	require.ErrorIs(t, err, ErrLifecycleFrozen)
	_, err = f.store.LookupJoinRequestGroupForAttempt(f.ctx, f.space, a.ID)
	require.ErrorIs(t, err, ErrLifecycleFrozen)
	// The final hook must recheck even when its caller already holds a tx.
	tx, err := f.store.Pool.Begin(f.ctx)
	require.NoError(t, err)
	defer tx.Rollback(f.ctx)
	local := *f.store
	local.tx = tx
	require.ErrorIs(t, local.VerifyJoinRequestApprovalSelection(f.ctx, op, q.ID, a.ID), ErrLifecycleFrozen)
}

func TestJoinRequestGroupDowngradeRefusesEvidenceButAllowsEmptySchema(t *testing.T) {
	for _, populated := range []bool{false, true} {
		t.Run(map[bool]string{false: "empty", true: "evidence"}[populated], func(t *testing.T) {
			f := newJoinGroupFixture(t)
			if populated {
				f.child(t)
			}
			raw, err := os.ReadFile(filepath.Join(repoRoot(t), "src/backend/migrations/space_db/000037_join_request_groups_forward.down.sql"))
			require.NoError(t, err)
			conn, err := f.store.Pool.Acquire(f.ctx)
			require.NoError(t, err)
			_, err = conn.Exec(f.ctx, string(raw))
			if populated {
				var refusal *pgconn.PgError
				require.ErrorAs(t, err, &refusal)
				require.Equal(t, "55000", refusal.Code)
				_, err = conn.Exec(f.ctx, "ROLLBACK")
			}
			require.NoError(t, err)
			conn.Release()
			var exists bool
			require.NoError(t, f.store.Pool.QueryRow(f.ctx, `SELECT to_regclass('space_join_request_groups') IS NOT NULL`).Scan(&exists))
			require.Equal(t, populated, exists)
		})
	}
}

func TestJoinRequestGroupForgedPurgeSettingCannotEraseLiveAssociation(t *testing.T) {
	f := newJoinGroupFixture(t)
	a, q := f.child(t)
	for _, name := range []string{"000014_ownership_outbox_delivery.up.sql", "000015_audit_ledger.up.sql", "000022_lifecycle_runtime.up.sql"} {
		raw, err := os.ReadFile(filepath.Join(repoRoot(t), "src/backend/migrations/space_db", name))
		require.NoError(t, err)
		_, err = f.store.Pool.Exec(f.ctx, string(raw))
		require.NoError(t, err)
	}
	op := f.operation()
	_, err := f.store.ResolveJoinRequestApproval(f.ctx, op, q.ID, &a.ID)
	require.NoError(t, err)
	for _, table := range []string{"space_join_request_selections", "space_join_request_attempts", "space_join_request_groups"} {
		tx, err := f.store.Pool.Begin(f.ctx)
		require.NoError(t, err)
		_, err = tx.Exec(f.ctx, `SET LOCAL voice.audit_delete_mode='space_purge'`)
		require.NoError(t, err)
		_, err = tx.Exec(f.ctx, `DELETE FROM `+table)
		var refusal *pgconn.PgError
		require.ErrorAs(t, err, &refusal)
		require.Equal(t, "55000", refusal.Code, "actual terminal authority, not a session setting or missing function, denies erasure")
		require.NoError(t, tx.Rollback(f.ctx))
	}
}

func TestJoinRequestGroupDowngradeRejectsOldRepeatableReadSnapshot(t *testing.T) {
	f := newJoinGroupFixture(t)
	tx, err := f.store.Pool.BeginTx(f.ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	require.NoError(t, err)
	defer tx.Rollback(f.ctx)
	var before int
	require.NoError(t, tx.QueryRow(f.ctx, `SELECT count(*) FROM space_join_request_groups`).Scan(&before))
	require.Zero(t, before, "pin a real empty snapshot before the concurrent first group commits")
	a, q := f.child(t)
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), "src/backend/migrations/space_db/000037_join_request_groups_forward.down.sql"))
	require.NoError(t, err)
	_, err = tx.Exec(f.ctx, string(raw))
	var refusal *pgconn.PgError
	require.ErrorAs(t, err, &refusal, "LOCK must not let a stale RR snapshot erase committed Q/A evidence")
	require.Equal(t, "55000", refusal.Code)
	require.NoError(t, tx.Rollback(f.ctx))
	got, err := f.store.LookupJoinRequestGroupForAttempt(f.ctx, f.space, a.ID)
	require.NoError(t, err)
	require.Equal(t, q.ID, got.ID)
}
