package registry

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/pkg/integrationtest"
)

func TestSandboxApprovalRequiresDistinctOperatorAndIsIdempotent(t *testing.T) {
	ctx := context.Background()
	migration := filepath.Join("..", "..", "..", "migrations", "game_integration_db", "000001_init.up.sql")
	pool := integrationtest.StartPostgres(t, ctx, "game_integration_test", migration)
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
