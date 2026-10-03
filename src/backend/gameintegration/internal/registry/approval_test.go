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

func TestProductionAdmissionStagesOnlyPendingEnvironment(t *testing.T) {
	ctx := context.Background()
	pool := startT12Postgres(t, ctx)
	store := &Store{Pool: pool}
	owner, operator := uuid.New(), uuid.New()
	app, err := store.CreateApplication(ctx, CreateApplicationInput{OwnerAccountID: owner, Name: "Game", IdempotencyKey: "create-prod"})
	require.NoError(t, err)
	_, err = store.ApproveSandbox(ctx, ApproveSandboxInput{ApplicationID: app.ID, OperatorAccountID: uuid.New(), IdempotencyKey: "sandbox-prod"})
	require.NoError(t, err)
	_, err = store.PrepareProductionAdmission(ctx, PrepareProductionAdmissionInput{ApplicationID: app.ID, OperatorAccountID: owner, IdempotencyKey: "production-1"})
	require.ErrorIs(t, err, ErrSelfApproval)

	env, err := store.PrepareProductionAdmission(ctx, PrepareProductionAdmissionInput{ApplicationID: app.ID, OperatorAccountID: operator, IdempotencyKey: "production-1"})
	require.NoError(t, err)
	require.Equal(t, "production", env.Kind)
	require.Equal(t, "pending", env.Status)
	require.Equal(t, app.ID, env.ApplicationID)
	_, err = store.IssueCredential(ctx, IssueCredentialInput{OwnerAccountID: owner, ApplicationID: app.ID,
		EnvironmentID: env.ID, Scopes: []string{"game.events.write"}, IdempotencyKey: "pending-production-credential",
		SecretKey: []byte("0123456789abcdef0123456789abcdef")})
	require.ErrorIs(t, err, ErrAdmissionConflict)
	again, err := store.PrepareProductionAdmission(ctx, PrepareProductionAdmissionInput{ApplicationID: app.ID, OperatorAccountID: operator, IdempotencyKey: "production-1"})
	require.NoError(t, err)
	require.Equal(t, env.ID, again.ID)
	var appStatus string
	require.NoError(t, pool.QueryRow(ctx, `SELECT status FROM applications WHERE id=$1`, app.ID).Scan(&appStatus))
	require.Equal(t, "sandbox", appStatus)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM environments WHERE application_id=$1 AND kind='production'`, app.ID).Scan(&count))
	require.Equal(t, 1, count)
	_, err = store.SetApplicationSuspension(ctx, SetApplicationSuspensionInput{ApplicationID: app.ID, OperatorAccountID: operator, Suspended: true, IdempotencyKey: "suspend-prod-pending"})
	require.NoError(t, err)
	_, err = store.SetApplicationSuspension(ctx, SetApplicationSuspensionInput{ApplicationID: app.ID, OperatorAccountID: operator, Suspended: false, IdempotencyKey: "restore-prod-pending"})
	require.NoError(t, err)
	var envStatus string
	require.NoError(t, pool.QueryRow(ctx, `SELECT status FROM environments WHERE id=$1`, env.ID).Scan(&envStatus))
	require.Equal(t, "pending", envStatus)
	_, err = store.PrepareProductionAdmission(ctx, PrepareProductionAdmissionInput{ApplicationID: app.ID, OperatorAccountID: operator, IdempotencyKey: "production-2"})
	require.ErrorIs(t, err, ErrAdmissionConflict)
}
