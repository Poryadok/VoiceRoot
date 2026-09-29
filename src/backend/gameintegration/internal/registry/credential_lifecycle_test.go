package registry

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/pkg/integrationtest"
)

func TestT04CredentialMigrationRetiresLegacyRowsIrreversibly(t *testing.T) {
	ctx := context.Background()
	migrations := filepath.Join("..", "..", "..", "migrations", "game_integration_db")
	pool := integrationtest.StartPostgres(t, ctx, "game_integration_test", filepath.Join(migrations, "000001_init.up.sql"))
	t12, err := os.ReadFile(filepath.Join(migrations, "000002_t12_registry_security.up.sql"))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(t12))
	require.NoError(t, err)
	store := &Store{Pool: pool}
	owner, operator := uuid.New(), uuid.New()
	app, err := store.CreateApplication(ctx, CreateApplicationInput{OwnerAccountID: owner, Name: "Legacy", IdempotencyKey: "legacy-app"})
	require.NoError(t, err)
	env, err := store.ApproveSandbox(ctx, ApproveSandboxInput{ApplicationID: app.ID, OperatorAccountID: operator, IdempotencyKey: "legacy-env"})
	require.NoError(t, err)
	key := []byte("0123456789abcdef0123456789abcdef")
	issued, err := store.IssueCredential(ctx, IssueCredentialInput{OwnerAccountID: owner, ApplicationID: app.ID,
		EnvironmentID: env.ID, Scopes: []string{"game.events.write"}, IdempotencyKey: "legacy-credential", SecretKey: key})
	require.NoError(t, err)

	t04Up, err := os.ReadFile(filepath.Join(migrations, "000013_t04_credential_lifecycle_fence.up.sql"))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(t04Up))
	require.NoError(t, err)
	var revoked bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT revoked_at IS NOT NULL AND secret_digest=decode(repeat('00',32),'hex')
		FROM service_credentials WHERE id=$1`, issued.ID).Scan(&revoked))
	require.True(t, revoked, "upgrading the seal format must retire every preexisting credential")

	t04Down, err := os.ReadFile(filepath.Join(migrations, "000013_t04_credential_lifecycle_fence.down.sql"))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(t04Down))
	require.NoError(t, err)
	require.NoError(t, pool.QueryRow(ctx, `SELECT revoked_at IS NOT NULL AND secret_digest=decode(repeat('00',32),'hex')
		FROM service_credentials WHERE id=$1`, issued.ID).Scan(&revoked))
	require.True(t, revoked, "migration rollback removes triggers but never restores legacy credentials")
}

func TestApplicationStatusSQLRestoreCannotReviveCredential(t *testing.T) {
	ctx := context.Background()
	pool := startT12Postgres(t, ctx)
	store := &Store{Pool: pool}
	owner, operator := uuid.New(), uuid.New()
	app, err := store.CreateApplication(ctx, CreateApplicationInput{OwnerAccountID: owner, Name: "Lifecycle", IdempotencyKey: "lifecycle-app"})
	require.NoError(t, err)
	env, err := store.ApproveSandbox(ctx, ApproveSandboxInput{ApplicationID: app.ID, OperatorAccountID: operator, IdempotencyKey: "lifecycle-env"})
	require.NoError(t, err)
	key := []byte("0123456789abcdef0123456789abcdef")
	issued, err := store.IssueCredential(ctx, IssueCredentialInput{OwnerAccountID: owner, ApplicationID: app.ID,
		EnvironmentID: env.ID, Scopes: []string{"game.events.write"}, IdempotencyKey: "lifecycle-credential", SecretKey: key})
	require.NoError(t, err)
	bearer := "vgi1_" + issued.ID.String() + "_" + issued.Secret

	// Even a direct SQL suspend followed by restoration fires the database fence.
	_, err = pool.Exec(ctx, `UPDATE applications SET status='suspended' WHERE id=$1`, app.ID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE applications SET status='sandbox' WHERE id=$1`, app.ID)
	require.NoError(t, err)
	_, err = store.VerifyCredential(ctx, bearer, "game.events.write", key)
	require.ErrorIs(t, err, ErrInvalidServiceCredential, "restoring app status must not restore authority")
	var revoked bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT revoked_at IS NOT NULL AND secret_digest=decode(repeat('00',32),'hex')
		FROM service_credentials WHERE id=$1`, issued.ID).Scan(&revoked))
	require.True(t, revoked, "application status transition must destroy prior credential authority")
}

func TestEnvironmentStatusSQLRestoreCannotReviveCredential(t *testing.T) {
	ctx := context.Background()
	pool := startT12Postgres(t, ctx)
	store := &Store{Pool: pool}
	owner, operator := uuid.New(), uuid.New()
	app, err := store.CreateApplication(ctx, CreateApplicationInput{OwnerAccountID: owner, Name: "Environment lifecycle", IdempotencyKey: "env-lifecycle-app"})
	require.NoError(t, err)
	env, err := store.ApproveSandbox(ctx, ApproveSandboxInput{ApplicationID: app.ID, OperatorAccountID: operator, IdempotencyKey: "env-lifecycle-env"})
	require.NoError(t, err)
	key := []byte("0123456789abcdef0123456789abcdef")
	issued, err := store.IssueCredential(ctx, IssueCredentialInput{OwnerAccountID: owner, ApplicationID: app.ID,
		EnvironmentID: env.ID, Scopes: []string{"game.events.write"}, IdempotencyKey: "env-lifecycle-credential", SecretKey: key})
	require.NoError(t, err)
	bearer := "vgi1_" + issued.ID.String() + "_" + issued.Secret

	_, err = pool.Exec(ctx, `UPDATE environments SET status='suspended' WHERE id=$1`, env.ID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE environments SET status='active' WHERE id=$1`, env.ID)
	require.NoError(t, err)
	_, err = store.VerifyCredential(ctx, bearer, "game.events.write", key)
	require.ErrorIs(t, err, ErrInvalidServiceCredential, "restoring environment status must not restore authority")
}
