package sourcecustody

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"voice/backend/user/internal/identityassociation"
)

func openSourceFixture(t *testing.T, tracers ...pgx.QueryTracer) (context.Context, *pgxpool.Pool, *pgxpool.Pool, *pgxpool.Pool, *identityassociation.Keyset, *Controller, OpenAttestation, ed25519.PublicKey, uuid.UUID, uuid.UUID) {
	return openSourceFixtureOptions(t, true, tracers...)
}

func openSourceFixtureOptions(t *testing.T, guardsActive bool, tracers ...pgx.QueryTracer) (context.Context, *pgxpool.Pool, *pgxpool.Pool, *pgxpool.Pool, *identityassociation.Keyset, *Controller, OpenAttestation, ed25519.PublicKey, uuid.UUID, uuid.UUID) {
	return openSourceFixtureWithParent(t, guardsActive, nil, tracers...)
}

func openSourceFixtureWithParent(t *testing.T, guardsActive bool, lifetime func(context.Context) context.Context, tracers ...pgx.QueryTracer) (context.Context, *pgxpool.Pool, *pgxpool.Pool, *pgxpool.Pool, *identityassociation.Keyset, *Controller, OpenAttestation, ed25519.PublicKey, uuid.UUID, uuid.UUID) {
	t.Helper()
	ctx, control, admin, request := creationFixture(t)
	physical, err := CreateClosedSource(ctx, control, admin, request)
	require.NoError(t, err)
	dir, keys := migrationFixture(t)
	require.NoError(t, InitializeClosedSource(ctx, control, admin, request, physical, dir, keys))
	_, err = admin.Exec(ctx, `CREATE ROLE custody_app NOLOGIN PASSWORD 'fixture_only'`)
	require.NoError(t, err)
	_, err = admin.Exec(ctx, `GRANT CONNECT ON DATABASE `+pgx.Identifier{physical.Database}.Sanitize()+` TO custody_app`)
	require.NoError(t, err)
	isolateWriterDatabases(t, admin, physical.Database)
	_, err = admin.Exec(ctx, `ALTER DATABASE `+pgx.Identifier{physical.Database}.Sanitize()+` WITH ALLOW_CONNECTIONS true`)
	require.NoError(t, err)
	config := admin.Config().Copy()
	config.ConnConfig.Database = physical.Database
	if len(tracers) != 0 {
		config.ConnConfig.Tracer = tracers[0]
	}
	config.MaxConns = 1
	config.MinConns = 0
	source, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	t.Cleanup(source.Close)
	_, err = source.Exec(ctx, `GRANT USAGE ON SCHEMA public TO custody_app;
GRANT SELECT,INSERT,UPDATE,DELETE ON ALL TABLES IN SCHEMA public TO custody_app;
GRANT USAGE,SELECT,UPDATE ON ALL SEQUENCES IN SCHEMA public TO custody_app;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO custody_app;
GRANT EXECUTE ON FUNCTION pg_catalog.pg_control_system() TO custody_app`)
	require.NoError(t, err)
	profile, account := uuid.New(), uuid.New()
	_, err = source.Exec(ctx, `INSERT INTO profiles(id,account_id,username,discriminator,display_name) VALUES($1,$2,'retained','1234','Retained')`, profile, account)
	require.NoError(t, err)
	var binding Binding
	var digest []byte
	require.NoError(t, control.QueryRow(ctx, `SELECT epoch,source_id,history_generation,user_source_generation,keyset_digest FROM user_custody_state WHERE singleton`).Scan(&binding.Epoch, &binding.SourceID, &binding.HistoryGeneration, &binding.UserSourceGeneration, &digest))
	copy(binding.KeysetDigest[:], digest)
	store, err := identityassociation.NewStore(source, keys, binding.HistoryGeneration)
	require.NoError(t, err)
	if guardsActive {
		// Fixture-only activation stands in for a previously qualified cutover.
		require.NoError(t, store.ActivateGuards(ctx))
	}
	source.Close()
	_, err = admin.Exec(ctx, `ALTER DATABASE `+pgx.Identifier{physical.Database}.Sanitize()+` WITH ALLOW_CONNECTIONS false`)
	require.NoError(t, err)
	source, err = pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	t.Cleanup(source.Close)
	parent := ctx
	if lifetime != nil {
		parent = lifetime(ctx)
	}
	controller, err := Start(parent, control, binding)
	require.NoError(t, err)
	t.Cleanup(func() { _ = controller.Shutdown(context.Background()) })
	challenge, err := controller.ActivationChallenge()
	require.NoError(t, err)
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	now := time.Now()
	claim := OpenAttestation{Challenge: challenge, ControlSystem: controller.controlSystem, ControlDatabase: controller.controlDatabase, ControlDatabaseOID: controller.controlDatabaseOID, Binding: binding, Physical: physical, WriterRole: "custody_app", SourceEvidenceDigest: [32]byte{3}, WriterManifestDigest: [32]byte{1}, AuthEvidenceDigest: [32]byte{2}, GuardCutover: !guardsActive, IssuedAtUnix: now.Unix(), ExpiresAtUnix: now.Add(4 * time.Minute).Unix()}
	payload, err := OpenAttestationPayload(claim)
	require.NoError(t, err)
	claim.Signature = ed25519.Sign(private, payload)
	return ctx, control, admin, source, keys, controller, claim, public, profile, account
}

type openingCommitTrace struct {
	armed   atomic.Bool
	entered chan struct{}
}

func (t *openingCommitTrace) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	if strings.EqualFold(strings.TrimSpace(data.SQL), "commit") && t.armed.CompareAndSwap(true, false) {
		close(t.entered)
		<-ctx.Done()
	}
	return ctx
}

func (*openingCommitTrace) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

func TestOpenQualifiedFencesAfterCancelledSourceCommit(t *testing.T) {
	trace := &openingCommitTrace{entered: make(chan struct{})}
	ctx, control, admin, source, keys, controller, claim, public, _, _ := openSourceFixture(t, trace)
	require.NoError(t, controller.PrepareOpenSource(ctx, admin, claim, public))
	call, cancel := context.WithCancel(ctx)
	defer cancel()
	trace.armed.Store(true)
	result := make(chan error, 1)
	go func() { result <- controller.OpenQualified(call, admin, source, keys, claim, public) }()
	select {
	case <-trace.entered:
	case err := <-result:
		t.Fatalf("opening ended before the source commit: %v", err)
	case <-time.After(20 * time.Second):
		t.Fatal("opening did not reach the source commit")
	}
	var state string
	require.NoError(t, control.QueryRow(ctx, `SELECT state FROM user_custody_state WHERE singleton`).Scan(&state))
	require.Equal(t, "OPEN", state, "control commit precedes the writer-login commit")
	cancel()
	select {
	case err := <-result:
		require.Error(t, err)
	case <-time.After(15 * time.Second):
		t.Fatal("cancelled opening did not fence")
	}
	var phase string
	var allowed, login bool
	require.NoError(t, control.QueryRow(ctx, `SELECT state FROM user_custody_state WHERE singleton`).Scan(&state))
	require.Equal(t, "CLOSED", state)
	require.NoError(t, control.QueryRow(ctx, `SELECT phase FROM user_custody_creation WHERE singleton`).Scan(&phase))
	require.Equal(t, "RECOVERY_REQUIRED", phase)
	require.NoError(t, admin.QueryRow(ctx, `SELECT datallowconn FROM pg_database WHERE oid=$1`, claim.Physical.DatabaseOID).Scan(&allowed))
	require.False(t, allowed)
	require.NoError(t, admin.QueryRow(ctx, `SELECT rolcanlogin FROM pg_roles WHERE rolname=$1`, claim.WriterRole).Scan(&login))
	require.False(t, login)
}

func TestOpenQualifiedEnablesOnlyAttestedSourceAndCannotReplay(t *testing.T) {
	ctx, control, admin, source, keys, controller, claim, public, profile, account := openSourceFixture(t)
	bad := claim
	bad.Signature = append([]byte(nil), claim.Signature...)
	bad.Signature[0] ^= 1
	require.Error(t, controller.OpenQualified(ctx, admin, source, keys, bad, public))
	var state string
	var login bool
	require.NoError(t, control.QueryRow(ctx, `SELECT state FROM user_custody_state WHERE singleton`).Scan(&state))
	require.Equal(t, "CLOSED", state)
	require.NoError(t, admin.QueryRow(ctx, `SELECT rolcanlogin FROM pg_roles WHERE rolname='custody_app'`).Scan(&login))
	require.False(t, login)
	require.NoError(t, controller.PrepareOpenSource(ctx, admin, claim, public))
	require.NoError(t, controller.OpenQualified(ctx, admin, source, keys, claim, public))
	require.NoError(t, control.QueryRow(ctx, `SELECT state FROM user_custody_state WHERE singleton`).Scan(&state))
	require.Equal(t, "OPEN", state)
	require.NoError(t, admin.QueryRow(ctx, `SELECT rolcanlogin FROM pg_roles WHERE rolname='custody_app'`).Scan(&login))
	require.True(t, login)
	var phase string
	var evidence int
	require.NoError(t, control.QueryRow(ctx, `SELECT phase,(SELECT count(*) FROM user_custody_open_evidence) FROM user_custody_creation WHERE singleton`).Scan(&phase, &evidence))
	require.Equal(t, "ACTIVE", phase)
	require.Equal(t, 1, evidence)
	permit, physical, err := controller.AcquireAttested(ctx, claim.Binding)
	require.NoError(t, err)
	require.Equal(t, claim.Physical, physical)
	require.NoError(t, permit.Release(ctx))
	config := source.Config().Copy()
	config.ConnConfig.User = "custody_app"
	config.ConnConfig.Password = "fixture_only"
	app, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	store, err := identityassociation.NewStore(app, keys, claim.Binding.HistoryGeneration)
	require.NoError(t, err)
	result, err := store.CaptureProfilesForPhysicalSource(ctx, []uuid.UUID{profile}, claim.Binding.HistoryGeneration, claim.Binding.UserSourceGeneration, claim.Binding.KeysetDigest, identityassociation.PhysicalSourceIdentity{SystemIdentifier: physical.SystemIdentifier, Database: physical.Database, DatabaseOID: physical.DatabaseOID})
	require.NoError(t, err)
	require.Equal(t, account, result.Observations[profile].AccountID)
	app.Close()
	require.NoError(t, controller.CloseAdmission(ctx))
	_, err = control.Exec(ctx, `UPDATE user_custody_state SET state='OPEN' WHERE singleton`)
	require.NoError(t, err)
	_, _, err = controller.AcquireAttested(ctx, claim.Binding)
	require.Error(t, err, "direct SQL OPEN cannot authorize a grant")
	require.NoError(t, controller.CloseAdmission(ctx))
	require.Error(t, controller.OpenQualified(ctx, admin, source, keys, claim, public), "a consumed decision cannot reopen")
	require.NoError(t, controller.Shutdown(ctx))
	restarted, err := Start(ctx, control, claim.Binding)
	require.NoError(t, err)
	defer func() { require.NoError(t, restarted.Shutdown(ctx)) }()
	require.Error(t, restarted.OpenQualified(ctx, admin, source, keys, claim, public), "an old leader's decision cannot authorize a restart")
}

func TestOpenQualifiedRejectsUndrainedSource(t *testing.T) {
	ctx, control, admin, source, keys, controller, claim, public, _, _ := openSourceFixture(t)
	require.NoError(t, controller.PrepareOpenSource(ctx, admin, claim, public))
	config := source.Config().Copy()
	other, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	defer other.Close()
	require.NoError(t, other.Ping(ctx))
	require.Error(t, controller.OpenQualified(ctx, admin, source, keys, claim, public), "an extra source session must block admission")
	var state string
	var login bool
	require.NoError(t, control.QueryRow(ctx, `SELECT state FROM user_custody_state WHERE singleton`).Scan(&state))
	require.Equal(t, "CLOSED", state)
	require.NoError(t, admin.QueryRow(ctx, `SELECT rolcanlogin FROM pg_roles WHERE rolname='custody_app'`).Scan(&login))
	require.False(t, login)
	_, err = controller.ActivationChallenge()
	require.Error(t, err, "a decision is consumed on any local validation failure")
}

func TestOpenQualifiedRejectsWriterSessionInAnotherDatabase(t *testing.T) {
	ctx, control, admin, _, _, controller, claim, public, _, _ := openSourceFixture(t)
	_, err := admin.Exec(ctx, `GRANT CONNECT ON DATABASE postgres TO custody_app; ALTER ROLE custody_app LOGIN`)
	require.NoError(t, err)
	config := admin.Config().Copy()
	config.ConnConfig.Database = "postgres"
	config.ConnConfig.User = "custody_app"
	config.ConnConfig.Password = "fixture_only"
	stray, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	defer stray.Close()
	require.NoError(t, stray.Ping(ctx))
	_, err = admin.Exec(ctx, `ALTER ROLE custody_app NOLOGIN; REVOKE CONNECT ON DATABASE postgres FROM custody_app`)
	require.NoError(t, err)
	require.NoError(t, stray.Ping(ctx), "revoking LOGIN and CONNECT does not end an existing session")
	require.Error(t, controller.PrepareOpenSource(ctx, admin, claim, public), "a stale authenticated writer must block source opening")
	var state string
	require.NoError(t, control.QueryRow(ctx, `SELECT state FROM user_custody_state WHERE singleton`).Scan(&state))
	require.Equal(t, "CLOSED", state)
}
