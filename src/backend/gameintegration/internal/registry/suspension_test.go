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

func TestApplicationSuspensionBlocksCredentialsAndPolicyAndRestoreKeepsRevocations(t *testing.T) {
	ctx := context.Background()
	migrations := filepath.Join("..", "..", "..", "migrations", "game_integration_db")
	pool := integrationtest.StartPostgres(t, ctx, "game_integration_test", filepath.Join(migrations, "000001_init.up.sql"))
	migration, err := os.ReadFile(filepath.Join(migrations, "000002_t12_registry_security.up.sql"))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(migration))
	require.NoError(t, err)
	store := &Store{Pool: pool}
	ownerID, operatorID := uuid.New(), uuid.New()
	app, err := store.CreateApplication(ctx, CreateApplicationInput{
		OwnerAccountID: ownerID, Name: "Suspension", IdempotencyKey: "suspend-app",
	})
	require.NoError(t, err)
	env, err := store.ApproveSandbox(ctx, ApproveSandboxInput{
		ApplicationID: app.ID, OperatorAccountID: operatorID, IdempotencyKey: "suspend-env",
	})
	require.NoError(t, err)
	_, err = store.UpdateSandboxPolicy(ctx, UpdateSandboxPolicyInput{
		OwnerAccountID: ownerID, ApplicationID: app.ID, EnvironmentID: env.ID,
		ExpectedRevision: env.Revision, RedirectURIs: []string{"https://game.example/callback"},
		AllowedOrigins: []string{"https://game.example"}, Providers: []string{"google"},
		PlayerScopes: []string{"game.chat.read"}, IdempotencyKey: "suspend-policy",
	})
	require.NoError(t, err)
	key := []byte("0123456789abcdef0123456789abcdef")
	active, err := store.IssueCredential(ctx, IssueCredentialInput{
		OwnerAccountID: ownerID, ApplicationID: app.ID, EnvironmentID: env.ID,
		Scopes: []string{"game.events.write"}, IdempotencyKey: "suspend-active", SecretKey: key,
	})
	require.NoError(t, err)
	revoked, err := store.IssueCredential(ctx, IssueCredentialInput{
		OwnerAccountID: ownerID, ApplicationID: app.ID, EnvironmentID: env.ID,
		Scopes: []string{"game.roster.write"}, IdempotencyKey: "suspend-revoked", SecretKey: key,
	})
	require.NoError(t, err)
	require.NoError(t, store.RevokeCredential(ctx, ownerID, app.ID, env.ID, revoked.ID))

	suspended, err := store.SetApplicationSuspension(ctx, SetApplicationSuspensionInput{
		ApplicationID: app.ID, OperatorAccountID: operatorID, Suspended: true, IdempotencyKey: "suspend-on",
	})
	require.NoError(t, err)
	require.Equal(t, "suspended", suspended.Status)
	retry, err := store.SetApplicationSuspension(ctx, SetApplicationSuspensionInput{
		ApplicationID: app.ID, OperatorAccountID: operatorID, Suspended: true, IdempotencyKey: "suspend-on",
	})
	require.NoError(t, err)
	require.Equal(t, suspended.Revision, retry.Revision, "an identical idempotent retry returns the saved transition")
	_, err = store.SetApplicationSuspension(ctx, SetApplicationSuspensionInput{
		ApplicationID: app.ID, OperatorAccountID: operatorID, Suspended: true, IdempotencyKey: "suspend-on-again",
	})
	require.ErrorIs(t, err, ErrApplicationStateConflict)
	_, err = store.VerifyCredential(ctx, "vgi1_"+active.ID.String()+"_"+active.Secret, "game.events.write", key)
	require.ErrorIs(t, err, ErrApplicationSuspended)
	_, err = store.IssueCredential(ctx, IssueCredentialInput{
		OwnerAccountID: ownerID, ApplicationID: app.ID, EnvironmentID: env.ID,
		Scopes: []string{"game.sessions.write"}, IdempotencyKey: "suspended-owner-credential", SecretKey: key,
	})
	require.ErrorIs(t, err, ErrApplicationSuspended,
		"the authorized owner must receive the suspension-specific fail-closed result")
	_, err = store.IssueCredential(ctx, IssueCredentialInput{
		OwnerAccountID: uuid.New(), ApplicationID: app.ID, EnvironmentID: env.ID,
		Scopes: []string{"game.events.write"}, IdempotencyKey: "suspended-foreign-credential", SecretKey: key,
	})
	require.ErrorIs(t, err, ErrAdmissionConflict,
		"foreign owners receive the generic scope denial even when the application is suspended")
	_, err = store.LoadAuthorizationPolicy(ctx, env.ID)
	require.ErrorIs(t, err, ErrApplicationSuspended)
	_, err = store.UpdateSandboxPolicy(ctx, UpdateSandboxPolicyInput{
		OwnerAccountID: ownerID, ApplicationID: app.ID, EnvironmentID: env.ID,
		ExpectedRevision: env.Revision + 1, RedirectURIs: []string{"https://game.example/updated"},
		AllowedOrigins: []string{"https://game.example"}, Providers: []string{"google"},
		PlayerScopes: []string{"game.chat.read"}, IdempotencyKey: "suspended-owner-policy",
	})
	require.ErrorIs(t, err, ErrApplicationSuspended,
		"the authorized owner must receive the suspension-specific fail-closed result")
	_, err = store.UpdateSandboxPolicy(ctx, UpdateSandboxPolicyInput{
		OwnerAccountID: uuid.New(), ApplicationID: app.ID, EnvironmentID: env.ID,
		ExpectedRevision: env.Revision, RedirectURIs: []string{"https://game.example/callback"},
		AllowedOrigins: []string{"https://game.example"}, Providers: []string{"google"},
		PlayerScopes: []string{"game.chat.read"}, IdempotencyKey: "suspended-foreign-policy",
	})
	require.ErrorIs(t, err, ErrAdmissionConflict,
		"foreign owners receive the generic scope denial even when the application is suspended")
	var credentialCount int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM service_credentials WHERE environment_id=$1`, env.ID).Scan(&credentialCount))
	require.Equal(t, 2, credentialCount, "suspended owner attempts must not create credentials")
	var policyRevision int64
	require.NoError(t, pool.QueryRow(ctx, `SELECT revision FROM environments WHERE id=$1`, env.ID).Scan(&policyRevision))
	require.Equal(t, env.Revision+1, policyRevision, "suspended owner attempts must not update policy revision")
	var deniedMutationAuditCount int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM registry_audit WHERE application_id=$1
		AND environment_id=$2 AND action IN ('issue_credential','update_sandbox_policy')`, app.ID, env.ID).Scan(&deniedMutationAuditCount))
	require.Equal(t, 3, deniedMutationAuditCount,
		"the credential retry that created the second credential and the original policy update are the only mutation audits")

	_, err = pool.Exec(ctx, `UPDATE environments SET status='suspended' WHERE id=$1`, env.ID)
	require.NoError(t, err)
	restored, err := store.SetApplicationSuspension(ctx, SetApplicationSuspensionInput{
		ApplicationID: app.ID, OperatorAccountID: operatorID, Suspended: false, IdempotencyKey: "suspend-off",
	})
	require.NoError(t, err)
	require.Equal(t, "sandbox", restored.Status)
	replayed, err := store.SetApplicationSuspension(ctx, SetApplicationSuspensionInput{
		ApplicationID: app.ID, OperatorAccountID: operatorID, Suspended: true, IdempotencyKey: "suspend-on",
	})
	require.NoError(t, err)
	require.Equal(t, suspended.Status, replayed.Status, "idempotent replay returns the saved transition snapshot")
	require.Equal(t, suspended.Revision, replayed.Revision, "idempotent replay returns the saved revision")
	require.Equal(t, suspended.UpdatedAt, replayed.UpdatedAt, "idempotent replay returns the saved update timestamp")
	var envStatus string
	require.NoError(t, pool.QueryRow(ctx, `SELECT status FROM environments WHERE id=$1`, env.ID).Scan(&envStatus))
	require.Equal(t, "suspended", envStatus, "restoring the app must not restore an independently suspended environment")
	_, err = store.VerifyCredential(ctx, "vgi1_"+revoked.ID.String()+"_"+revoked.Secret, "game.roster.write", key)
	require.ErrorIs(t, err, ErrInvalidServiceCredential, "restore must not reactivate a revoked credential")
	var auditCount int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM registry_audit WHERE application_id=$1
		AND actor_kind='operator' AND actor_id=$2 AND action IN ('suspend_application','restore_application')
		AND source='operator_allowlist' AND result='success'`, app.ID, operatorID).Scan(&auditCount))
	require.Equal(t, 2, auditCount, "both committed transitions retain their actor and provenance")
	var auditJSON string
	require.NoError(t, pool.QueryRow(ctx, `SELECT to_jsonb(registry_audit)::text FROM registry_audit
		WHERE application_id=$1 AND action='suspend_application'`, app.ID).Scan(&auditJSON))
	require.NotContains(t, auditJSON, active.Secret)
	require.NotContains(t, auditJSON, revoked.Secret)
	require.NoError(t, pool.QueryRow(ctx, `SELECT to_jsonb(registry_audit)::text FROM registry_audit
		WHERE application_id=$1 AND action='restore_application'`, app.ID).Scan(&auditJSON))
	require.NotContains(t, auditJSON, active.Secret)
	require.NotContains(t, auditJSON, revoked.Secret)
}

func TestApplicationSuspensionRestoresPreviouslyActiveState(t *testing.T) {
	ctx := context.Background()
	migrations := filepath.Join("..", "..", "..", "migrations", "game_integration_db")
	pool := integrationtest.StartPostgres(t, ctx, "game_integration_test", filepath.Join(migrations, "000001_init.up.sql"))
	migration, err := os.ReadFile(filepath.Join(migrations, "000002_t12_registry_security.up.sql"))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(migration))
	require.NoError(t, err)
	store := &Store{Pool: pool}
	app, err := store.CreateApplication(ctx, CreateApplicationInput{
		OwnerAccountID: uuid.New(), Name: "Active restore", IdempotencyKey: "active-restore-app",
	})
	require.NoError(t, err)
	_, err = store.ApproveSandbox(ctx, ApproveSandboxInput{
		ApplicationID: app.ID, OperatorAccountID: uuid.New(), IdempotencyKey: "active-restore-env",
	})
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE applications SET status='active' WHERE id=$1`, app.ID)
	require.NoError(t, err)
	operatorID := uuid.New()
	_, err = store.SetApplicationSuspension(ctx, SetApplicationSuspensionInput{
		ApplicationID: app.ID, OperatorAccountID: operatorID, Suspended: true, IdempotencyKey: "active-suspend",
	})
	require.NoError(t, err)
	restored, err := store.SetApplicationSuspension(ctx, SetApplicationSuspensionInput{
		ApplicationID: app.ID, OperatorAccountID: operatorID, Suspended: false, IdempotencyKey: "active-restore",
	})
	require.NoError(t, err)
	require.Equal(t, "active", restored.Status, "restore must recover active rather than hard-code sandbox")
}

func TestApplicationSuspensionRejectsBlockedRestoreAndAuditsDeniedTransition(t *testing.T) {
	ctx := context.Background()
	migrations := filepath.Join("..", "..", "..", "migrations", "game_integration_db")
	pool := integrationtest.StartPostgres(t, ctx, "game_integration_test", filepath.Join(migrations, "000001_init.up.sql"))
	migration, err := os.ReadFile(filepath.Join(migrations, "000002_t12_registry_security.up.sql"))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(migration))
	require.NoError(t, err)
	store := &Store{Pool: pool}
	ownerID, operatorID := uuid.New(), uuid.New()
	app, err := store.CreateApplication(ctx, CreateApplicationInput{
		OwnerAccountID: ownerID, Name: "Suspension draft", IdempotencyKey: "suspend-draft-app",
	})
	require.NoError(t, err)
	_, err = store.SetApplicationSuspension(ctx, SetApplicationSuspensionInput{
		ApplicationID: app.ID, OperatorAccountID: operatorID, Suspended: false, IdempotencyKey: "restore-draft",
	})
	require.ErrorIs(t, err, ErrApplicationStateConflict)
	var status string
	require.NoError(t, pool.QueryRow(ctx, `SELECT status FROM applications WHERE id=$1`, app.ID).Scan(&status))
	require.Equal(t, "draft", status)
	var denied int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM registry_audit WHERE application_id=$1
		AND actor_kind='operator' AND actor_id=$2 AND action='restore_application'
		AND result='denied' AND source='operator_allowlist'`, app.ID, operatorID).Scan(&denied))
	require.Equal(t, 1, denied)
}

func TestApplicationSuspensionAuditFailureRollsBackBothTransitions(t *testing.T) {
	ctx := context.Background()
	migrations := filepath.Join("..", "..", "..", "migrations", "game_integration_db")
	pool := integrationtest.StartPostgres(t, ctx, "game_integration_test", filepath.Join(migrations, "000001_init.up.sql"))
	migration, err := os.ReadFile(filepath.Join(migrations, "000002_t12_registry_security.up.sql"))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(migration))
	require.NoError(t, err)
	store := &Store{Pool: pool}
	app, err := store.CreateApplication(ctx, CreateApplicationInput{
		OwnerAccountID: uuid.New(), Name: "Audit rollback", IdempotencyKey: "audit-rollback-app",
	})
	require.NoError(t, err)
	_, err = store.ApproveSandbox(ctx, ApproveSandboxInput{
		ApplicationID: app.ID, OperatorAccountID: uuid.New(), IdempotencyKey: "audit-rollback-env",
	})
	require.NoError(t, err)
	operatorID := uuid.New()
	_, err = pool.Exec(ctx, `CREATE FUNCTION reject_suspension_audit() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN IF NEW.action IN ('suspend_application','restore_application') THEN
			RAISE EXCEPTION 'forced suspension audit failure'; END IF; RETURN NEW; END $$;
		CREATE TRIGGER reject_suspension_audit BEFORE INSERT ON registry_audit
		FOR EACH ROW EXECUTE FUNCTION reject_suspension_audit()`)
	require.NoError(t, err)
	_, err = store.SetApplicationSuspension(ctx, SetApplicationSuspensionInput{
		ApplicationID: app.ID, OperatorAccountID: operatorID, Suspended: true, IdempotencyKey: "audit-fail-suspend",
	})
	require.Error(t, err)
	var status string
	require.NoError(t, pool.QueryRow(ctx, `SELECT status FROM applications WHERE id=$1`, app.ID).Scan(&status))
	require.Equal(t, "sandbox", status, "failed suspend audit must roll back the status transition")
	_, err = pool.Exec(ctx, `DROP TRIGGER reject_suspension_audit ON registry_audit`)
	require.NoError(t, err)
	_, err = store.SetApplicationSuspension(ctx, SetApplicationSuspensionInput{
		ApplicationID: app.ID, OperatorAccountID: operatorID, Suspended: true, IdempotencyKey: "audit-rollback-suspend",
	})
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `CREATE TRIGGER reject_suspension_audit BEFORE INSERT ON registry_audit
		FOR EACH ROW EXECUTE FUNCTION reject_suspension_audit()`)
	require.NoError(t, err)
	_, err = store.SetApplicationSuspension(ctx, SetApplicationSuspensionInput{
		ApplicationID: app.ID, OperatorAccountID: operatorID, Suspended: false, IdempotencyKey: "audit-fail-restore",
	})
	require.Error(t, err)
	require.NoError(t, pool.QueryRow(ctx, `SELECT status FROM applications WHERE id=$1`, app.ID).Scan(&status))
	require.Equal(t, "suspended", status, "failed restore audit must leave the app suspended")
}
