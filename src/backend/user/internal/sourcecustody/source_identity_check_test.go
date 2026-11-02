package sourcecustody

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"voice/backend/user/internal/identityassociation"
	"voice/backend/user/internal/testsupport/associationfixture"
)

func quarantinedIdentityFixture(t *testing.T) (context.Context, *pgxpool.Pool, *pgxpool.Pool, CreationRequest, PhysicalSource, Binding, *identityassociation.Keyset) {
	t.Helper()
	ctx, control, admin, request := creationFixture(t)
	t.Setenv("USER_ASSOCIATION_TEST_DSN", admin.Config().ConnConfig.ConnString())
	db := associationfixture.New(t, 1)
	receipt, err := associationfixture.ReadInstallerReceipt(db.InstallerReceiptPath)
	require.NoError(t, err)
	keys, err := identityassociation.NewKeyset(identityassociation.KeysetSnapshot{Revision: 1, ActiveVersion: receipt.KeyVersion, Versions: map[uint32][32]byte{receipt.KeyVersion: receipt.KeyFingerprint}}, map[uint32][]byte{receipt.KeyVersion: receipt.UserKey})
	require.NoError(t, err)
	identities, err := identityassociation.NewFreshStore(ctx, db.Pool, keys, identityassociation.FreshSourcePin{HistoryGeneration: receipt.HistoryGeneration, UserSourceGeneration: receipt.UserSourceGeneration})
	require.NoError(t, err)
	require.NoError(t, identities.ActivateGuards(ctx))
	request.Database = db.Pool.Config().ConnConfig.Database
	physical := PhysicalSource{SystemIdentifier: request.SourceSystemID, Database: request.Database}
	require.NoError(t, admin.QueryRow(ctx, `SELECT oid FROM pg_database WHERE datname=$1`, request.Database).Scan(&physical.DatabaseOID))
	digest, err := keys.CustodyDigest()
	require.NoError(t, err)
	binding := Binding{Epoch: request.Epoch, SourceID: request.SourceID, HistoryGeneration: receipt.HistoryGeneration, UserSourceGeneration: receipt.UserSourceGeneration, KeysetDigest: digest}
	require.NoError(t, QuarantineExistingSource(ctx, control, admin, request, physical))
	_, err = admin.Exec(ctx, `REVOKE CONNECT ON DATABASE `+pgx.Identifier{request.Database}.Sanitize()+` FROM PUBLIC`)
	require.NoError(t, err)
	return ctx, control, admin, request, physical, binding, keys
}

func TestCheckQuarantinedIdentityKeepsSourceClosed(t *testing.T) {
	ctx, control, admin, request, physical, binding, keys := quarantinedIdentityFixture(t)
	require.NoError(t, CheckQuarantinedIdentity(ctx, control, admin, request, physical, binding, keys))
	var phase string
	var allowed, stateExists bool
	require.NoError(t, control.QueryRow(ctx, `SELECT phase,to_regclass('public.user_custody_state') IS NOT NULL FROM user_custody_creation WHERE singleton`).Scan(&phase, &stateExists))
	require.Equal(t, "IDENTITY_CHECKED_CLOSED", phase)
	require.False(t, stateExists, "identity checks alone cannot authorize capture")
	require.NoError(t, admin.QueryRow(ctx, `SELECT datallowconn FROM pg_database WHERE oid=$1`, physical.DatabaseOID).Scan(&allowed))
	require.False(t, allowed)
}

func TestMaterializeCheckedSourcePublishesClosedOnly(t *testing.T) {
	ctx, control, admin, request, physical, binding, keys := quarantinedIdentityFixture(t)
	require.Error(t, MaterializeCheckedSource(ctx, control, admin, request, physical, binding), "quarantine alone cannot publish the state")
	require.NoError(t, CheckQuarantinedIdentity(ctx, control, admin, request, physical, binding, keys))
	wrong := binding
	wrong.HistoryGeneration = uuid.New()
	require.Error(t, MaterializeCheckedSource(ctx, control, admin, request, physical, wrong))
	_, err := admin.Exec(ctx, `ALTER DATABASE `+pgx.Identifier{physical.Database}.Sanitize()+` WITH ALLOW_CONNECTIONS true`)
	require.NoError(t, err)
	require.Error(t, MaterializeCheckedSource(ctx, control, admin, request, physical, binding), "an accessible source cannot be materialized")
	_, err = admin.Exec(ctx, `ALTER DATABASE `+pgx.Identifier{physical.Database}.Sanitize()+` WITH ALLOW_CONNECTIONS false`)
	require.NoError(t, err)
	require.NoError(t, MaterializeCheckedSource(ctx, control, admin, request, physical, binding))
	var phase, state string
	require.NoError(t, control.QueryRow(ctx, `SELECT phase FROM user_custody_creation WHERE singleton`).Scan(&phase))
	require.NoError(t, control.QueryRow(ctx, `SELECT state FROM user_custody_state WHERE singleton`).Scan(&state))
	require.Equal(t, "EXISTING_INITIALIZED_CLOSED", phase)
	require.Equal(t, "CLOSED", state)
	controller, err := Start(ctx, control, binding)
	require.NoError(t, err)
	_, _, err = controller.AcquireAttested(ctx, binding)
	require.Error(t, err, "a checked source still has no signed opening")
	require.NoError(t, controller.Shutdown(ctx))
	require.Error(t, MaterializeCheckedSource(ctx, control, admin, request, physical, binding), "an existing state cannot be relabelled")
}

func TestCheckedExistingSourceCanOpenOnlyWithFreshSignedDecision(t *testing.T) {
	ctx, control, admin, request, physical, binding, keys := quarantinedIdentityFixture(t)
	_, err := admin.Exec(ctx, `CREATE ROLE custody_app NOLOGIN PASSWORD 'fixture_only'; GRANT CONNECT ON DATABASE `+pgx.Identifier{physical.Database}.Sanitize()+` TO custody_app`)
	require.NoError(t, err)
	isolateWriterDatabases(t, admin, physical.Database)
	_, err = admin.Exec(ctx, `ALTER DATABASE `+pgx.Identifier{physical.Database}.Sanitize()+` WITH ALLOW_CONNECTIONS true`)
	require.NoError(t, err)
	config := admin.Config().Copy()
	config.ConnConfig.Database = physical.Database
	config.MaxConns = 1
	source, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	_, err = source.Exec(ctx, `GRANT USAGE ON SCHEMA public TO custody_app;
GRANT SELECT,INSERT,UPDATE,DELETE ON ALL TABLES IN SCHEMA public TO custody_app;
GRANT USAGE,SELECT,UPDATE ON ALL SEQUENCES IN SCHEMA public TO custody_app;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO custody_app;
GRANT EXECUTE ON FUNCTION pg_catalog.pg_control_system() TO custody_app`)
	require.NoError(t, err)
	var profile, account uuid.UUID
	require.NoError(t, source.QueryRow(ctx, `SELECT id,account_id FROM profiles LIMIT 1`).Scan(&profile, &account))
	source.Close()
	_, err = admin.Exec(ctx, `ALTER DATABASE `+pgx.Identifier{physical.Database}.Sanitize()+` WITH ALLOW_CONNECTIONS false`)
	require.NoError(t, err)
	require.NoError(t, CheckQuarantinedIdentity(ctx, control, admin, request, physical, binding, keys))
	require.NoError(t, MaterializeCheckedSource(ctx, control, admin, request, physical, binding))
	source, err = pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	defer source.Close()
	controller, err := Start(ctx, control, binding)
	require.NoError(t, err)
	defer func() { _ = controller.Shutdown(ctx) }()
	challenge, err := controller.ActivationChallenge()
	require.NoError(t, err)
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	now := time.Now()
	claim := OpenAttestation{Challenge: challenge, ControlSystem: controller.controlSystem, ControlDatabase: controller.controlDatabase, ControlDatabaseOID: controller.controlDatabaseOID, Binding: binding, Physical: physical, WriterRole: "custody_app", SourceEvidenceDigest: [32]byte{3}, WriterManifestDigest: [32]byte{1}, AuthEvidenceDigest: [32]byte{2}, IssuedAtUnix: now.Unix(), ExpiresAtUnix: now.Add(time.Minute).Unix()}
	claim, err = SignOpenAttestation(claim, private)
	require.NoError(t, err)
	require.NoError(t, controller.PrepareOpenSource(ctx, admin, claim, public))
	require.NoError(t, controller.OpenQualified(ctx, admin, source, keys, claim, public))
	writerConfig := config.Copy()
	writerConfig.ConnConfig.User = "custody_app"
	writerConfig.ConnConfig.Password = "fixture_only"
	writer, err := pgxpool.NewWithConfig(ctx, writerConfig)
	require.NoError(t, err)
	defer writer.Close()
	identity, err := identityassociation.NewStore(writer, keys, binding.HistoryGeneration)
	require.NoError(t, err)
	result, err := identity.CaptureProfilesForPhysicalSource(ctx, []uuid.UUID{profile}, binding.HistoryGeneration, binding.UserSourceGeneration, binding.KeysetDigest, identityassociation.PhysicalSourceIdentity{SystemIdentifier: physical.SystemIdentifier, Database: physical.Database, DatabaseOID: physical.DatabaseOID})
	require.NoError(t, err)
	require.Equal(t, account, result.Observations[profile].AccountID)
}

func TestCheckQuarantinedIdentityRejectsForeignLineage(t *testing.T) {
	ctx, control, admin, request, physical, binding, keys := quarantinedIdentityFixture(t)
	binding.UserSourceGeneration = uuid.New()
	require.Error(t, CheckQuarantinedIdentity(ctx, control, admin, request, physical, binding, keys))
	var allowed, stateExists bool
	var phase string
	require.NoError(t, control.QueryRow(ctx, `SELECT phase,to_regclass('public.user_custody_state') IS NOT NULL FROM user_custody_creation WHERE singleton`).Scan(&phase, &stateExists))
	require.Equal(t, "VERIFYING_IDENTITY", phase)
	require.False(t, stateExists)
	require.NoError(t, admin.QueryRow(ctx, `SELECT datallowconn FROM pg_database WHERE oid=$1`, physical.DatabaseOID).Scan(&allowed))
	require.False(t, allowed)
}

func TestCheckQuarantinedIdentityRejectsPublicAccess(t *testing.T) {
	ctx, control, admin, request, physical, binding, keys := quarantinedIdentityFixture(t)
	_, err := admin.Exec(ctx, `GRANT CONNECT ON DATABASE `+pgx.Identifier{request.Database}.Sanitize()+` TO PUBLIC`)
	require.NoError(t, err)
	require.Error(t, CheckQuarantinedIdentity(ctx, control, admin, request, physical, binding, keys))
	var phase string
	var allowed bool
	require.NoError(t, control.QueryRow(ctx, `SELECT phase FROM user_custody_creation WHERE singleton`).Scan(&phase))
	require.Equal(t, "QUARANTINED_EXISTING", phase)
	require.NoError(t, admin.QueryRow(ctx, `SELECT datallowconn FROM pg_database WHERE oid=$1`, physical.DatabaseOID).Scan(&allowed))
	require.False(t, allowed)
}

func TestCheckQuarantinedIdentityRejectsForeignKeyDigest(t *testing.T) {
	ctx, control, admin, request, physical, binding, keys := quarantinedIdentityFixture(t)
	binding.KeysetDigest[0] ^= 1
	require.Error(t, CheckQuarantinedIdentity(ctx, control, admin, request, physical, binding, keys))
	var phase string
	var allowed bool
	require.NoError(t, control.QueryRow(ctx, `SELECT phase FROM user_custody_creation WHERE singleton`).Scan(&phase))
	require.Equal(t, "QUARANTINED_EXISTING", phase)
	require.NoError(t, admin.QueryRow(ctx, `SELECT datallowconn FROM pg_database WHERE oid=$1`, physical.DatabaseOID).Scan(&allowed))
	require.False(t, allowed)
}

func TestCheckQuarantinedIdentityRejectsSplitControlPool(t *testing.T) {
	ctx, control, admin, request, physical, binding, keys := quarantinedIdentityFixture(t)
	_, err := control.Exec(ctx, `CREATE DATABASE alternate_identity_control`)
	require.NoError(t, err)
	otherConfig := control.Config().Copy()
	otherConfig.ConnConfig.Database = "alternate_identity_control"
	other, err := pgxpool.NewWithConfig(ctx, otherConfig)
	require.NoError(t, err)
	defer other.Close()
	_, err = other.Exec(ctx, `CREATE TABLE user_custody_physical_source(source_id text NOT NULL,epoch bigint NOT NULL,system_identifier text NOT NULL,database_name text NOT NULL,database_oid oid NOT NULL,PRIMARY KEY(source_id,epoch))`)
	require.NoError(t, err)
	_, err = other.Exec(ctx, `INSERT INTO user_custody_physical_source VALUES($1,$2,$3,$4,$5)`, request.SourceID, request.Epoch, physical.SystemIdentifier, physical.Database, physical.DatabaseOID)
	require.NoError(t, err)
	config := control.Config().Copy()
	var connects atomic.Int32
	config.BeforeConnect = func(_ context.Context, conn *pgx.ConnConfig) error {
		if connects.Add(1) > 1 {
			*conn = *other.Config().ConnConfig.Copy()
		}
		return nil
	}
	mixed, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	defer mixed.Close()
	require.Error(t, CheckQuarantinedIdentity(ctx, mixed, admin, request, physical, binding, keys))
	var phase string
	var allowed bool
	require.NoError(t, control.QueryRow(ctx, `SELECT phase FROM user_custody_creation WHERE singleton`).Scan(&phase))
	require.Equal(t, "QUARANTINED_EXISTING", phase)
	require.NoError(t, admin.QueryRow(ctx, `SELECT datallowconn FROM pg_database WHERE oid=$1`, physical.DatabaseOID).Scan(&allowed))
	require.False(t, allowed)
}

func TestInitializeClosedSourceRejectsSplitControlPool(t *testing.T) {
	ctx, control, admin, request := creationFixture(t)
	physical, err := CreateClosedSource(ctx, control, admin, request)
	require.NoError(t, err)
	dir, keys := migrationFixture(t)
	_, err = control.Exec(ctx, `CREATE DATABASE alternate_init_control`)
	require.NoError(t, err)
	otherConfig := control.Config().Copy()
	otherConfig.ConnConfig.Database = "alternate_init_control"
	other, err := pgxpool.NewWithConfig(ctx, otherConfig)
	require.NoError(t, err)
	defer other.Close()
	_, err = other.Exec(ctx, `CREATE TABLE user_custody_physical_source(source_id text NOT NULL,epoch bigint NOT NULL,system_identifier text NOT NULL,database_name text NOT NULL,database_oid oid NOT NULL,PRIMARY KEY(source_id,epoch))`)
	require.NoError(t, err)
	_, err = other.Exec(ctx, `INSERT INTO user_custody_physical_source VALUES($1,$2,$3,$4,$5)`, request.SourceID, request.Epoch, physical.SystemIdentifier, physical.Database, physical.DatabaseOID)
	require.NoError(t, err)
	config := control.Config().Copy()
	var connects atomic.Int32
	config.BeforeConnect = func(_ context.Context, conn *pgx.ConnConfig) error {
		if connects.Add(1) > 1 {
			*conn = *other.Config().ConnConfig.Copy()
		}
		return nil
	}
	mixed, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	defer mixed.Close()
	require.Error(t, InitializeClosedSource(ctx, mixed, admin, request, physical, dir, keys))
	var phase string
	var allowed bool
	require.NoError(t, control.QueryRow(ctx, `SELECT phase FROM user_custody_creation WHERE singleton`).Scan(&phase))
	require.Equal(t, "CREATED_CLOSED", phase)
	require.NoError(t, admin.QueryRow(ctx, `SELECT datallowconn FROM pg_database WHERE oid=$1`, physical.DatabaseOID).Scan(&allowed))
	require.False(t, allowed)
}
