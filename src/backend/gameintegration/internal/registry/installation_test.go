package registry

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"voice/backend/gameintegration/internal/callbacksecurity"
	"voice/backend/pkg/integrationtest"
)

type installationResolverFunc func(context.Context, string, string) ([]netip.Addr, error)

func (f installationResolverFunc) LookupNetIP(ctx context.Context, network, host string) ([]netip.Addr, error) {
	return f(ctx, network, host)
}

type installationDialerFunc func(context.Context, string, string) (net.Conn, error)

func (f installationDialerFunc) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return f(ctx, network, address)
}

type botAuthorityFunc func(context.Context, uuid.UUID, uuid.UUID) error

func (f botAuthorityFunc) VerifyGameIntegrationBot(ctx context.Context, botID, ownerID uuid.UUID) error {
	return f(ctx, botID, ownerID)
}

func TestInstallationCallbackPersistenceIsAdmittedAndConsumedThroughPinnedClient(t *testing.T) {
	ctx := context.Background()
	pool := startT12Postgres(t, ctx)
	lookupCalls := 0
	resolver := installationResolverFunc(func(_ context.Context, network, host string) ([]netip.Addr, error) {
		require.Equal(t, "ip", network)
		require.Equal(t, "callback.example", host)
		lookupCalls++
		return []netip.Addr{netip.MustParseAddr("93.184.216.34")}, nil
	})
	ownerID := uuid.New()
	store := &Store{Pool: pool, CallbackResolver: resolver, BotAuthority: botAuthorityFunc(func(_ context.Context, _, owner uuid.UUID) error {
		require.Equal(t, ownerID, owner, "GIS derives the Bot proof owner from the registry row")
		return nil
	})}
	app, err := store.CreateApplication(ctx, CreateApplicationInput{
		OwnerAccountID: ownerID, Name: "HerdTrip", IdempotencyKey: "installation-app",
	})
	require.NoError(t, err)
	env, err := store.ApproveSandbox(ctx, ApproveSandboxInput{
		ApplicationID: app.ID, OperatorAccountID: uuid.New(), IdempotencyKey: "installation-env",
	})
	require.NoError(t, err)

	registered, err := store.CreateInstallation(ctx, CreateInstallationInput{
		OwnerAccountID: ownerID, ApplicationID: app.ID, EnvironmentID: env.ID,
		BotID:       uuid.New(),
		CallbackURL: "https://callback.example/callback-v1", IdempotencyKey: "install-1",
	})
	require.NoError(t, err)
	require.Equal(t, app.ID, registered.ApplicationID)
	require.Equal(t, env.ID, registered.EnvironmentID)
	require.Equal(t, "https://callback.example/callback-v1", registered.CallbackURL)
	var storedAppID, storedEnvironmentID uuid.UUID
	var storedBotID uuid.UUID
	var storedURL string
	require.NoError(t, pool.QueryRow(ctx, `SELECT application_id, environment_id, bot_id, callback_url
		FROM installations WHERE id=$1`, registered.ID).Scan(&storedAppID, &storedEnvironmentID, &storedBotID, &storedURL))
	require.Equal(t, app.ID, storedAppID)
	require.Equal(t, env.ID, storedEnvironmentID)
	require.Equal(t, registered.BotID, storedBotID)
	require.Equal(t, registered.CallbackURL, storedURL)

	var dialAddresses []string
	client := callbacksecurity.NewClient(resolver, installationDialerFunc(func(_ context.Context, _, address string) (net.Conn, error) {
		dialAddresses = append(dialAddresses, address)
		return nil, errors.New("stop after observing approved callback dial")
	}))
	_, err = client.Get(storedURL)
	require.Error(t, err)
	require.Equal(t, 2, lookupCalls,
		"T15 must re-resolve the exact URL admitted for this installation at connection time")
	require.Equal(t, []string{"93.184.216.34:443"}, dialAddresses)

	otherApp, err := store.CreateApplication(ctx, CreateApplicationInput{
		OwnerAccountID: uuid.New(), Name: "Other", IdempotencyKey: "other-app",
	})
	require.NoError(t, err)
	otherEnv, err := store.ApproveSandbox(ctx, ApproveSandboxInput{
		ApplicationID: otherApp.ID, OperatorAccountID: uuid.New(), IdempotencyKey: "other-env",
	})
	require.NoError(t, err)
	_, err = store.CreateInstallation(ctx, CreateInstallationInput{
		OwnerAccountID: ownerID, ApplicationID: app.ID, EnvironmentID: otherEnv.ID,
		BotID:       uuid.New(),
		CallbackURL: "https://callback.example/callback-v1", IdempotencyKey: "cross-env-install",
	})
	require.ErrorIs(t, err, ErrInstallationConflict)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM installations WHERE application_id=$1`, app.ID).Scan(&count))
	require.Equal(t, 1, count, "cross-environment registration must persist no partial installation")
	var credentialCount int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM service_credentials c
		JOIN environments e ON e.id=c.environment_id WHERE e.application_id=$1`, app.ID).Scan(&credentialCount))
	require.Zero(t, credentialCount, "cross-environment registration must not create credentials")
	var quotaCount int
	require.NoError(t, pool.QueryRow(ctx, `SELECT request_count FROM app_quota_windows WHERE application_id=$1`, app.ID).Scan(&quotaCount))
	require.Equal(t, 2, quotaCount, "the owner-scoped attempt is charged after app resolution and limiter admission")
	var deniedAudits int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM registry_audit
		WHERE application_id=$1 AND environment_id=$2 AND actor_id=$3 AND action='register_installation'
		AND result='denied' AND source='authenticated_account'`, app.ID, otherEnv.ID, ownerID).Scan(&deniedAudits))
	require.Equal(t, 1, deniedAudits, "cross-environment denial records only sanitized provenance")

	foreignOwner := uuid.New()
	_, err = store.CreateInstallation(ctx, CreateInstallationInput{
		OwnerAccountID: foreignOwner, ApplicationID: app.ID, EnvironmentID: env.ID,
		BotID:       uuid.New(),
		CallbackURL: "https://callback.example/callback-v1", IdempotencyKey: "foreign-owner-install",
	})
	require.ErrorIs(t, err, ErrInstallationConflict)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM installations WHERE application_id=$1`, app.ID).Scan(&count))
	require.Equal(t, 1, count, "a foreign owner must not create an installation")
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM service_credentials c
		JOIN environments e ON e.id=c.environment_id WHERE e.application_id=$1`, app.ID).Scan(&credentialCount))
	require.Zero(t, credentialCount, "a foreign owner must not create credentials")
	require.NoError(t, pool.QueryRow(ctx, `SELECT request_count FROM app_quota_windows WHERE application_id=$1`, app.ID).Scan(&quotaCount))
	require.Equal(t, 2, quotaCount, "a foreign owner is rejected before quota admission")
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM registry_audit
		WHERE application_id=$1 AND environment_id=$2 AND actor_id=$3 AND action='register_installation'
		AND result='denied' AND source='authenticated_account'`, app.ID, env.ID, foreignOwner).Scan(&deniedAudits))
	require.Equal(t, 1, deniedAudits, "authenticated foreign-owner denial is durably audited without payload")
}

func TestInstallationCallbackRejectsUnsafeDNSBeforePersistence(t *testing.T) {
	ctx := context.Background()
	pool := startT12Postgres(t, ctx)
	resolver := installationResolverFunc(func(context.Context, string, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("93.184.216.34"), netip.MustParseAddr("169.254.169.254")}, nil
	})
	ownerID := uuid.New()
	store := &Store{Pool: pool, CallbackResolver: resolver, BotAuthority: botAuthorityFunc(func(context.Context, uuid.UUID, uuid.UUID) error { return nil })}
	app, err := store.CreateApplication(ctx, CreateApplicationInput{
		OwnerAccountID: ownerID, Name: "HerdTrip", IdempotencyKey: "unsafe-installation-app",
	})
	require.NoError(t, err)
	env, err := store.ApproveSandbox(ctx, ApproveSandboxInput{
		ApplicationID: app.ID, OperatorAccountID: uuid.New(), IdempotencyKey: "unsafe-installation-env",
	})
	require.NoError(t, err)

	_, err = store.CreateInstallation(ctx, CreateInstallationInput{
		OwnerAccountID: ownerID, ApplicationID: app.ID, EnvironmentID: env.ID,
		BotID:       uuid.New(),
		CallbackURL: "https://callback.example/callback-v1", IdempotencyKey: "unsafe-installation",
	})
	require.ErrorIs(t, err, callbacksecurity.ErrUnsafeURL)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM installations WHERE application_id=$1`, app.ID).Scan(&count))
	require.Zero(t, count, "unsafe callbacks must persist no installation or credential material")
	var credentialCount int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM service_credentials c
		JOIN environments e ON e.id=c.environment_id WHERE e.application_id=$1`, app.ID).Scan(&credentialCount))
	require.Zero(t, credentialCount, "unsafe callbacks must not create credentials")
	var quotaCount int
	require.NoError(t, pool.QueryRow(ctx, `SELECT request_count FROM app_quota_windows WHERE application_id=$1`, app.ID).Scan(&quotaCount))
	require.Equal(t, 1, quotaCount, "the app owner was accepted by the limiter before destination validation")
	var auditCount int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM registry_audit
		WHERE application_id=$1 AND environment_id=$2 AND actor_id=$3 AND action='register_installation'
		AND result='denied'`, app.ID, env.ID, ownerID).Scan(&auditCount))
	require.Equal(t, 1, auditCount, "unsafe destination denial must create exactly one sanitized event")
	var actorID, auditedAppID, auditedEnvID uuid.UUID
	var source, result, action string
	var eventJSON string
	require.NoError(t, pool.QueryRow(ctx, `SELECT to_jsonb(registry_audit)::text, actor_id, application_id, environment_id, source, result, action
		FROM registry_audit WHERE application_id=$1 AND action='register_installation' AND result='denied'`, app.ID).
		Scan(&eventJSON, &actorID, &auditedAppID, &auditedEnvID, &source, &result, &action))
	require.NotContains(t, eventJSON, "callback.example")
	require.NotContains(t, eventJSON, "callback_url")
	require.NotContains(t, eventJSON, "payload")
	require.NotContains(t, eventJSON, "credential")
	require.Equal(t, ownerID, actorID)
	require.Equal(t, app.ID, auditedAppID)
	require.Equal(t, env.ID, auditedEnvID)
	require.Equal(t, "developer_asserted", source)
	require.Equal(t, "denied", result)
	require.Equal(t, "register_installation", action)
}

func TestInstallationQuotaIsAppWideIndependentAndResetsAtUTCMinute(t *testing.T) {
	ctx := context.Background()
	pool := startT12Postgres(t, ctx)
	resolver := installationResolverFunc(func(context.Context, string, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("93.184.216.34")}, nil
	})
	now := time.Date(2026, 9, 27, 12, 34, 5, 0, time.UTC)
	ownerID := uuid.New()
	store := &Store{Pool: pool, CallbackResolver: resolver, Now: func() time.Time { return now },
		BotAuthority: botAuthorityFunc(func(context.Context, uuid.UUID, uuid.UUID) error { return nil })}
	app, err := store.CreateApplication(ctx, CreateApplicationInput{
		OwnerAccountID: ownerID, Name: "Quota", IdempotencyKey: "quota-installation-app",
	})
	require.NoError(t, err)
	env, err := store.ApproveSandbox(ctx, ApproveSandboxInput{
		ApplicationID: app.ID, OperatorAccountID: uuid.New(), IdempotencyKey: "quota-installation-env",
	})
	require.NoError(t, err)
	// Seed a valid production environment through the existing integration-test
	// fixture path: production admission is distinct and not implemented here.
	env2ID := uuid.New()
	_, err = pool.Exec(ctx, `UPDATE applications SET status='active' WHERE id=$1`, app.ID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO environments (id, application_id, kind, status)
		VALUES ($1,$2,'production','active')`, env2ID, app.ID)
	require.NoError(t, err)
	otherApp, err := store.CreateApplication(ctx, CreateApplicationInput{
		OwnerAccountID: ownerID, Name: "Quota Other", IdempotencyKey: "quota-installation-other-app",
	})
	require.NoError(t, err)
	otherEnv, err := store.ApproveSandbox(ctx, ApproveSandboxInput{
		ApplicationID: otherApp.ID, OperatorAccountID: uuid.New(), IdempotencyKey: "quota-installation-other-env",
	})
	require.NoError(t, err)

	create := func(appID, envID uuid.UUID, key string) error {
		_, createErr := store.CreateInstallation(ctx, CreateInstallationInput{
			OwnerAccountID: ownerID, ApplicationID: appID, EnvironmentID: envID,
			BotID:       uuid.New(),
			CallbackURL: "https://callback.example/callback-v1", IdempotencyKey: key,
		})
		return createErr
	}
	now = time.Date(2026, 9, 27, 12, 35, 0, 0, time.UTC)

	for n := 1; n <= 120; n++ {
		environmentID := env.ID
		if n%2 == 0 {
			environmentID = env2ID
		}
		require.NoError(t, create(app.ID, environmentID, fmt.Sprintf("quota-install-%03d", n)),
			"request %d should fit in the shared app quota across both environments", n)
	}
	for n := 1; n <= 120; n++ {
		require.NoError(t, create(otherApp.ID, otherEnv.ID, fmt.Sprintf("quota-other-install-%03d", n)),
			"a second app receives its own full quota")
	}
	now = time.Date(2026, 9, 27, 12, 35, 5, 0, time.UTC)
	err = create(app.ID, env.ID, "quota-install-121")
	var rateLimitErr *RateLimitError
	require.ErrorAs(t, err, &rateLimitErr)
	require.Equal(t, 55*time.Second, rateLimitErr.RetryAfter,
		"at 12:35:05 UTC, Retry-After reaches the 12:36:00 UTC boundary")
	require.ErrorIs(t, create(app.ID, env2ID, "quota-install-122"), ErrRateLimited)
	require.ErrorIs(t, create(otherApp.ID, otherEnv.ID, "quota-other-install-121"), ErrRateLimited)
	require.ErrorIs(t, create(otherApp.ID, otherEnv.ID, "quota-other-install-122"), ErrRateLimited)
	var count, quotaCount, credentialCount int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM installations WHERE application_id=$1`, app.ID).Scan(&count))
	require.Equal(t, 120, count, "the denied 121st attempt must create no installation")
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM service_credentials c
		JOIN environments e ON e.id=c.environment_id WHERE e.application_id=$1`, app.ID).Scan(&credentialCount))
	require.Zero(t, credentialCount, "quota rejection must create no credentials")
	require.NoError(t, pool.QueryRow(ctx, `SELECT request_count FROM app_quota_windows WHERE application_id=$1`, app.ID).Scan(&quotaCount))
	require.Equal(t, 120, quotaCount, "a rejected attempt must not overrun the durable quota")
	var denialRows, denialCount int
	var quotaEventJSON string
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*), coalesce(sum(denial_count),0),
		coalesce((array_agg(to_jsonb(registry_audit)::text ORDER BY created_at))[1], '') FROM registry_audit
		WHERE application_id=$1 AND action='quota_denied'`, app.ID).Scan(&denialRows, &denialCount, &quotaEventJSON))
	require.Equal(t, 1, denialRows, "repeated quota rejections coalesce to one app-minute event")
	require.Equal(t, 2, denialCount, "coalesced quota event counts both 429 responses")
	require.NotContains(t, quotaEventJSON, "callback.example")
	require.NotContains(t, quotaEventJSON, "callback_url")
	require.NotContains(t, quotaEventJSON, "payload")
	require.NotContains(t, quotaEventJSON, "credential")
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*), coalesce(sum(denial_count),0),
		coalesce((array_agg(to_jsonb(registry_audit)::text ORDER BY created_at))[1], '') FROM registry_audit
		WHERE application_id=$1 AND action='quota_denied'`, otherApp.ID).Scan(&denialRows, &denialCount, &quotaEventJSON))
	require.Equal(t, 1, denialRows)
	require.Equal(t, 2, denialCount)
	require.NotContains(t, quotaEventJSON, "callback.example")
	require.NotContains(t, quotaEventJSON, "callback_url")
	require.NotContains(t, quotaEventJSON, "payload")
	require.NotContains(t, quotaEventJSON, "credential")
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM installations WHERE application_id=$1`, otherApp.ID).Scan(&count))
	require.Equal(t, 120, count, "the second app independently uses its full quota")
	require.NoError(t, pool.QueryRow(ctx, `SELECT request_count FROM app_quota_windows WHERE application_id=$1`, otherApp.ID).Scan(&quotaCount))
	require.Equal(t, 120, quotaCount)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM service_credentials c
		JOIN environments e ON e.id=c.environment_id WHERE e.application_id=$1`, otherApp.ID).Scan(&credentialCount))
	require.Zero(t, credentialCount, "the second app's rejected attempts create no credentials")

	now = time.Date(2026, 9, 27, 12, 36, 0, 0, time.UTC)
	require.NoError(t, create(app.ID, env.ID, "quota-after-boundary"), "the UTC-minute rollover admits the next attempt")
	require.NoError(t, pool.QueryRow(ctx, `SELECT request_count FROM app_quota_windows WHERE application_id=$1`, app.ID).Scan(&quotaCount))
	require.Equal(t, 1, quotaCount, "the new UTC minute begins a fresh app quota window")
	for n := 2; n <= 120; n++ {
		environmentID := env.ID
		if n%2 == 0 {
			environmentID = env2ID
		}
		require.NoError(t, create(app.ID, environmentID, fmt.Sprintf("quota-after-boundary-%03d", n)))
	}
	now = time.Date(2026, 9, 27, 12, 36, 5, 0, time.UTC)
	err = create(app.ID, env.ID, "quota-after-boundary-121")
	require.ErrorAs(t, err, &rateLimitErr)
	require.Equal(t, 55*time.Second, rateLimitErr.RetryAfter)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*), coalesce(sum(denial_count),0) FROM registry_audit
		WHERE application_id=$1 AND action='quota_denied'`, app.ID).Scan(&denialRows, &denialCount))
	require.Equal(t, 2, denialRows, "a new UTC-minute window creates its own coalesced denial event")
	require.Equal(t, 3, denialCount, "the new bucket increments independently from the prior minute")
	var newMinuteDenials int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM registry_audit WHERE application_id=$1
		AND action='quota_denied' AND quota_window_start=$2`, app.ID, time.Date(2026, 9, 27, 12, 36, 0, 0, time.UTC)).Scan(&newMinuteDenials))
	require.Equal(t, 1, newMinuteDenials, "the event is keyed to the UTC-minute bucket")
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM installations WHERE application_id=$1`, app.ID).Scan(&count))
	require.Equal(t, 240, count, "the second-minute 121st attempt creates no installation")
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM service_credentials c
		JOIN environments e ON e.id=c.environment_id WHERE e.application_id=$1`, app.ID).Scan(&credentialCount))
	require.Zero(t, credentialCount)
}

func TestInstallationCallbackRejectsPendingEnvironmentBeforePersistence(t *testing.T) {
	ctx := context.Background()
	pool := startT12Postgres(t, ctx)
	resolver := installationResolverFunc(func(context.Context, string, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("93.184.216.34")}, nil
	})
	ownerID := uuid.New()
	store := &Store{Pool: pool, CallbackResolver: resolver, BotAuthority: botAuthorityFunc(func(context.Context, uuid.UUID, uuid.UUID) error { return nil })}
	app, err := store.CreateApplication(ctx, CreateApplicationInput{
		OwnerAccountID: ownerID, Name: "Pending Callback", IdempotencyKey: "pending-install-app",
	})
	require.NoError(t, err)
	_, err = store.ApproveSandbox(ctx, ApproveSandboxInput{
		ApplicationID: app.ID, OperatorAccountID: uuid.New(), IdempotencyKey: "pending-install-sandbox",
	})
	require.NoError(t, err)
	pendingEnvID := uuid.New()
	_, err = pool.Exec(ctx, `INSERT INTO environments (id, application_id, kind, status)
		VALUES ($1, $2, 'production', 'pending')`, pendingEnvID, app.ID)
	require.NoError(t, err)

	_, err = store.CreateInstallation(ctx, CreateInstallationInput{
		OwnerAccountID: ownerID, ApplicationID: app.ID, EnvironmentID: pendingEnvID,
		BotID:       uuid.New(),
		CallbackURL: "https://callback.example/callback-v1", IdempotencyKey: "pending-install",
	})
	require.ErrorIs(t, err, ErrInstallationConflict, "callbacks can only be registered on active environments")
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM installations WHERE application_id=$1`, app.ID).Scan(&count))
	require.Zero(t, count, "pending environment denial must not persist an installation")
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM service_credentials WHERE environment_id=$1`, pendingEnvID).Scan(&count))
	require.Zero(t, count, "pending environment denial must not persist credentials")
	var quotaCount int
	require.NoError(t, pool.QueryRow(ctx, `SELECT request_count FROM app_quota_windows WHERE application_id=$1`, app.ID).Scan(&quotaCount))
	require.Equal(t, 1, quotaCount, "the authenticated app owner's attempt was admitted before environment validation")
	var auditCount int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM registry_audit
		WHERE application_id=$1 AND environment_id=$2 AND actor_id=$3 AND action='register_installation'
		AND result='denied' AND reason_code='environment_scope_denied'`, app.ID, pendingEnvID, ownerID).Scan(&auditCount))
	require.Equal(t, 1, auditCount, "pending environment denial is represented by one sanitized scope audit")
}

func TestInstallationRequiresBotAuthorityProofBeforePersistence(t *testing.T) {
	ctx := context.Background()
	pool := startT12Postgres(t, ctx)
	ownerID, botID := uuid.New(), uuid.New()
	store := &Store{Pool: pool}
	app, err := store.CreateApplication(ctx, CreateApplicationInput{
		OwnerAccountID: ownerID, Name: "Unproven Bot", IdempotencyKey: "unproven-bot-app",
	})
	require.NoError(t, err)
	env, err := store.ApproveSandbox(ctx, ApproveSandboxInput{
		ApplicationID: app.ID, OperatorAccountID: uuid.New(), IdempotencyKey: "unproven-bot-env",
	})
	require.NoError(t, err)

	_, err = store.CreateInstallation(ctx, CreateInstallationInput{
		OwnerAccountID: ownerID, ApplicationID: app.ID, EnvironmentID: env.ID,
		BotID: botID, CallbackURL: "https://callback.example/callback-v1", IdempotencyKey: "unproven-bot-install",
	})
	require.ErrorIs(t, err, ErrBotAuthorityUnavailable)
	var installationCount, successfulOperations int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM installations WHERE application_id=$1`, app.ID).Scan(&installationCount))
	require.Zero(t, installationCount, "missing Bot proof must not persist an active installation")
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM registry_operations
		WHERE actor_id=$1 AND route=$2 AND idempotency_key=$3 AND status='succeeded'`,
		ownerID, createInstallationRoute, "unproven-bot-install").Scan(&successfulOperations))
	require.Zero(t, successfulOperations, "missing Bot proof must not persist a successful idempotency result")
}

func TestInstallationRechecksOwnerAndLifecycleAfterBotProof(t *testing.T) {
	ctx := context.Background()
	pool := startT12Postgres(t, ctx)
	resolver := installationResolverFunc(func(context.Context, string, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("93.184.216.34")}, nil
	})
	store := &Store{Pool: pool, CallbackResolver: resolver}
	for _, scenario := range []struct {
		name       string
		want       error
		mutateRows func(uuid.UUID, uuid.UUID, uuid.UUID) error
	}{
		{
			name: "owner changed during proof",
			want: ErrInstallationConflict,
			mutateRows: func(appID, _, newOwner uuid.UUID) error {
				_, err := pool.Exec(ctx, `UPDATE applications SET owner_account_id=$2 WHERE id=$1`, appID, newOwner)
				return err
			},
		},
		{
			name: "application suspended during proof",
			want: ErrApplicationSuspended,
			mutateRows: func(appID, _, _ uuid.UUID) error {
				_, err := pool.Exec(ctx, `UPDATE applications SET status='suspended' WHERE id=$1`, appID)
				return err
			},
		},
		{
			name: "environment suspended during proof",
			want: ErrInstallationConflict,
			mutateRows: func(_, environmentID, _ uuid.UUID) error {
				_, err := pool.Exec(ctx, `UPDATE environments SET status='suspended' WHERE id=$1`, environmentID)
				return err
			},
		},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			ownerID, newOwnerID := uuid.New(), uuid.New()
			app, err := store.CreateApplication(ctx, CreateApplicationInput{
				OwnerAccountID: ownerID, Name: "Proof race " + scenario.name, IdempotencyKey: "race-app-" + scenario.name,
			})
			require.NoError(t, err)
			env, err := store.ApproveSandbox(ctx, ApproveSandboxInput{
				ApplicationID: app.ID, OperatorAccountID: uuid.New(), IdempotencyKey: "race-env-" + scenario.name,
			})
			require.NoError(t, err)
			store.BotAuthority = botAuthorityFunc(func(_ context.Context, _, proofOwner uuid.UUID) error {
				require.Equal(t, ownerID, proofOwner, "Bot proof must use the owner read from GIS registry preflight")
				return scenario.mutateRows(app.ID, env.ID, newOwnerID)
			})

			_, err = store.CreateInstallation(ctx, CreateInstallationInput{
				OwnerAccountID: ownerID, ApplicationID: app.ID, EnvironmentID: env.ID,
				BotID: uuid.New(), CallbackURL: "https://callback.example/hook",
				IdempotencyKey: "race-install-" + scenario.name,
			})
			require.ErrorIs(t, err, scenario.want)
			var installations, successfulOperations int
			require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM installations WHERE application_id=$1`, app.ID).Scan(&installations))
			require.Zero(t, installations, "a registry owner or lifecycle change during proof cannot be bound")
			require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM registry_operations WHERE actor_id=$1 AND route=$2 AND idempotency_key=$3 AND status='succeeded'`,
				ownerID, createInstallationRoute, "race-install-"+scenario.name).Scan(&successfulOperations))
			require.Zero(t, successfulOperations, "proof race denial cannot create a successful idempotency result")
		})
	}
}

func startT12Postgres(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()
	migrations := filepath.Join("..", "..", "..", "migrations", "game_integration_db")
	pool := integrationtest.StartPostgres(t, ctx, "game_integration_test", filepath.Join(migrations, "000001_init.up.sql"))
	t12Migration, err := os.ReadFile(filepath.Join(migrations, "000002_t12_registry_security.up.sql"))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(t12Migration))
	require.NoError(t, err)
	t11Migration, err := os.ReadFile(filepath.Join(migrations, "000004_t11_installation_bot_binding.up.sql"))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(t11Migration))
	require.NoError(t, err)
	return pool
}
