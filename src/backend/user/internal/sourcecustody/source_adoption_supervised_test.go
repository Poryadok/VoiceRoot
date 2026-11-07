package sourcecustody

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"voice/backend/pkg/integrationtest"
	"voice/backend/user/internal/identityassociation"
)

// The custodian's operational evidence and operator request are fixtures. The
// supervisor is the real executable, using mTLS, two actual TLS PostgreSQL
// servers and the signed current decision. This proves assembly, not deployed
// backup/restore custody, complete writer inventory, Auth continuity or Ban.
func TestAdoptedSourceSupervisedGuardCutoverCapturesRetainedProfileOverTLS(t *testing.T) {
	if testing.Short() {
		t.Skip("requires two owned TLS PostgreSQL servers and production supervisor")
	}
	ctx := context.Background()
	integrationtest.ConfigureDockerTesting()
	controlURL, control := recoveryTLSDatabase(t, ctx, "adopt-open-control")
	adminURL, admin := recoveryTLSDatabase(t, ctx, "adopt-open-source")
	request := CreationRequest{SourceID: uuid.NewString(), Epoch: 1, Database: "voice_adopt_open"}
	require.NoError(t, control.QueryRow(ctx, `SELECT system_identifier::text FROM pg_control_system()`).Scan(&request.ControlSystemID))
	require.NoError(t, admin.QueryRow(ctx, `SELECT system_identifier::text FROM pg_control_system()`).Scan(&request.SourceSystemID))
	physical, profile, _ := prepareExistingMigration(t, ctx, control, admin, request, 22)
	_, keys := migrationFixture(t)
	pool := adoptionProbe(t, ctx, admin, physical, true)
	var account uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT account_id FROM profiles WHERE id=$1`, profile).Scan(&account))
	binding := Binding{SourceID: request.SourceID, Epoch: request.Epoch}
	require.NoError(t, pool.QueryRow(ctx, `SELECT history_generation,user_source_generation FROM identity_meta WHERE singleton`).Scan(&binding.HistoryGeneration, &binding.UserSourceGeneration))
	var err error
	binding.KeysetDigest, err = keys.CustodyDigest()
	require.NoError(t, err)
	_, err = admin.Exec(ctx, `CREATE ROLE custody_app NOLOGIN PASSWORD 'fixture_only'; GRANT CONNECT ON DATABASE `+pgx.Identifier{physical.Database}.Sanitize()+` TO custody_app`)
	require.NoError(t, err)
	isolateWriterDatabases(t, admin, physical.Database)
	_, err = pool.Exec(ctx, `GRANT USAGE ON SCHEMA public TO custody_app;
GRANT SELECT,INSERT,UPDATE,DELETE ON ALL TABLES IN SCHEMA public TO custody_app;
GRANT USAGE,SELECT,UPDATE ON ALL SEQUENCES IN SCHEMA public TO custody_app;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO custody_app;
GRANT EXECUTE ON FUNCTION pg_catalog.pg_control_system() TO custody_app`)
	require.NoError(t, err)
	pool.Close()
	_, err = admin.Exec(ctx, `ALTER DATABASE `+pgx.Identifier{physical.Database}.Sanitize()+` WITH ALLOW_CONNECTIONS false`)
	require.NoError(t, err)
	require.NoError(t, AdoptPristineExistingSource(ctx, control, admin, request, physical, binding, keys, uuid.New(), [32]byte{1}))
	controller, err := Start(ctx, control, binding)
	require.NoError(t, err)
	t.Cleanup(func() { _ = controller.Shutdown(context.Background()) })
	_, _, err = controller.AcquireAttested(ctx, binding)
	require.Error(t, err, "adoption alone must not authorize capture")
	config := admin.Config().Copy()
	config.ConnConfig.Database = physical.Database
	config.MaxConns, config.MinConns = 1, 0
	source, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	t.Cleanup(source.Close)
	challenge, err := controller.ActivationChallenge()
	require.NoError(t, err)
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	now := time.Now()
	claim := OpenAttestation{Challenge: challenge, ControlSystem: controller.controlSystem, ControlDatabase: controller.controlDatabase, ControlDatabaseOID: controller.controlDatabaseOID, Binding: binding, Physical: physical, WriterRole: "custody_app", SourceEvidenceDigest: [32]byte{3}, WriterManifestDigest: [32]byte{1}, AuthEvidenceDigest: [32]byte{2}, GuardCutover: true, IssuedAtUnix: now.Unix(), ExpiresAtUnix: now.Add(4 * time.Minute).Unix()}
	claim, err = SignOpenAttestation(claim, private)
	require.NoError(t, err)
	binary := buildProductionSupervisor(t)
	client, _, _ := startProductionSupervisor(t, binary, controlURL, adminURL, claim, public)
	h := &activationHandler{controller: controller, admin: admin, source: source, keys: keys, signer: public, supervisor: client}
	t.Cleanup(func() {
		if h.supervised != nil {
			require.NoError(t, h.supervised.stop())
		}
	})
	require.Equal(t, http.StatusNoContent, openThroughOperator(t, h, claim))
	permit, boundPhysical, err := controller.AcquireAttested(ctx, binding)
	require.NoError(t, err)
	require.Equal(t, physical, boundPhysical)
	defer func() { require.NoError(t, permit.Release(ctx)) }()
	writerConfig := config.Copy()
	writerConfig.ConnConfig.User, writerConfig.ConnConfig.Password = "custody_app", "fixture_only"
	writer, err := pgxpool.NewWithConfig(ctx, writerConfig)
	require.NoError(t, err)
	defer writer.Close()
	store, err := identityassociation.NewStore(writer, keys, binding.HistoryGeneration)
	require.NoError(t, err)
	result, err := store.CaptureProfilesForPhysicalSource(ctx, []uuid.UUID{profile}, binding.HistoryGeneration, binding.UserSourceGeneration, binding.KeysetDigest, identityassociation.PhysicalSourceIdentity{SystemIdentifier: physical.SystemIdentifier, Database: physical.Database, DatabaseOID: physical.DatabaseOID})
	require.NoError(t, err)
	require.Equal(t, account, result.Observations[profile].AccountID)
	require.Equal(t, identityassociation.CaptureAssociated, result.Observations[profile].Status)
	require.Equal(t, identityassociation.OriginRetainedNow, result.Observations[profile].Fact.Origin)
	var guards bool
	var baseline uint64
	require.NoError(t, writer.QueryRow(ctx, `SELECT guards_active,baseline_cut FROM identity_meta WHERE singleton`).Scan(&guards, &baseline))
	require.True(t, guards)
	require.Positive(t, baseline)
	var receiptPhase, phase string
	require.NoError(t, control.QueryRow(ctx, `SELECT phase FROM user_custody_adoption WHERE singleton`).Scan(&receiptPhase))
	require.Equal(t, "ADOPTED_CLOSED", receiptPhase, "adoption receipt remains an historical closed result")
	require.NoError(t, control.QueryRow(ctx, `SELECT phase FROM user_custody_creation WHERE singleton`).Scan(&phase))
	require.Equal(t, "ACTIVE", phase)
}
