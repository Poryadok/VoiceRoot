package store

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"voice/backend/space/internal/entrypolicy"
)

func applyStartContextMigration(t *testing.T, st *SpaceStore, name string) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), "src/backend/migrations/space_db", name))
	require.NoError(t, err)
	_, err = st.Pool.Exec(context.Background(), string(raw))
	require.NoError(t, err)
}

func TestJoinStartSelectionDurablePolicyPinAndNoBackfill(t *testing.T) {
	ctx := context.Background()
	pool := startJoinAttemptPostgres(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}
	owner := uuid.New()
	space, err := st.CreateSpace(ctx, owner, "START context", "", "public")
	require.NoError(t, err)
	applyEntryPolicyMigration(t, st)
	applyJoinAttemptMigration(t, st)
	applyJoinOperationMigration(t, st)
	applyStartContextMigration(t, st, "000040_join_decision_basis_v2.up.sql")
	applyStartContextMigration(t, st, "000043_join_public_operation_context.up.sql")
	in := JoinOperationInput{SpaceID: space.ID, ActorAccountID: uuid.New(), ActorProfileID: uuid.New(), OperationID: uuid.New(), Method: JoinOperationStart, RequestSHA256: joinDigest("test", "start")}
	_, exists, err := st.LookupJoinStartSelection(ctx, in)
	require.NoError(t, err)
	require.False(t, exists)
	saved, err := st.ReserveJoinStartSelection(ctx, in, nil)
	require.NoError(t, err)
	require.EqualValues(t, 1, saved.Context.PolicyVersion)
	require.Nil(t, saved.Context.SelectedAttemptID)
	require.Nil(t, saved.Operation.AttemptID)
	_, err = st.SetEntryPolicy(ctx, SetEntryPolicyInput{SpaceID: space.ID, ActorProfileID: owner, OperationID: uuid.New(), ExpectedVersion: 1, Policy: entrypolicy.Policy{SchemaVersion: 1, Questions: []entrypolicy.Question{}, RequireManualApproval: true}})
	require.NoError(t, err)
	again, err := (&SpaceStore{Pool: pool}).ReserveJoinStartSelection(ctx, in, nil)
	require.NoError(t, err)
	require.Equal(t, saved, again, "restart or policy drift cannot refresh a reserved context")
	changed := in
	changed.RequestSHA256 = joinDigest("test", "changed")
	_, err = st.ReserveJoinStartSelection(ctx, changed, nil)
	require.ErrorIs(t, err, ErrJoinAttemptConflict)
	_, err = pool.Exec(ctx, `UPDATE space_join_operations SET start_selection_context=start_selection_context||'{"policy_version":2}'::jsonb WHERE actor_profile_id=$1 AND operation_id=$2`, in.ActorProfileID, in.OperationID)
	require.Error(t, err, "DB must reject replacement")
	legacy := in
	legacy.OperationID = uuid.New()
	_, err = st.ReserveJoinOperation(ctx, legacy, nil)
	require.NoError(t, err)
	_, exists, err = st.LookupJoinStartSelection(ctx, legacy)
	require.True(t, exists)
	require.ErrorIs(t, err, ErrJoinAttemptUnavailable, "old context is not guessed from current policy")
	var attempts int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM space_join_attempts`).Scan(&attempts))
	require.Zero(t, attempts, "reservation never allocates A or Guest")
	down, err := os.ReadFile(filepath.Join(repoRoot(t), "src/backend/migrations/space_db/000043_join_public_operation_context.down.sql"))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(down))
	require.Error(t, err, "even inert reserved context is retained evidence")
}

func TestJoinStartSelectionDownSeesWaitingFirstWriter(t *testing.T) {
	ctx := context.Background()
	pool := startJoinAttemptPostgres(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}
	space, err := st.CreateSpace(ctx, uuid.New(), "DOWN race", "", "public")
	require.NoError(t, err)
	applyEntryPolicyMigration(t, st)
	applyJoinAttemptMigration(t, st)
	applyJoinOperationMigration(t, st)
	applyStartContextMigration(t, st, "000040_join_decision_basis_v2.up.sql")
	applyStartContextMigration(t, st, "000043_join_public_operation_context.up.sql")
	down, err := os.ReadFile(filepath.Join(repoRoot(t), "src/backend/migrations/space_db/000043_join_public_operation_context.down.sql"))
	require.NoError(t, err)
	oldSnapshot, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	require.NoError(t, err)
	_, err = oldSnapshot.Exec(ctx, `SELECT count(*) FROM space_join_operations`)
	require.NoError(t, err)
	_, err = oldSnapshot.Exec(ctx, string(down))
	require.Error(t, err, "RR rejected even when its inherited snapshot was empty")
	require.NoError(t, oldSnapshot.Rollback(ctx))
	in := JoinOperationInput{SpaceID: space.ID, ActorAccountID: uuid.New(), ActorProfileID: uuid.New(), OperationID: uuid.New(), Method: JoinOperationStart, RequestSHA256: joinDigest("test", "waiting writer")}
	selection := JoinStartSelectionContext{Operation: in, Kind: 1, PolicyVersion: 1, PolicySHA256: joinDigest("test", "policy")}
	sha, err := selection.SHA256()
	require.NoError(t, err)
	raw, err := json.Marshal(selection)
	require.NoError(t, err)
	writer, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = writer.Rollback(ctx) }()
	_, err = writer.Exec(ctx, `INSERT INTO space_join_operations(actor_profile_id,operation_id,actor_account_id,space_id,request_method,request_sha256,start_selection_context,start_selection_sha256) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, in.ActorProfileID, in.OperationID, in.ActorAccountID, in.SpaceID, in.Method, in.RequestSHA256, raw, sha[:])
	require.NoError(t, err)
	done := make(chan error, 1)
	go func() {
		conn, err := pool.Acquire(ctx)
		if err != nil {
			done <- err
			return
		}
		defer conn.Release()
		_, err = conn.Exec(ctx, string(down))
		_, _ = conn.Exec(ctx, `ROLLBACK`)
		done <- err
	}()
	require.Eventually(t, func() bool {
		var blocked bool
		err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_locks WHERE relation='space_join_operations'::regclass AND mode='AccessExclusiveLock' AND NOT granted)`).Scan(&blocked)
		return err == nil && blocked
	}, 3*time.Second, 10*time.Millisecond)
	require.NoError(t, writer.Commit(ctx))
	require.Error(t, <-done, "DOWN must see committed first writer after acquiring its lock")
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM space_join_operations WHERE start_selection_context IS NOT NULL`).Scan(&count))
	require.Equal(t, 1, count)
}
