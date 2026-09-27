package registry

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestSandboxApprovalRequiresDistinctOperatorAndIsIdempotent(t *testing.T) {
	ctx := context.Background()
	pool := startT12Postgres(t, ctx)
	store := &Store{Pool: pool}
	owner, operator := uuid.New(), uuid.New()
	app, err := store.CreateApplication(ctx, CreateApplicationInput{OwnerAccountID: owner, Name: "Game", IdempotencyKey: "create"})
	require.NoError(t, err)
	_, err = store.ApproveSandbox(ctx, ApproveSandboxInput{ApplicationID: app.ID, OperatorAccountID: owner, IdempotencyKey: "approve"})
	require.ErrorIs(t, err, ErrSelfApproval)
	env, err := store.ApproveSandbox(ctx, ApproveSandboxInput{ApplicationID: app.ID, OperatorAccountID: operator, IdempotencyKey: "approve"})
	require.NoError(t, err)
	require.Equal(t, app.ID, env.ApplicationID)
	require.Equal(t, "sandbox", env.Kind)
	require.Equal(t, "active", env.Status)
	again, err := store.ApproveSandbox(ctx, ApproveSandboxInput{ApplicationID: app.ID, OperatorAccountID: operator, IdempotencyKey: "approve"})
	require.NoError(t, err)
	require.Equal(t, env.ID, again.ID)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM environments WHERE application_id=$1`, app.ID).Scan(&count))
	require.Equal(t, 1, count)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM registry_audit WHERE application_id=$1 AND action='approve_sandbox'`, app.ID).Scan(&count))
	require.Equal(t, 1, count)
}
