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
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/entrypolicy"
)

func TestJoinLegacySelectionPinsPolicyAndOriginBeforeOwner(t *testing.T) {
	ctx := context.Background()
	pool := startJoinAttemptPostgres(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}
	owner := uuid.New()
	space, err := st.CreateSpace(ctx, owner, "Legacy reservation", "", "public")
	require.NoError(t, err)
	applyEntryPolicyMigration(t, st)
	applyJoinAttemptMigration(t, st)
	applyJoinOperationMigration(t, st)
	applyJoinV2Migration(t, st)
	applyStartContextMigration(t, st, "000043_join_public_operation_context.up.sql")
	applyStartContextMigration(t, st, "000045_join_legacy_selection_context.up.sql")
	in := JoinOperationInput{SpaceID: space.ID, ActorAccountID: uuid.New(), ActorProfileID: uuid.New(), OperationID: uuid.New(), Method: spacev1.SpaceService_JoinSpace_FullMethodName, RequestSHA256: joinDigest("test", "actual legacy request")}
	saved, err := st.ReserveLegacyJoinSelection(ctx, in, JoinOperationServerNoKey, nil)
	require.NoError(t, err)
	require.Nil(t, saved.Operation.AttemptID, "reservation cannot guess A or canonical Guest")
	require.EqualValues(t, 1, saved.Context.PolicyVersion)
	_, err = st.SetEntryPolicy(ctx, SetEntryPolicyInput{SpaceID: space.ID, ActorProfileID: owner, OperationID: uuid.New(), ExpectedVersion: 1, Policy: entrypolicy.Policy{SchemaVersion: 1, Questions: []entrypolicy.Question{}, RequireManualApproval: true}})
	require.NoError(t, err)
	again, err := (&SpaceStore{Pool: pool}).ReserveLegacyJoinSelection(ctx, in, JoinOperationServerNoKey, nil)
	require.NoError(t, err)
	require.Equal(t, saved, again, "restart never refreshes policy or origin")
	_, err = st.ReserveLegacyJoinSelection(ctx, in, JoinOperationClientKey, nil)
	require.ErrorIs(t, err, ErrJoinAttemptConflict)
	changed := in
	changed.RequestSHA256 = joinDigest("test", "changed request")
	_, err = st.ReserveLegacyJoinSelection(ctx, changed, JoinOperationServerNoKey, nil)
	require.ErrorIs(t, err, ErrJoinAttemptConflict)
	_, err = pool.Exec(ctx, `UPDATE space_join_operations SET legacy_selection_context=legacy_selection_context||'{"policy_version":2}'::jsonb WHERE actor_profile_id=$1 AND operation_id=$2`, in.ActorProfileID, in.OperationID)
	require.Error(t, err, "DB forbids replacing the pre-owner selection")
	var attempts int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM space_join_attempts`).Scan(&attempts))
	require.Zero(t, attempts)
	// An older reservation lacks provenance; it cannot be backfilled from now.
	old := in
	old.OperationID = uuid.New()
	_, err = pool.Exec(ctx, `INSERT INTO space_join_operations(actor_profile_id,operation_id,actor_account_id,space_id,request_method,request_sha256,basis_version,operation_origin) VALUES($1,$2,$3,$4,$5,$6,2,2)`, old.ActorProfileID, old.OperationID, old.ActorAccountID, old.SpaceID, old.Method, old.RequestSHA256)
	require.NoError(t, err)
	_, err = st.ReserveLegacyJoinSelection(ctx, old, JoinOperationServerNoKey, nil)
	require.ErrorIs(t, err, ErrJoinAttemptUnavailable)
	down, err := os.ReadFile(filepath.Join(repoRoot(t), "src/backend/migrations/space_db/000045_join_legacy_selection_context.down.sql"))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(down))
	require.Error(t, err, "even RESERVED selection is retained provenance")
	_, err = pool.Exec(ctx, "ROLLBACK")
	require.NoError(t, err)
	// An inherited snapshot is never acceptable to guarded DOWN.
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	require.NoError(t, err)
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, string(down))
	require.Error(t, err)
}

func TestJoinLegacySelectionDownWaitsForFirstWriter(t *testing.T) {
	ctx := context.Background()
	pool := startJoinAttemptPostgres(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}
	space, err := st.CreateSpace(ctx, uuid.New(), "Legacy DOWN race", "", "public")
	require.NoError(t, err)
	applyEntryPolicyMigration(t, st)
	applyJoinAttemptMigration(t, st)
	applyJoinOperationMigration(t, st)
	applyJoinV2Migration(t, st)
	applyStartContextMigration(t, st, "000043_join_public_operation_context.up.sql")
	applyStartContextMigration(t, st, "000045_join_legacy_selection_context.up.sql")
	down, err := os.ReadFile(filepath.Join(repoRoot(t), "src/backend/migrations/space_db/000045_join_legacy_selection_context.down.sql"))
	require.NoError(t, err)
	// Empty RC control succeeds; reinstall so the writer/downgrade race is real.
	_, err = pool.Exec(ctx, string(down))
	require.NoError(t, err)
	applyStartContextMigration(t, st, "000045_join_legacy_selection_context.up.sql")
	in := JoinOperationInput{SpaceID: space.ID, ActorAccountID: uuid.New(), ActorProfileID: uuid.New(), OperationID: uuid.New(), Method: spacev1.SpaceService_JoinSpace_FullMethodName, RequestSHA256: joinDigest("test", "waiting legacy writer")}
	c := JoinLegacySelectionContext{Operation: in, Origin: JoinOperationServerNoKey, PolicyVersion: 1, PolicySHA256: joinDigest("test", "policy")}
	sha, err := c.SHA256()
	require.NoError(t, err)
	raw, err := json.Marshal(c)
	require.NoError(t, err)
	writer, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer writer.Rollback(ctx)
	_, err = writer.Exec(ctx, `INSERT INTO space_join_operations(actor_profile_id,operation_id,actor_account_id,space_id,request_method,request_sha256,basis_version,operation_origin,legacy_selection_context,legacy_selection_sha256) VALUES($1,$2,$3,$4,$5,$6,2,2,$7,$8)`, in.ActorProfileID, in.OperationID, in.ActorAccountID, in.SpaceID, in.Method, in.RequestSHA256, raw, sha[:])
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
		_, _ = conn.Exec(ctx, "ROLLBACK")
		done <- err
	}()
	require.Eventually(t, func() bool {
		var blocked bool
		err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_locks WHERE relation='space_join_operations'::regclass AND mode='AccessExclusiveLock' AND NOT granted)`).Scan(&blocked)
		return err == nil && blocked
	}, 3*time.Second, 10*time.Millisecond)
	require.NoError(t, writer.Commit(ctx))
	require.Error(t, <-done, "DOWN sees newly committed first legacy reservation after waiting")
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM space_join_operations WHERE legacy_selection_context IS NOT NULL`).Scan(&count))
	require.Equal(t, 1, count)
}
