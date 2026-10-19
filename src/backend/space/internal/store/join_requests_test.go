package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"voice/backend/space/internal/entrypolicy"
)

func applyJoinAttemptMigration(t *testing.T, st *SpaceStore) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), "src/backend/migrations/space_db/000035_join_attempts.up.sql"))
	require.NoError(t, err)
	_, err = st.Pool.Exec(context.Background(), string(raw))
	require.NoError(t, err)
}

// A private Linux runner can provide its own PostgreSQL without host ports or
// nested Docker. Each test still owns a separate real database, never a mock.
func startJoinAttemptPostgres(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("BE010_TEST_DATABASE_DSN")
	if dsn == "" {
		return startSpacePostgresForStoreTest(t, ctx)
	}
	admin, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	name := "be010_" + uuid.New().String()[:8]
	_, err = admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize())
	require.NoError(t, err)
	config, err := pgxpool.ParseConfig(dsn)
	require.NoError(t, err)
	config.ConnConfig.Database = name
	pool, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	t.Cleanup(func() {
		pool.Close()
		_, _ = admin.Exec(context.Background(), "DROP DATABASE "+pgx.Identifier{name}.Sanitize()+" WITH (FORCE)")
		admin.Close()
	})
	return pool
}

func TestJoinAttempt_ImmutableQuestionsAndIndependentManual(t *testing.T) {
	ctx := context.Background()
	pool := startJoinAttemptPostgres(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}
	owner := uuid.New()
	space, err := st.CreateSpace(ctx, owner, "Questions", "", "public")
	require.NoError(t, err)
	applyEntryPolicyMigration(t, st)
	applyJoinAttemptMigration(t, st)
	question := uuid.New()
	policy := entrypolicy.Policy{SchemaVersion: 1, Questions: []entrypolicy.Question{{ID: question, Prompt: "2x2", Kind: "exact_text", ExpectedAnswer: "4"}}}
	_, err = st.SetEntryPolicy(ctx, SetEntryPolicyInput{SpaceID: space.ID, ActorProfileID: owner, OperationID: uuid.New(), ExpectedVersion: 1, Policy: policy})
	require.NoError(t, err)
	in := JoinAttemptInput{SpaceID: space.ID, AccountID: uuid.New(), ProfileID: uuid.New()}
	attempt, err := st.StartJoinAttempt(ctx, in)
	require.NoError(t, err, "questions must start a durable collecting attempt, not legacy admission-required")
	require.Equal(t, "COLLECTING", attempt.State)
	reused, err := st.StartJoinAttempt(ctx, in)
	require.NoError(t, err)
	require.Equal(t, attempt.ID, reused.ID)
	require.Equal(t, attempt.CollectionDeadline, reused.CollectionDeadline)
	ready, err := st.SubmitJoinAttempt(ctx, in, attempt.ID, []entrypolicy.Answer{{QuestionID: question, Value: " 4\n"}})
	require.NoError(t, err)
	require.Equal(t, "READY", ready.State, "questions never imply manual review")
	retry, err := st.SubmitJoinAttempt(ctx, in, attempt.ID, []entrypolicy.Answer{{QuestionID: question, Value: "4"}})
	require.NoError(t, err)
	require.Equal(t, ready.EvidenceSHA256, retry.EvidenceSHA256)
	_, err = st.SubmitJoinAttempt(ctx, in, attempt.ID, []entrypolicy.Answer{{QuestionID: question, Value: "04"}})
	require.Error(t, err, "cannot replace accepted immutable submission")
	var members int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM space_members WHERE space_id=$1 AND profile_id=$2`, space.ID, in.ProfileID).Scan(&members))
	require.Zero(t, members, "automatic pass is evidence, not membership publication")
}

func TestJoinAttempt_WrongAnswerTerminalAndManualIndependent(t *testing.T) {
	ctx := context.Background()
	pool := startJoinAttemptPostgres(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}
	owner := uuid.New()
	space, err := st.CreateSpace(ctx, owner, "Manual AND", "", "public")
	require.NoError(t, err)
	applyEntryPolicyMigration(t, st)
	applyJoinAttemptMigration(t, st)
	question := uuid.New()
	policy := entrypolicy.Policy{SchemaVersion: 1, RequireManualApproval: true, Questions: []entrypolicy.Question{{ID: question, Prompt: "2x2", Kind: "exact_text", ExpectedAnswer: "4"}}}
	_, err = st.SetEntryPolicy(ctx, SetEntryPolicyInput{SpaceID: space.ID, ActorProfileID: owner, OperationID: uuid.New(), ExpectedVersion: 1, Policy: policy})
	require.NoError(t, err)
	in := JoinAttemptInput{SpaceID: space.ID, AccountID: uuid.New(), ProfileID: uuid.New()}
	a, err := st.StartJoinAttempt(ctx, in)
	require.NoError(t, err)
	failed, err := st.SubmitJoinAttempt(ctx, in, a.ID, []entrypolicy.Answer{{QuestionID: question, Value: "04"}})
	require.ErrorIs(t, err, entrypolicy.ErrDenied)
	require.Equal(t, "FAILED", failed.State)
	_, err = st.SubmitJoinAttempt(ctx, in, a.ID, []entrypolicy.Answer{{QuestionID: question, Value: "4"}})
	require.ErrorIs(t, err, ErrJoinAttemptConflict)
	fresh, err := st.StartJoinAttempt(ctx, in)
	require.NoError(t, err)
	require.NotEqual(t, a.ID, fresh.ID)
	pending, err := st.SubmitJoinAttempt(ctx, in, fresh.ID, []entrypolicy.Answer{{QuestionID: question, Value: "4"}})
	require.NoError(t, err)
	require.Equal(t, "PENDING", pending.State)
	require.Empty(t, pending.EvidenceSHA256, "automatic pass cannot synthesize manual approval")
	approval := JoinAttemptApprovalInput{SpaceID: space.ID, AttemptID: pending.ID, ActorAccountID: uuid.New(), ActorProfileID: owner, OperationID: uuid.New()}
	_, err = st.ApproveJoinAttempt(ctx, approval, nil)
	require.ErrorIs(t, err, ErrJoinAttemptUnavailable, "missing fresh Role authority cannot approve")
	guardCalls := 0
	_, err = st.ReserveJoinAttemptApproval(ctx, approval)
	require.NoError(t, err)
	guard := func(_ context.Context, scoped *SpaceStore, got JoinAttemptApprovalInput) error {
		guardCalls++
		require.NotNil(t, scoped.tx)
		require.Equal(t, approval, got)
		return nil // Fixture owns a freshly authorized reviewer; transport is not mocked as proof.
	}
	approved, err := st.ApproveJoinAttempt(ctx, approval, guard)
	require.NoError(t, err)
	require.Equal(t, "READY", approved.State)
	require.Len(t, approved.EvidenceSHA256, 32)
	require.Positive(t, guardCalls)
	authorizedCalls := guardCalls
	replay, err := st.ApproveJoinAttempt(ctx, approval, nil)
	require.NoError(t, err, "original-actor exact completed replay requires no new guard/effect")
	require.Equal(t, approved, replay)
	changed := approval
	changed.AttemptID = uuid.New()
	_, err = st.ApproveJoinAttempt(ctx, changed, guard)
	require.ErrorIs(t, err, ErrJoinAttemptConflict, "selected attempt is immutable before any owner call")
	require.Equal(t, authorizedCalls, guardCalls)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM space_members WHERE space_id=$1 AND profile_id=$2`, in.SpaceID, in.ProfileID).Scan(&count))
	require.Zero(t, count)
}

func TestJoinAttempt_ApprovalBudgetExpiresDuringJournalWrite(t *testing.T) {
	ctx := context.Background()
	pool := startJoinAttemptPostgres(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}
	owner := uuid.New()
	space, err := st.CreateSpace(ctx, owner, "Budget", "", "public")
	require.NoError(t, err)
	applyEntryPolicyMigration(t, st)
	applyJoinAttemptMigration(t, st)
	_, err = st.SetEntryPolicy(ctx, SetEntryPolicyInput{SpaceID: space.ID, ActorProfileID: owner, OperationID: uuid.New(), ExpectedVersion: 1, Policy: entrypolicy.Policy{SchemaVersion: 1, RequireManualApproval: true, Questions: []entrypolicy.Question{}}})
	require.NoError(t, err)
	in := JoinAttemptInput{SpaceID: space.ID, AccountID: uuid.New(), ProfileID: uuid.New()}
	a, err := st.StartJoinAttempt(ctx, in)
	require.NoError(t, err)
	a, err = st.SubmitJoinAttempt(ctx, in, a.ID, nil)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `CREATE FUNCTION wait_join_approval_test() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN PERFORM pg_sleep(0.2); RETURN NEW; END $$;
CREATE TRIGGER wait_join_approval_test BEFORE UPDATE ON space_join_approval_operations FOR EACH ROW EXECUTE FUNCTION wait_join_approval_test()`)
	require.NoError(t, err)
	approval := JoinAttemptApprovalInput{SpaceID: space.ID, AttemptID: a.ID, ActorAccountID: uuid.New(), ActorProfileID: owner, OperationID: uuid.New()}
	var observed time.Time
	_, err = st.ReserveJoinAttemptApproval(ctx, approval)
	require.NoError(t, err)
	guard := func(context.Context, *SpaceStore, JoinAttemptApprovalInput) error {
		if observed.IsZero() {
			observed = time.Now().Add(-4900 * time.Millisecond)
		}
		if time.Since(observed) >= 5*time.Second {
			return ErrJoinAttemptUnavailable
		}
		return nil
	}
	_, err = st.ApproveJoinAttempt(ctx, approval, guard)
	require.ErrorIs(t, err, ErrJoinAttemptUnavailable)
	var state string
	var operations int
	require.NoError(t, pool.QueryRow(ctx, `SELECT state FROM space_join_attempts WHERE id=$1`, a.ID).Scan(&state))
	require.Equal(t, "PENDING", state, "expired owner snapshot rolls back READY and approval identity")
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM space_join_approval_operations WHERE state='COMPLETED'`).Scan(&operations))
	require.Zero(t, operations)
}

func TestJoinAttempt_ExactPublicationPinAndPolicyRevision(t *testing.T) {
	ctx := context.Background()
	pool := startJoinAttemptPostgres(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}
	owner := uuid.New()
	space, err := st.CreateSpace(ctx, owner, "Proof binding", "", "public")
	require.NoError(t, err)
	applyEntryPolicyMigration(t, st)
	applyJoinAttemptMigration(t, st)
	in := JoinAttemptInput{SpaceID: space.ID, AccountID: uuid.New(), ProfileID: uuid.New()}
	a, err := st.StartJoinAttempt(ctx, in)
	require.NoError(t, err)
	a, err = st.SubmitJoinAttempt(ctx, in, a.ID, nil)
	require.NoError(t, err)
	ref := JoinAttemptReference{AttemptID: a.ID, EvidenceSHA256: a.EvidenceSHA256}
	publication := uuid.New()
	verify := func(input JoinAttemptInput, p uuid.UUID) error {
		return st.withOwnershipScope(ctx, []uuid.UUID{space.ID}, func(scoped *SpaceStore) error {
			return scoped.VerifyJoinAttemptAdmission(ctx, input, ref, a.PolicyVersion, p)
		})
	}
	require.NoError(t, verify(in, publication))
	require.NoError(t, verify(in, publication))
	require.ErrorIs(t, verify(in, uuid.New()), ErrJoinAttemptConflict)
	foreign := in
	foreign.AccountID = uuid.New()
	require.ErrorIs(t, verify(foreign, publication), ErrJoinAttemptDenied)
	_, err = st.SetEntryPolicy(ctx, SetEntryPolicyInput{SpaceID: space.ID, ActorProfileID: owner, OperationID: uuid.New(), ExpectedVersion: 1, Policy: entrypolicy.Policy{SchemaVersion: 1, RequireManualApproval: true, Questions: []entrypolicy.Question{}}})
	require.NoError(t, err)
	require.ErrorIs(t, verify(in, publication), ErrJoinAttemptDenied)
}

func TestJoinAttempt_ApprovalSelectionReservedBeforeOwnerCalls(t *testing.T) {
	ctx := context.Background()
	pool := startJoinAttemptPostgres(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}
	owner := uuid.New()
	space, err := st.CreateSpace(ctx, owner, "Exact selection", "", "public")
	require.NoError(t, err)
	applyEntryPolicyMigration(t, st)
	applyJoinAttemptMigration(t, st)
	_, err = st.SetEntryPolicy(ctx, SetEntryPolicyInput{SpaceID: space.ID, ActorProfileID: owner, OperationID: uuid.New(), ExpectedVersion: 1, Policy: entrypolicy.Policy{SchemaVersion: 1, RequireManualApproval: true, Questions: []entrypolicy.Question{}}})
	require.NoError(t, err)
	in := JoinAttemptInput{SpaceID: space.ID, AccountID: uuid.New(), ProfileID: uuid.New()}
	attempts := make([]*JoinAttempt, 0, 2)
	for range 2 {
		invite, err := st.CreateInvite(ctx, CreateInviteInput{SpaceID: space.ID, CreatorProfileID: owner})
		require.NoError(t, err)
		in.InviteID = &invite.ID
		a, err := st.StartJoinAttempt(ctx, in)
		require.NoError(t, err)
		a, err = st.SubmitJoinAttempt(ctx, in, a.ID, nil)
		require.NoError(t, err)
		attempts = append(attempts, a)
	}
	approval := JoinAttemptApprovalInput{SpaceID: space.ID, AttemptID: attempts[0].ID, ActorAccountID: uuid.New(), ActorProfileID: owner, OperationID: uuid.New()}
	reserved, err := st.ReserveJoinAttemptApproval(ctx, approval)
	require.NoError(t, err, "selection must survive process loss before any owner observation")
	require.Equal(t, "RESERVED", reserved.State)
	require.Equal(t, approval, reserved.Input)
	changed := approval
	changed.AttemptID = attempts[1].ID
	_, err = st.ReserveJoinAttemptApproval(ctx, changed)
	require.ErrorIs(t, err, ErrJoinAttemptConflict)
	second, err := pgxpool.NewWithConfig(ctx, pool.Config())
	require.NoError(t, err)
	t.Cleanup(second.Close)
	restarted := &SpaceStore{Pool: second}
	saved, err := restarted.ReserveJoinAttemptApproval(ctx, approval)
	require.NoError(t, err)
	require.Equal(t, reserved, saved)
	var selected uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT attempt_id FROM space_join_approval_operations WHERE actor_profile_id=$1 AND operation_id=$2`, approval.ActorProfileID, approval.OperationID).Scan(&selected))
	require.Equal(t, attempts[0].ID, selected)
}

func TestJoinAttempt_ConcurrentStartUsesDatabaseFence(t *testing.T) {
	ctx := context.Background()
	pool := startJoinAttemptPostgres(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}
	owner := uuid.New()
	space, err := st.CreateSpace(ctx, owner, "Start race", "", "public")
	require.NoError(t, err)
	applyEntryPolicyMigration(t, st)
	applyJoinAttemptMigration(t, st)
	invite, err := st.CreateInvite(ctx, CreateInviteInput{SpaceID: space.ID, CreatorProfileID: owner})
	require.NoError(t, err)
	second, err := pgxpool.NewWithConfig(ctx, pool.Config())
	require.NoError(t, err)
	t.Cleanup(second.Close)
	var firstDB, secondDB string
	require.NoError(t, pool.QueryRow(ctx, `SELECT current_database()`).Scan(&firstDB))
	require.NoError(t, second.QueryRow(ctx, `SELECT current_database()`).Scan(&secondDB))
	require.Equal(t, firstDB, secondDB, "both independent pools address the same fixture database")
	blocker, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = blocker.Rollback(context.Background()) }()
	_, err = blocker.Exec(ctx, `SELECT pg_advisory_xact_lock($1::bigint)`, spaceMutationAdvisoryKey(space.ID))
	require.NoError(t, err)
	in := JoinAttemptInput{SpaceID: space.ID, AccountID: uuid.New(), ProfileID: uuid.New(), InviteID: &invite.ID}
	type result struct {
		attempt *JoinAttempt
		err     error
	}
	results := make(chan result, 2)
	for _, candidate := range []*SpaceStore{st, {Pool: second}} {
		go func(candidate *SpaceStore) { a, err := candidate.StartJoinAttempt(ctx, in); results <- result{a, err} }(candidate)
	}
	require.Eventually(t, func() bool {
		var waiting int
		err := pool.QueryRow(ctx, `SELECT count(*) FROM pg_locks WHERE locktype='advisory' AND NOT granted AND database=(SELECT oid FROM pg_database WHERE datname=current_database())`).Scan(&waiting)
		return err == nil && waiting == 2
	}, 5*time.Second, 10*time.Millisecond, "both writers must reach the held database fence")
	require.NoError(t, blocker.Commit(ctx))
	first, other := <-results, <-results
	require.NoError(t, first.err)
	require.NoError(t, other.err)
	require.Equal(t, first.attempt.ID, other.attempt.ID)
	require.Equal(t, first.attempt.CollectionDeadline, other.attempt.CollectionDeadline)
	var attempts, members int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM space_join_attempts WHERE space_id=$1 AND profile_id=$2`, in.SpaceID, in.ProfileID).Scan(&attempts))
	require.Equal(t, 1, attempts)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM space_members WHERE space_id=$1 AND profile_id=$2`, in.SpaceID, in.ProfileID).Scan(&members))
	require.Zero(t, members)
	savedInvite, err := st.GetInviteByID(ctx, invite.ID)
	require.NoError(t, err)
	require.Zero(t, savedInvite.UseCount)
}

func TestJoinAttempt_PublicationMarkerSharesOuterRollback(t *testing.T) {
	ctx := context.Background()
	pool := startJoinAttemptPostgres(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}
	space, err := st.CreateSpace(ctx, uuid.New(), "Publication marker", "", "public")
	require.NoError(t, err)
	applyEntryPolicyMigration(t, st)
	applyJoinAttemptMigration(t, st)
	in := JoinAttemptInput{SpaceID: space.ID, AccountID: uuid.New(), ProfileID: uuid.New()}
	a, err := st.StartJoinAttempt(ctx, in)
	require.NoError(t, err)
	a, err = st.SubmitJoinAttempt(ctx, in, a.ID, nil)
	require.NoError(t, err)
	ref := JoinAttemptReference{AttemptID: a.ID, EvidenceSHA256: a.EvidenceSHA256}
	publication := uuid.New()
	require.NoError(t, st.withOwnershipScope(ctx, []uuid.UUID{space.ID}, func(scoped *SpaceStore) error {
		return scoped.VerifyJoinAttemptAdmission(ctx, in, ref, a.PolicyVersion, publication)
	}))
	require.ErrorIs(t, st.MarkJoinAttemptPublished(ctx, ref, publication), ErrJoinAttemptUnavailable, "marker cannot run outside the owning native transaction")
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()
	require.NoError(t, lockAndCheckOwnershipScope(ctx, tx, []uuid.UUID{space.ID}))
	scoped := *st
	scoped.tx = tx
	require.ErrorIs(t, scoped.MarkJoinAttemptPublished(ctx, ref, uuid.New()), ErrJoinAttemptUnavailable)
	require.NoError(t, scoped.MarkJoinAttemptPublished(ctx, ref, publication))
	var state string
	require.NoError(t, tx.QueryRow(ctx, `SELECT state FROM space_join_attempts WHERE id=$1`, a.ID).Scan(&state))
	require.Equal(t, "PUBLISHED", state)
	// Deliberately never commit this bookkeeping-only fixture. Positive native
	// member/count/use/Role authority is a separate assembled producer test.
	require.NoError(t, tx.Rollback(ctx))
	require.NoError(t, pool.QueryRow(ctx, `SELECT state FROM space_join_attempts WHERE id=$1`, a.ID).Scan(&state))
	require.Equal(t, "CLAIMED", state)
}

func TestJoinAttempt_AccountBanInvalidatesOnlyExactUnboundAttempts(t *testing.T) {
	ctx := context.Background()
	pool := startJoinAttemptPostgres(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}
	owner := uuid.New()
	space, err := st.CreateSpace(ctx, owner, "Ban attempts", "", "public")
	require.NoError(t, err)
	applyEntryPolicyMigration(t, st)
	applyJoinAttemptMigration(t, st)
	in := JoinAttemptInput{SpaceID: space.ID, AccountID: uuid.New(), ProfileID: uuid.New()}
	collecting, err := st.StartJoinAttempt(ctx, in)
	require.NoError(t, err)
	in.ProfileID = uuid.New()
	ready, err := st.StartJoinAttempt(ctx, in)
	require.NoError(t, err)
	ready, err = st.SubmitJoinAttempt(ctx, in, ready.ID, nil)
	require.NoError(t, err)
	_, err = st.SetEntryPolicy(ctx, SetEntryPolicyInput{SpaceID: space.ID, ActorProfileID: owner, OperationID: uuid.New(), ExpectedVersion: 1, Policy: entrypolicy.Policy{SchemaVersion: 1, RequireManualApproval: true, Questions: []entrypolicy.Question{}}})
	require.NoError(t, err)
	pendingInput := in
	pendingInput.ProfileID = uuid.New()
	pending, err := st.StartJoinAttempt(ctx, pendingInput)
	require.NoError(t, err)
	pending, err = st.SubmitJoinAttempt(ctx, pendingInput, pending.ID, nil)
	require.NoError(t, err)
	require.Equal(t, "PENDING", pending.State)
	other := in
	other.AccountID, other.ProfileID = uuid.New(), uuid.New()
	unrelated, err := st.StartJoinAttempt(ctx, other)
	require.NoError(t, err)
	require.ErrorIs(t, st.InvalidateJoinAttemptsForAccount(ctx, space.ID, in.AccountID), ErrJoinAttemptUnavailable)
	err = st.withOwnershipScope(ctx, []uuid.UUID{space.ID}, func(scoped *SpaceStore) error {
		return scoped.InvalidateJoinAttemptsForAccount(ctx, space.ID, in.AccountID)
	})
	require.ErrorIs(t, err, ErrJoinAttemptUnavailable, "actual ban row is mandatory")
	err = st.withOwnershipScope(ctx, []uuid.UUID{space.ID}, func(scoped *SpaceStore) error {
		_, err := scoped.db().Exec(ctx, `INSERT INTO space_bans(space_id,account_id,banned_by_profile_id) VALUES($1,$2,$3)`, space.ID, in.AccountID, owner)
		if err != nil {
			return err
		}
		return scoped.InvalidateJoinAttemptsForAccount(ctx, space.ID, in.AccountID)
	})
	require.NoError(t, err)
	for _, id := range []uuid.UUID{collecting.ID, ready.ID, pending.ID} {
		var state string
		require.NoError(t, pool.QueryRow(ctx, `SELECT state FROM space_join_attempts WHERE id=$1`, id).Scan(&state))
		require.Equal(t, "INVALIDATED", state)
	}
	var state string
	require.NoError(t, pool.QueryRow(ctx, `SELECT state FROM space_join_attempts WHERE id=$1`, unrelated.ID).Scan(&state))
	require.Equal(t, "COLLECTING", state)
	_, err = pool.Exec(ctx, `DELETE FROM space_bans WHERE space_id=$1 AND account_id=$2`, space.ID, in.AccountID)
	require.NoError(t, err)
	_, err = st.SubmitJoinAttempt(ctx, in, ready.ID, nil)
	require.ErrorIs(t, err, ErrJoinAttemptDenied, "unban must not resurrect old evidence")
}

func TestJoinAttempt_AccountBanCannotGuessClaimedPublicationRetirement(t *testing.T) {
	ctx := context.Background()
	pool := startJoinAttemptPostgres(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}
	owner := uuid.New()
	space, err := st.CreateSpace(ctx, owner, "Claimed ban", "", "public")
	require.NoError(t, err)
	applyEntryPolicyMigration(t, st)
	applyJoinAttemptMigration(t, st)
	in := JoinAttemptInput{SpaceID: space.ID, AccountID: uuid.New(), ProfileID: uuid.New()}
	a, err := st.StartJoinAttempt(ctx, in)
	require.NoError(t, err)
	a, err = st.SubmitJoinAttempt(ctx, in, a.ID, nil)
	require.NoError(t, err)
	ref := JoinAttemptReference{AttemptID: a.ID, EvidenceSHA256: a.EvidenceSHA256}
	require.NoError(t, st.withOwnershipScope(ctx, []uuid.UUID{space.ID}, func(scoped *SpaceStore) error {
		return scoped.VerifyJoinAttemptAdmission(ctx, in, ref, a.PolicyVersion, uuid.New())
	}))
	otherInput := in
	otherInput.ProfileID = uuid.New()
	unbound, err := st.StartJoinAttempt(ctx, otherInput)
	require.NoError(t, err)
	err = st.withOwnershipScope(ctx, []uuid.UUID{space.ID}, func(scoped *SpaceStore) error {
		_, err := scoped.db().Exec(ctx, `INSERT INTO space_bans(space_id,account_id,banned_by_profile_id) VALUES($1,$2,$3)`, space.ID, in.AccountID, owner)
		if err != nil {
			return err
		}
		return scoped.InvalidateJoinAttemptsForAccount(ctx, space.ID, in.AccountID)
	})
	require.ErrorIs(t, err, ErrJoinAttemptUnavailable)
	var state string
	require.NoError(t, pool.QueryRow(ctx, `SELECT state FROM space_join_attempts WHERE id=$1`, unbound.ID).Scan(&state))
	require.Equal(t, "COLLECTING", state, "unknown native retirement must preserve all attempts")
	var banned bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM space_bans WHERE space_id=$1 AND account_id=$2)`, space.ID, in.AccountID).Scan(&banned))
	require.False(t, banned, "same outer transaction must roll back the ban too")
}
