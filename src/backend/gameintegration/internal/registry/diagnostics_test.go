package registry

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/netip"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/pkg/integrationtest"
)

func TestT12MigrationPreservesLegacyOperatorApprovalProvenance(t *testing.T) {
	ctx := context.Background()
	migrations := filepath.Join("..", "..", "..", "migrations", "game_integration_db")
	pool := integrationtest.StartPostgres(t, ctx, "game_integration_test", filepath.Join(migrations, "000001_init.up.sql"))
	ownerID, operatorID, appID, envID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	_, err := pool.Exec(ctx, `INSERT INTO applications (id, owner_account_id, name, status, revision)
		VALUES ($1, $2, 'Legacy approved game', 'sandbox', 2)`, appID, ownerID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO environments (id, application_id, kind, status)
		VALUES ($1, $2, 'sandbox', 'active')`, envID, appID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO registry_audit
		(id, actor_kind, actor_id, application_id, environment_id, action, previous_status, new_status, operation_key)
		VALUES ($1, 'operator', $2, $3, $4, 'approve_sandbox', 'draft', 'sandbox', 'legacy-approval')`,
		uuid.New(), operatorID, appID, envID)
	require.NoError(t, err)
	legacyAccountEventID := uuid.New()
	_, err = pool.Exec(ctx, `INSERT INTO registry_audit
		(id, actor_kind, actor_id, application_id, action, new_status, operation_key)
		VALUES ($1, 'account', $2, $3, 'create_application', 'draft', 'legacy-create')`,
		legacyAccountEventID, ownerID, appID)
	require.NoError(t, err)

	t12Migration, err := os.ReadFile(filepath.Join(migrations, "000002_t12_registry_security.up.sql"))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(t12Migration))
	require.NoError(t, err)

	diagnostics, err := (&Store{Pool: pool}).LoadOwnerDiagnostics(ctx, ownerID, appID)
	require.NoError(t, err)
	require.Equal(t, "operator_approved", diagnostics.ApplicationAdmission,
		"the T12 migration must preserve the provenance of an operator approval already present on the old schema")
	var legacySource, legacyResult string
	require.NoError(t, pool.QueryRow(ctx, `SELECT source, result FROM registry_audit WHERE id=$1`, legacyAccountEventID).
		Scan(&legacySource, &legacyResult))
	require.Equal(t, "system", legacySource, "other legacy audit rows retain the migration's neutral source")
	require.Equal(t, "success", legacyResult, "all prior success rows use the normalized audit result")
}

func TestOwnerDiagnosticsPreserveProvenanceAndRedactSensitiveValues(t *testing.T) {
	ctx := context.Background()
	pool := startT12Postgres(t, ctx)
	now := time.Date(2026, 9, 27, 12, 34, 5, 0, time.UTC)
	resolver := installationResolverFunc(func(context.Context, string, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("93.184.216.34")}, nil
	})
	ownerID, operatorID := uuid.New(), uuid.New()
	store := &Store{Pool: pool, CallbackResolver: resolver, Now: func() time.Time { return now },
		BotAuthority: botAuthorityFunc(func(_ context.Context, _, proofOwner uuid.UUID) error {
			require.Equal(t, ownerID, proofOwner, "proof authority must use the GIS registry owner")
			return nil
		})}
	app, err := store.CreateApplication(ctx, CreateApplicationInput{
		OwnerAccountID: ownerID, Name: "Diagnostic game", IdempotencyKey: "diagnostic-app",
	})
	require.NoError(t, err)
	env, err := store.ApproveSandbox(ctx, ApproveSandboxInput{
		ApplicationID: app.ID, OperatorAccountID: operatorID, IdempotencyKey: "diagnostic-env",
	})
	require.NoError(t, err)
	_, err = store.UpdateSandboxPolicy(ctx, UpdateSandboxPolicyInput{
		OwnerAccountID: ownerID, ApplicationID: app.ID, EnvironmentID: env.ID,
		ExpectedRevision: env.Revision, RedirectURIs: []string{"https://game.example/callback"},
		AllowedOrigins: []string{"https://game.example"}, Providers: []string{"google"},
		PlayerScopes: []string{"game.chat.read"}, IdempotencyKey: "diagnostic-policy",
	})
	require.NoError(t, err)
	key := []byte("0123456789abcdef0123456789abcdef")
	credential, err := store.IssueCredential(ctx, IssueCredentialInput{
		OwnerAccountID: ownerID, ApplicationID: app.ID, EnvironmentID: env.ID,
		Scopes: []string{"game.events.write"}, IdempotencyKey: "diagnostic-credential", SecretKey: key,
	})
	require.NoError(t, err)
	callbackURL := "https://callback.example/private-registration-path"
	installation, err := store.CreateInstallation(ctx, CreateInstallationInput{
		OwnerAccountID: ownerID, ApplicationID: app.ID, EnvironmentID: env.ID,
		BotID: uuid.New(), CallbackURL: callbackURL, IdempotencyKey: "diagnostic-installation",
	})
	require.NoError(t, err)
	var digest []byte
	require.NoError(t, pool.QueryRow(ctx, `SELECT secret_digest FROM service_credentials WHERE id=$1`, credential.ID).Scan(&digest))
	_, err = pool.Exec(ctx, `INSERT INTO registry_audit
		(id, actor_kind, actor_id, application_id, environment_id, action, new_status, operation_key,
		 source, result, reason_code, created_at)
		SELECT gen_random_uuid(), 'account', $2, $1, $3, 'diagnostic_fixture', 'recorded', 'fixture',
		'developer_asserted', 'success', 'fixture-' || n::text, now() + n * interval '1 second'
		FROM generate_series(1,25) AS n`, app.ID, ownerID, env.ID)
	require.NoError(t, err)

	diagnostics, err := store.LoadOwnerDiagnostics(ctx, ownerID, app.ID)
	require.NoError(t, err)
	require.Equal(t, app.ID, diagnostics.ApplicationID)
	require.Equal(t, "sandbox", diagnostics.ApplicationStatus)
	require.Equal(t, "operator_approved", diagnostics.ApplicationAdmission)
	require.Equal(t, "not_verified", diagnostics.ProviderAdmission)
	require.Equal(t, int64(1), diagnostics.Quota.Used)
	require.Equal(t, int64(120), diagnostics.Quota.Limit)
	require.Equal(t, now.Truncate(time.Minute).Add(time.Minute), diagnostics.Quota.ResetAt)
	require.Len(t, diagnostics.Environments, 1)
	environment := diagnostics.Environments[0]
	require.Equal(t, env.ID, environment.ID)
	require.Equal(t, "sandbox", environment.Kind)
	require.Equal(t, "active", environment.Status)
	require.Equal(t, []ProviderAssertion{{Provider: "google", Source: "developer_asserted"}}, environment.ProviderAssertions)
	require.Len(t, environment.Installations, 1)
	require.Equal(t, installation.ID, environment.Installations[0].ID)
	require.Equal(t, "active", environment.Installations[0].Status)
	require.Equal(t, "passed", environment.Installations[0].RegistrationValidation)
	require.Equal(t, "developer_asserted", environment.Installations[0].Source)

	var admissionSource string
	require.NoError(t, pool.QueryRow(ctx, `SELECT source FROM registry_audit WHERE application_id=$1 AND action='approve_sandbox'`, app.ID).Scan(&admissionSource))
	require.Equal(t, "operator_approved", admissionSource)
	require.Len(t, diagnostics.RecentAudit, 20)
	require.Equal(t, "fixture-25", diagnostics.RecentAudit[0].ReasonCode)
	require.Equal(t, "fixture-6", diagnostics.RecentAudit[19].ReasonCode)
	require.Equal(t, "account", diagnostics.RecentAudit[0].ActorKind)
	require.Equal(t, ownerID, diagnostics.RecentAudit[0].ActorID)
	require.Equal(t, app.ID, diagnostics.RecentAudit[0].ApplicationID)
	require.NotNil(t, diagnostics.RecentAudit[0].EnvironmentID)
	require.Equal(t, env.ID, *diagnostics.RecentAudit[0].EnvironmentID)
	require.Equal(t, "developer_asserted", diagnostics.RecentAudit[0].Source)
	require.Equal(t, "success", diagnostics.RecentAudit[0].Result)
	require.False(t, diagnostics.RecentAudit[0].CreatedAt.IsZero())
	for index := 1; index < len(diagnostics.RecentAudit); index++ {
		require.False(t, diagnostics.RecentAudit[index].CreatedAt.After(diagnostics.RecentAudit[index-1].CreatedAt),
			"audit events must be ordered newest first")
	}
	var registrationActor, registrationApp, registrationEnv, registrationInstallation uuid.UUID
	var registrationSource, registrationResult string
	require.NoError(t, pool.QueryRow(ctx, `SELECT actor_id,application_id,environment_id,installation_id,source,result
		FROM registry_audit WHERE application_id=$1 AND action='register_installation'`, app.ID).
		Scan(&registrationActor, &registrationApp, &registrationEnv, &registrationInstallation,
			&registrationSource, &registrationResult))
	require.Equal(t, ownerID, registrationActor)
	require.Equal(t, app.ID, registrationApp)
	require.Equal(t, env.ID, registrationEnv)
	require.Equal(t, installation.ID, registrationInstallation)
	require.Equal(t, "developer_asserted", registrationSource)
	require.Equal(t, "success", registrationResult)

	encoded, err := json.Marshal(diagnostics)
	require.NoError(t, err)
	serialized := string(encoded)
	require.NotContains(t, serialized, credential.Secret)
	require.NotContains(t, serialized, base64.StdEncoding.EncodeToString(digest))
	require.NotContains(t, serialized, hex.EncodeToString(digest))
	require.NotContains(t, serialized, callbackURL)
	require.NotContains(t, serialized, "provider_proof")
	require.NotContains(t, serialized, "oauth_subject")
	require.NotContains(t, serialized, "assertion_token")

	_, err = store.LoadOwnerDiagnostics(ctx, uuid.New(), app.ID)
	require.ErrorIs(t, err, ErrDiagnosticsDenied)
}
