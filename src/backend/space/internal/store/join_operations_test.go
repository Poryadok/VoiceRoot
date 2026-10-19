package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func applyJoinOperationMigration(t *testing.T, st *SpaceStore) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), "src/backend/migrations/space_db/000036_join_operation_namespace.up.sql"))
	require.NoError(t, err)
	_, err = st.Pool.Exec(context.Background(), string(raw))
	require.NoError(t, err)
}

func TestJoinAttempt_PublicOperationNamespaceSurvivesRestart(t *testing.T) {
	ctx := context.Background()
	pool := startJoinAttemptPostgres(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}
	space, err := st.CreateSpace(ctx, uuid.New(), "Public operation", "", "public")
	require.NoError(t, err)
	applyEntryPolicyMigration(t, st)
	applyJoinAttemptMigration(t, st)
	applyJoinOperationMigration(t, st)
	in := JoinOperationInput{SpaceID: space.ID, ActorAccountID: uuid.New(), ActorProfileID: uuid.New(), OperationID: uuid.New(), Method: JoinOperationStart, RequestSHA256: joinDigest("test request", "start")}
	reserved, err := st.ReserveJoinOperation(ctx, in, nil)
	require.NoError(t, err)
	require.Equal(t, "RESERVED", reserved.State)
	require.Nil(t, reserved.AttemptID)
	changed := in
	changed.Method = JoinOperationSubmit
	_, err = st.ReserveJoinOperation(ctx, changed, nil)
	require.ErrorIs(t, err, ErrJoinAttemptConflict, "method changes cannot escape actor/op namespace")
	changed = in
	changed.RequestSHA256 = joinDigest("test request", "changed")
	_, err = st.ReserveJoinOperation(ctx, changed, nil)
	require.ErrorIs(t, err, ErrJoinAttemptConflict)
	restarted, err := pgxpool.NewWithConfig(ctx, pool.Config())
	require.NoError(t, err)
	defer restarted.Close()
	again, err := (&SpaceStore{Pool: restarted}).ReserveJoinOperation(ctx, in, nil)
	require.NoError(t, err)
	require.Equal(t, reserved, again)
	var attempts, members int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM space_join_attempts WHERE space_id=$1`, space.ID).Scan(&attempts))
	require.Zero(t, attempts)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM space_members WHERE space_id=$1 AND profile_id=$2`, space.ID, in.ActorProfileID).Scan(&members))
	require.Zero(t, members)
}

func TestJoinAttempt_PublicOperationSelectionSharesAttemptTransaction(t *testing.T) {
	ctx := context.Background()
	pool := startJoinAttemptPostgres(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}
	space, err := st.CreateSpace(ctx, uuid.New(), "Public selection", "", "public")
	require.NoError(t, err)
	applyEntryPolicyMigration(t, st)
	applyJoinAttemptMigration(t, st)
	applyJoinOperationMigration(t, st)
	in := JoinOperationInput{SpaceID: space.ID, ActorAccountID: uuid.New(), ActorProfileID: uuid.New(), OperationID: uuid.New(), Method: JoinOperationStart, RequestSHA256: joinDigest("test request", "select")}
	_, err = st.ReserveJoinOperation(ctx, in, nil)
	require.NoError(t, err)
	applicant := JoinAttemptInput{SpaceID: space.ID, AccountID: in.ActorAccountID, ProfileID: in.ActorProfileID}
	a, err := st.StartJoinAttempt(ctx, applicant)
	require.NoError(t, err)
	require.ErrorIs(t, st.SelectJoinOperationAttempt(ctx, in, a.ID), ErrJoinAttemptUnavailable)
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()
	require.NoError(t, lockAndCheckOwnershipScope(ctx, tx, []uuid.UUID{space.ID}))
	scoped := *st
	scoped.tx = tx
	require.NoError(t, scoped.SelectJoinOperationAttempt(ctx, in, a.ID))
	require.NoError(t, tx.Rollback(ctx))
	reserved, err := st.ReserveJoinOperation(ctx, in, nil)
	require.NoError(t, err)
	require.Nil(t, reserved.AttemptID, "selection must roll back with its outer transaction")
	require.NoError(t, st.withOwnershipScope(ctx, []uuid.UUID{space.ID}, func(scoped *SpaceStore) error {
		return scoped.SelectJoinOperationAttempt(ctx, in, a.ID)
	}))
	reserved, err = st.ReserveJoinOperation(ctx, in, nil)
	require.NoError(t, err)
	require.Equal(t, a.ID, *reserved.AttemptID)
	otherInput := applicant
	otherInput.ProfileID = uuid.New()
	other, err := st.StartJoinAttempt(ctx, otherInput)
	require.NoError(t, err)
	err = st.withOwnershipScope(ctx, []uuid.UUID{space.ID}, func(scoped *SpaceStore) error {
		return scoped.SelectJoinOperationAttempt(ctx, in, other.ID)
	})
	require.ErrorIs(t, err, ErrJoinAttemptConflict, "selected A never moves to another attempt")
}
