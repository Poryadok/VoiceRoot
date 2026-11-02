//go:build integration

package nativeinstaller

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
	"voice/backend/space/internal/lifecycleprincipal"
	"voice/backend/space/internal/store"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// The caller supplies one genuinely fresh owned private server. This test never
// discovers a shared environment or a Docker socket and never resets a target.
func TestProductionFreshChainAndRetainedVerify(t *testing.T) {
	admin := os.Getenv("NATIVE_INSTALLER_TEST_ADMIN_URL")
	driver := os.Getenv("NATIVE_INSTALLER_TEST_MIGRATE")
	require.NotEmpty(t, admin, "owned private fresh server required")
	require.NotEmpty(t, driver)
	binary := os.Getenv("NATIVE_INSTALLER_TEST_BINARY")
	require.NotEmpty(t, binary, "source-bound production binary required; go test binaries omit dependency build metadata")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	var out bytes.Buffer
	retained := filepath.Join(t.TempDir(), "prior.jsonl")
	run := func(c Config) error {
		args := []string{"-mode", c.Mode, "-migrations-dir", c.MigrationsDir, "-migrate", c.MigratePath}
		if c.Prior != nil {
			args = append(args, "-prior-evidence", retained)
		}
		cmd := exec.CommandContext(ctx, binary, args...)
		cmd.Env = append(os.Environ(), "SPACE_NATIVE_ADMIN_URL="+c.AdminURL)
		cmd.Stdout = c.Output
		var diagnostic bytes.Buffer
		cmd.Stderr = &diagnostic
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("%w: %s", err, diagnostic.String())
		}
		return nil
	}
	c := Config{AdminURL: admin, MigratePath: driver, MigrationsDir: "../../../migrations/space_db", Mode: "provision", Output: &out}
	require.NoError(t, run(c))
	prior, err := ReadReceipt(bytes.NewReader(out.Bytes()))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(retained, out.Bytes(), 0600))
	t.Cleanup(func() {
		cleanupCtx, done := context.WithTimeout(context.Background(), 10*time.Second)
		defer done()
		owner, err := pgx.Connect(cleanupCtx, admin)
		require.NoError(t, err)
		defer owner.Close(context.Background())
		var actualOID uint32
		require.NoError(t, owner.QueryRow(cleanupCtx, `SELECT oid FROM pg_database WHERE datname='space_db'`).Scan(&actualOID))
		require.Equal(t, prior.Creation.DatabaseOID, actualOID)
		_, err = owner.Exec(cleanupCtx, `DROP DATABASE space_db`)
		require.NoError(t, err)
	})
	require.EqualValues(t, 44, prior.Trusted44.Version)
	require.EqualValues(t, 51, prior.Final.Version)
	require.True(t, prior.Transition45.Changed)
	require.True(t, prior.Transition50.Changed)
	require.EqualValues(t, 50, prior.Trusted50.Version)
	require.True(t, prior.Transition51.Changed)
	require.Equal(t, store.NativeCatalogRowJSONProfile, prior.Transition51.Profile)
	out.Reset()
	c.Mode = "verify"
	c.Prior = prior
	require.NoError(t, run(c))
	require.Contains(t, out.String(), `"event":"verified"`)
	require.NotContains(t, out.String(), `"event":"complete"`)
	// Compose grants the separate serving identity only after the initial
	// installer has qualified and sealed the fresh source. Exercise that actual
	// psql script before checking retained provenance with the installer login.
	targetURL, err := url.Parse(admin)
	require.NoError(t, err)
	targetURL.Path = "/space_db"
	cmd := exec.CommandContext(ctx, "psql", targetURL.String(), "-X", "-v", "ON_ERROR_STOP=1", "-f", "../../../../../docker/postgres/provision-space-runtime-role.sql")
	cmd.Env = append(os.Environ(), "SPACE_DB_PASSWORD=isolated-runtime-fixture-only")
	provisionOutput, err := cmd.CombinedOutput()
	require.NoError(t, err, "canonical runtime provisioning: %s", provisionOutput)
	withTarget := func(f func(*pgx.Conn)) {
		conn, err := pgx.Connect(ctx, targetURL.String())
		require.NoError(t, err)
		defer conn.Close(context.Background())
		f(conn)
	}
	withTarget(func(conn *pgx.Conn) {
		var seal, catalog string
		var grants int
		require.NoError(t, conn.QueryRow(ctx, `SELECT encode(catalog_sha256,'hex'),encode(native_membership_guard.catalog(),'hex') FROM native_membership_guard.epoch WHERE singleton`).Scan(&seal, &catalog))
		require.Equal(t, prior.Final.SealSHA256, seal)
		require.Equal(t, seal, catalog, "canonical serving grants must not change the retained catalog seal")
		require.NoError(t, conn.QueryRow(ctx, `SELECT count(*) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace CROSS JOIN LATERAL aclexplode(c.relacl) a WHERE n.nspname='public' AND c.relname IN ('space_account_ban_operations','space_account_ban_captures','space_account_ban_reconciliation','space_account_qualifications','space_account_ban_suppressions','space_account_roster_projections') AND a.grantee='space_runtime'::regrole AND NOT a.is_grantable AND a.privilege_type IN ('SELECT','INSERT','UPDATE','DELETE')`).Scan(&grants))
		require.Equal(t, 24, grants, "all six account-ban relations must have canonical serving DML")
		t.Logf("canonical runtime DML grants=%d retained seal=%s unchanged=true", grants, seal)
	})
	out.Reset()
	require.NoError(t, run(c), "retained source must qualify after canonical serving grants")
	expectedVerified, err := json.Marshal(event{Event: "verified", Receipt: prior})
	require.NoError(t, err)
	require.JSONEq(t, string(expectedVerified), out.String(), "verify preserves the exact original completed receipt")
	t.Run("actual serving login", func(t *testing.T) {
		servingURL := *targetURL
		servingURL.User = url.UserPassword("space_runtime", "isolated-runtime-fixture-only")
		pool, err := pgxpool.New(ctx, servingURL.String())
		require.NoError(t, err)
		defer pool.Close() // installer verification requires the owning window
		var login string
		var owner bool
		require.NoError(t, pool.QueryRow(ctx, `SELECT session_user,pg_has_role(session_user,(SELECT relowner FROM pg_class WHERE oid='public.spaces'::regclass),'MEMBER')`).Scan(&login, &owner))
		require.Equal(t, "space_runtime", login)
		require.False(t, owner)
		st := &store.SpaceStore{Pool: pool}
		row, profile, err := createRuntimeFixtureSpace(t, ctx, st)
		require.NoError(t, err, "actual non-owner native51 creator must retain serving authority")
		require.NotNil(t, row)
		observation, err := st.ReadMemberAdmissionAuthority(ctx, &spacev1.GetMemberAdmissionAuthorityRequest{ProtocolVersion: 1, SpaceId: row.ID.String(), ProfileId: profile})
		require.NoError(t, err, "serving catalog qualification must remain valid")
		require.NotNil(t, observation)
		_, err = st.InspectNativeInstallerSource(ctx, 51)
		require.Error(t, err, "runtime DML never grants owning installer authority")
	})
	for _, tc := range []struct{ name, change, restore string }{
		{"dirty marker", `UPDATE public.schema_migrations SET dirty=true`, `UPDATE public.schema_migrations SET dirty=false`},
		{"unknown marker", `UPDATE public.schema_migrations SET version=52`, `UPDATE public.schema_migrations SET version=51`},
		{"public DML", `GRANT SELECT ON public.space_account_ban_captures TO PUBLIC`, `REVOKE SELECT ON public.space_account_ban_captures FROM PUBLIC`},
		{"unrelated grantee", `CREATE ROLE unrelated_runtime; GRANT SELECT ON public.space_account_ban_captures TO unrelated_runtime`, `REVOKE SELECT ON public.space_account_ban_captures FROM unrelated_runtime; DROP ROLE unrelated_runtime`},
		{"grant option", `GRANT SELECT ON public.space_account_ban_captures TO space_runtime WITH GRANT OPTION`, `REVOKE GRANT OPTION FOR SELECT ON public.space_account_ban_captures FROM space_runtime`},
		{"excess privilege", `GRANT TRUNCATE ON public.space_account_ban_captures TO space_runtime`, `REVOKE TRUNCATE ON public.space_account_ban_captures FROM space_runtime`},
		{"elevated runtime", `ALTER ROLE space_runtime SUPERUSER`, `ALTER ROLE space_runtime NOSUPERUSER`},
		{"owner membership", `GRANT voice TO space_runtime`, `REVOKE voice FROM space_runtime`},
		{"catalog trigger", `ALTER TABLE public.space_account_qualifications DISABLE TRIGGER account_qualification_immutable`, `ALTER TABLE public.space_account_qualifications ENABLE TRIGGER account_qualification_immutable`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			withTarget(func(conn *pgx.Conn) { _, err := conn.Exec(ctx, tc.change); require.NoError(t, err) })
			defer withTarget(func(conn *pgx.Conn) { _, err := conn.Exec(ctx, tc.restore); require.NoError(t, err) })
			out.Reset()
			require.ErrorContains(t, run(c), "HOLD: current51 source verification failed")
			require.Empty(t, out.String(), "rejection must not emit verified provenance")
		})
		out.Reset()
		require.NoError(t, run(c), "restored source must verify without resealing or migration")
	}
	out.Reset()
	c.Mode = "provision"
	c.Prior = nil
	require.ErrorContains(t, run(c), "HOLD")
	require.Empty(t, out.String(), "existing target cannot be relabeled fresh")
}

// The Auth session floor is controlled here; signed transport, durable replay,
// native writer and serving catalog checks use their production implementations.
func createRuntimeFixtureSpace(t *testing.T, ctx context.Context, st *store.SpaceStore) (*store.SpaceRow, string, error) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	request := &spacev1.CreateSpaceRequest{Name: "runtime-grants-fixture", Visibility: "private"}
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	profile, requestID := uuid.NewString(), uuid.NewString()
	token, err := issuer.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "space", RPC: spacev1.SpaceService_CreateSpace_FullMethodName, RequestID: requestID, RequestHash: hash, AccountID: uuid.NewString(), ProfileID: profile, SessionEpoch: 7, AccountType: "regular", ClientExpiresAt: time.Now().Add(time.Minute)})
	require.NoError(t, err)
	ctx = metadata.NewIncomingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", requestID))
	v := lifecycleprincipal.Verifier{Keys: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, CreatorEpoch: func(context.Context, string, int64) error { return nil }, CreatorReplay: st.ClaimCreatorPrincipalReplay}
	result, err := v.Unary()(ctx, request, &grpc.UnaryServerInfo{FullMethod: spacev1.SpaceService_CreateSpace_FullMethodName}, func(ctx context.Context, _ any) (any, error) {
		creator, err := lifecycleprincipal.RequireCreator(ctx, request)
		if err != nil {
			return nil, err
		}
		return st.CreateSpaceWithCreator(ctx, creator, request)
	})
	if err != nil {
		return nil, profile, err
	}
	return result.(*store.SpaceRow), profile, nil
}

// The natural evidence-writer boundary injects loss of the owning deployment
// window before/after every real committed migration and transition. There is
// no production fault flag, fake migrate engine, marker force or epoch update.
func TestCommittedPhaseFailureNeverAutoResumes(t *testing.T) {
	adminURL := os.Getenv("NATIVE_INSTALLER_TEST_ADMIN_URL")
	driver := os.Getenv("NATIVE_INSTALLER_TEST_MIGRATE")
	binary := os.Getenv("NATIVE_INSTALLER_TEST_BINARY")
	helper := os.Getenv("NATIVE_INSTALLER_TEST_FAILURE_HELPER")
	require.NotEmpty(t, adminURL)
	require.NotEmpty(t, driver)
	require.NotEmpty(t, binary, "source-bound production binary required")
	require.NotEmpty(t, helper, "source-bound evidence failure helper required")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	admin, err := pgx.Connect(ctx, adminURL)
	require.NoError(t, err)
	defer admin.Close(context.Background())
	var existed bool
	require.NoError(t, admin.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname='space_db')`).Scan(&existed))
	require.False(t, existed, "fixture must start absent; never reset an unknown database")
	cases := []struct {
		event   string
		version int64
	}{{"created", 0}, {"migrated44", 44}, {"trusted44", 44}, {"migrated45", 45}, {"trusted45", 45}, {"migrated50", 50}, {"trusted50", 50}, {"trusted51", 51}, {"complete", 51}}
	for _, test := range cases {
		t.Run(test.event, func(t *testing.T) {
			migrations := "../../../migrations/space_db"
			injected := exec.CommandContext(ctx, helper, "-at", test.event, "-migrate", driver, "-migrations-dir", migrations)
			injected.Env = append(os.Environ(), "SPACE_NATIVE_ADMIN_URL="+adminURL)
			partial, err := injected.Output()
			var failure *exec.ExitError
			require.ErrorAs(t, err, &failure)
			require.Contains(t, string(failure.Stderr), "controlled deployment evidence persistence failure")
			var ownedOID uint32
			require.NoError(t, admin.QueryRow(ctx, `SELECT oid FROM pg_database WHERE datname='space_db'`).Scan(&ownedOID))
			require.NotZero(t, ownedOID)
			targetCfg, err := pgx.ParseConfig(adminURL)
			require.NoError(t, err)
			targetCfg.Database = "space_db"
			target, err := pgx.ConnectConfig(ctx, targetCfg)
			require.NoError(t, err)
			var marker bool
			require.NoError(t, target.QueryRow(ctx, `SELECT to_regclass('public.schema_migrations') IS NOT NULL`).Scan(&marker))
			if test.version == 0 {
				require.False(t, marker)
			} else {
				var version int64
				var dirty bool
				require.NoError(t, target.QueryRow(ctx, `SELECT version,dirty FROM public.schema_migrations`).Scan(&version, &dirty))
				require.Equal(t, test.version, version)
				require.False(t, dirty)
			}
			require.NoError(t, target.Close(ctx))
			retry := exec.CommandContext(ctx, binary, "-mode", "provision", "-migrate", driver, "-migrations-dir", migrations)
			retry.Env = append(os.Environ(), "SPACE_NATIVE_ADMIN_URL="+adminURL)
			retryOutput, err := retry.CombinedOutput()
			require.Error(t, err)
			require.Contains(t, string(retryOutput), "HOLD: existing database")
			_, err = ReadReceipt(bytes.NewReader(partial))
			require.Error(t, err, "partial events never become completed provenance")
			var sameOID uint32
			require.NoError(t, admin.QueryRow(ctx, `SELECT oid FROM pg_database WHERE datname='space_db'`).Scan(&sameOID))
			require.Equal(t, ownedOID, sameOID)
			// This fixture alone just created this exact database; the external private
			// server is discarded at the end. No cleanup targets an unknown/shared DB.
			_, err = admin.Exec(ctx, `DROP DATABASE space_db`)
			require.NoError(t, err)
		})
	}
}
