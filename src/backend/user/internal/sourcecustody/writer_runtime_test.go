package sourcecustody

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"voice/backend/pkg/integrationtest"
	"voice/backend/user/internal/identityassociation"
)

func TestWriterRuntimeRequiresRoleAndMatchingPins(t *testing.T) {
	values := map[string]string{
		"CUSTODY_CONTROL_DATABASE_URL":      "postgres://owner:password@control.example:5432/control?sslmode=verify-full&sslrootcert=ca.pem",
		"CUSTODY_SOURCE_ADMIN_DATABASE_URL": "postgres://owner:password@source.example:5432/postgres?sslmode=verify-full&sslrootcert=ca.pem",
		"CUSTODY_CONTROL_SYSTEM_ID":         "123", "CUSTODY_SOURCE_SYSTEM_ID": "456", "CUSTODY_SOURCE_DATABASE": "voice_source_test",
		"CUSTODY_SOURCE_DATABASE_OID": "42", "CUSTODY_SOURCE_ID": "c9a0874e-f626-4da9-8266-66de9f4e6fcb", "CUSTODY_EPOCH": "1",
		"CUSTODY_WRITER_ROLE": "voice_user",
	}
	lookup := func(key string) (string, bool) { v, ok := values[key]; return v, ok }
	cfg, err := LoadWriterConfig(lookup)
	require.NoError(t, err)
	require.Equal(t, "voice_user", cfg.WriterRole)
	delete(values, "CUSTODY_WRITER_ROLE")
	_, err = LoadWriterConfig(lookup)
	require.Error(t, err)
	values["CUSTODY_WRITER_ROLE"] = "Bad Role"
	_, err = LoadWriterConfig(lookup)
	require.Error(t, err)
	wrong := cfg
	wrong.Recovery.Physical.Database = "other_database"
	require.Error(t, RunWriterConfinement(context.Background(), wrong))
}

func TestStartupRecoveryFencesActiveWriterOverTLS(t *testing.T) {
	if testing.Short() {
		t.Skip("requires two private TLS PostgreSQL servers")
	}
	ctx := context.Background()
	integrationtest.ConfigureDockerTesting()
	_, control := recoveryTLSDatabase(t, ctx, "startup-recovery-control")
	adminURL, admin := recoveryTLSDatabase(t, ctx, "startup-recovery-source")
	request := CreationRequest{SourceID: uuid.NewString(), Epoch: 1, Database: "voice_source_test"}
	require.NoError(t, control.QueryRow(ctx, `SELECT system_identifier::text FROM pg_control_system()`).Scan(&request.ControlSystemID))
	require.NoError(t, admin.QueryRow(ctx, `SELECT system_identifier::text FROM pg_control_system()`).Scan(&request.SourceSystemID))
	physical, err := CreateClosedSource(ctx, control, admin, request)
	require.NoError(t, err)
	dir, keys := migrationFixture(t)
	require.NoError(t, InitializeClosedSource(ctx, control, admin, request, physical, dir, keys))
	_, err = admin.Exec(ctx, `CREATE ROLE custody_app NOLOGIN PASSWORD 'fixture_only'; GRANT CONNECT ON DATABASE `+pgx.Identifier{physical.Database}.Sanitize()+` TO custody_app`)
	require.NoError(t, err)
	isolateWriterDatabases(t, admin, physical.Database)
	_, err = admin.Exec(ctx, `ALTER DATABASE `+pgx.Identifier{physical.Database}.Sanitize()+` WITH ALLOW_CONNECTIONS true`)
	require.NoError(t, err)
	u, err := url.Parse(adminURL)
	require.NoError(t, err)
	u.Path = "/" + physical.Database
	source, err := openPinnedPostgres(ctx, u.String())
	require.NoError(t, err)
	_, err = source.Exec(ctx, `GRANT USAGE ON SCHEMA public TO custody_app;
GRANT SELECT,INSERT,UPDATE,DELETE ON ALL TABLES IN SCHEMA public TO custody_app;
GRANT USAGE,SELECT,UPDATE ON ALL SEQUENCES IN SCHEMA public TO custody_app;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO custody_app;
GRANT EXECUTE ON FUNCTION pg_catalog.pg_control_system() TO custody_app`)
	require.NoError(t, err)
	var binding Binding
	var digest []byte
	require.NoError(t, control.QueryRow(ctx, `SELECT epoch,source_id,history_generation,user_source_generation,keyset_digest FROM user_custody_state WHERE singleton`).Scan(&binding.Epoch, &binding.SourceID, &binding.HistoryGeneration, &binding.UserSourceGeneration, &digest))
	copy(binding.KeysetDigest[:], digest)
	identities, err := identityassociation.NewStore(source, keys, binding.HistoryGeneration)
	require.NoError(t, err)
	require.NoError(t, identities.ActivateGuards(ctx))
	source.Close()
	_, err = admin.Exec(ctx, `ALTER DATABASE `+pgx.Identifier{physical.Database}.Sanitize()+` WITH ALLOW_CONNECTIONS false`)
	require.NoError(t, err)
	source, err = openPinnedPostgres(ctx, u.String())
	require.NoError(t, err)
	defer source.Close()
	controller, err := Start(ctx, control, binding)
	require.NoError(t, err)
	challenge, err := controller.ActivationChallenge()
	require.NoError(t, err)
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	now := time.Now()
	claim := OpenAttestation{Challenge: challenge, ControlSystem: controller.controlSystem, ControlDatabase: controller.controlDatabase, ControlDatabaseOID: controller.controlDatabaseOID, Binding: binding, Physical: physical, WriterRole: "custody_app", SourceEvidenceDigest: [32]byte{3}, WriterManifestDigest: [32]byte{1}, AuthEvidenceDigest: [32]byte{2}, IssuedAtUnix: now.Unix(), ExpiresAtUnix: now.Add(4 * time.Minute).Unix()}
	claim, err = SignOpenAttestation(claim, private)
	require.NoError(t, err)
	require.NoError(t, controller.PrepareOpenSource(ctx, admin, claim, public))
	require.NoError(t, controller.OpenQualified(ctx, admin, source, keys, claim, public))
	writerConfig, err := pgxpool.ParseConfig(u.String())
	require.NoError(t, err)
	writerConfig.ConnConfig.User = "custody_app"
	writerConfig.ConnConfig.Password = "fixture_only"
	writer, err := pgxpool.NewWithConfig(ctx, writerConfig)
	require.NoError(t, err)
	defer writer.Close()
	require.NoError(t, writer.Ping(ctx))
	require.NoError(t, controller.Shutdown(ctx))
	require.NoError(t, recoverBeforeStart(ctx, control, &ActivationOperatorConfig{AdminURL: adminURL}, binding))
	var phase string
	var allowed, login bool
	require.NoError(t, control.QueryRow(ctx, `SELECT phase FROM user_custody_creation WHERE singleton`).Scan(&phase))
	require.Equal(t, "RECOVERED_CLOSED", phase)
	require.NoError(t, admin.QueryRow(ctx, `SELECT datallowconn FROM pg_database WHERE oid=$1`, physical.DatabaseOID).Scan(&allowed))
	require.False(t, allowed)
	require.NoError(t, admin.QueryRow(ctx, `SELECT rolcanlogin FROM pg_roles WHERE rolname='custody_app'`).Scan(&login))
	require.False(t, login)
	require.Eventually(t, func() bool {
		bounded, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		return writer.Ping(bounded) != nil
	}, 5*time.Second, 50*time.Millisecond)
	restarted, err := Start(ctx, control, binding)
	require.NoError(t, err)
	require.NoError(t, restarted.Shutdown(ctx))
}

func TestRecoverActivationCommandFencesPreparedSourceOverTLS(t *testing.T) {
	if testing.Short() {
		t.Skip("requires two private TLS PostgreSQL servers")
	}
	ctx := context.Background()
	integrationtest.ConfigureDockerTesting()
	controlURL, control := recoveryTLSDatabase(t, ctx, "activation-recovery-control")
	adminURL, admin := recoveryTLSDatabase(t, ctx, "activation-recovery-source")
	request := CreationRequest{SourceID: uuid.NewString(), Epoch: 1, Database: "voice_source_test"}
	require.NoError(t, control.QueryRow(ctx, `SELECT system_identifier::text FROM pg_control_system()`).Scan(&request.ControlSystemID))
	require.NoError(t, admin.QueryRow(ctx, `SELECT system_identifier::text FROM pg_control_system()`).Scan(&request.SourceSystemID))
	physical, err := CreateClosedSource(ctx, control, admin, request)
	require.NoError(t, err)
	dir, keys := migrationFixture(t)
	require.NoError(t, InitializeClosedSource(ctx, control, admin, request, physical, dir, keys))
	_, err = admin.Exec(ctx, `CREATE ROLE custody_app NOLOGIN; GRANT CONNECT ON DATABASE `+pgx.Identifier{physical.Database}.Sanitize()+` TO custody_app`)
	require.NoError(t, err)
	isolateWriterDatabases(t, admin, physical.Database)
	var binding Binding
	var digest []byte
	require.NoError(t, control.QueryRow(ctx, `SELECT epoch,source_id,history_generation,user_source_generation,keyset_digest FROM user_custody_state WHERE singleton`).Scan(&binding.Epoch, &binding.SourceID, &binding.HistoryGeneration, &binding.UserSourceGeneration, &digest))
	copy(binding.KeysetDigest[:], digest)
	controller, err := Start(ctx, control, binding)
	require.NoError(t, err)
	challenge, err := controller.ActivationChallenge()
	require.NoError(t, err)
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	now := time.Now()
	claim := OpenAttestation{Challenge: challenge, ControlSystem: controller.controlSystem, ControlDatabase: controller.controlDatabase, ControlDatabaseOID: controller.controlDatabaseOID, Binding: binding, Physical: physical, WriterRole: "custody_app", SourceEvidenceDigest: [32]byte{3}, WriterManifestDigest: [32]byte{1}, AuthEvidenceDigest: [32]byte{2}, IssuedAtUnix: now.Unix(), ExpiresAtUnix: now.Add(4 * time.Minute).Unix()}
	payload, err := OpenAttestationPayload(claim)
	require.NoError(t, err)
	claim.Signature = ed25519.Sign(private, payload)
	require.NoError(t, controller.PrepareOpenSource(ctx, admin, claim, public))
	startupConfig := &ActivationOperatorConfig{AdminURL: adminURL}
	require.Error(t, recoverBeforeStart(ctx, control, startupConfig, binding), "a live leader must not be fenced by a competing startup")
	require.NoError(t, controller.Shutdown(ctx))
	cfg := WriterConfig{Recovery: RecoveryConfig{ControlURL: controlURL, SourceAdminURL: adminURL, Request: request, Physical: physical}, WriterRole: claim.WriterRole}
	wrongCA, err := url.Parse(adminURL)
	require.NoError(t, err)
	controlParsed, err := url.Parse(controlURL)
	require.NoError(t, err)
	query := wrongCA.Query()
	query.Set("sslrootcert", controlParsed.Query().Get("sslrootcert"))
	wrongCA.RawQuery = query.Encode()
	wrong := cfg
	wrong.Recovery.SourceAdminURL = wrongCA.String()
	require.Error(t, RunActivationRecovery(ctx, wrong))
	var allowed bool
	require.NoError(t, admin.QueryRow(ctx, `SELECT datallowconn FROM pg_database WHERE oid=$1`, physical.DatabaseOID).Scan(&allowed))
	require.True(t, allowed)
	badStartup := &ActivationOperatorConfig{AdminURL: wrongCA.String()}
	require.Error(t, recoverBeforeStart(ctx, control, badStartup, binding))
	require.NoError(t, recoverBeforeStart(ctx, control, startupConfig, binding))
	require.NoError(t, admin.QueryRow(ctx, `SELECT datallowconn FROM pg_database WHERE oid=$1`, physical.DatabaseOID).Scan(&allowed))
	require.False(t, allowed)
	binary := filepath.Join(t.TempDir(), "sourcecustody.exe")
	build := exec.CommandContext(ctx, "go", "build", "-o", binary, "../../cmd/sourcecustody")
	output, err := build.CombinedOutput()
	require.NoError(t, err, string(output))
	command := exec.CommandContext(ctx, binary, "recover-activation")
	for _, item := range os.Environ() {
		if !strings.HasPrefix(strings.ToUpper(item), "PG") && !strings.HasPrefix(item, "CUSTODY_") {
			command.Env = append(command.Env, item)
		}
	}
	command.Env = append(command.Env, "CUSTODY_CONTROL_DATABASE_URL="+controlURL, "CUSTODY_SOURCE_ADMIN_DATABASE_URL="+adminURL,
		"CUSTODY_CONTROL_SYSTEM_ID="+request.ControlSystemID, "CUSTODY_SOURCE_SYSTEM_ID="+request.SourceSystemID,
		"CUSTODY_SOURCE_DATABASE="+request.Database, "CUSTODY_SOURCE_DATABASE_OID="+strconv.FormatUint(uint64(physical.DatabaseOID), 10),
		"CUSTODY_SOURCE_ID="+request.SourceID, "CUSTODY_EPOCH="+strconv.FormatInt(request.Epoch, 10), "CUSTODY_WRITER_ROLE="+claim.WriterRole)
	output, err = command.CombinedOutput()
	require.NoError(t, err, string(output))
	require.NoError(t, admin.QueryRow(ctx, `SELECT datallowconn FROM pg_database WHERE oid=$1`, physical.DatabaseOID).Scan(&allowed))
	require.False(t, allowed)
	var phase, state string
	require.NoError(t, control.QueryRow(ctx, `SELECT phase FROM user_custody_creation WHERE singleton`).Scan(&phase))
	require.Equal(t, "RECOVERED_CLOSED", phase)
	require.NoError(t, control.QueryRow(ctx, `SELECT state FROM user_custody_state WHERE singleton`).Scan(&state))
	require.Equal(t, "CLOSED", state)
}

func TestWriterCommandInspectsPinnedSourceOverTLS(t *testing.T) {
	if testing.Short() {
		t.Skip("requires two private TLS PostgreSQL servers")
	}
	ctx := context.Background()
	integrationtest.ConfigureDockerTesting()
	controlURL, control := recoveryTLSDatabase(t, ctx, "writer-control")
	adminURL, admin := recoveryTLSDatabase(t, ctx, "writer-source")
	request := CreationRequest{SourceID: uuid.NewString(), Epoch: 1, Database: "voice_source_test"}
	require.NoError(t, control.QueryRow(ctx, `SELECT system_identifier::text FROM pg_control_system()`).Scan(&request.ControlSystemID))
	require.NoError(t, admin.QueryRow(ctx, `SELECT system_identifier::text FROM pg_control_system()`).Scan(&request.SourceSystemID))
	physical, err := CreateClosedSource(ctx, control, admin, request)
	require.NoError(t, err)
	dir, keys := migrationFixture(t)
	require.NoError(t, InitializeClosedSource(ctx, control, admin, request, physical, dir, keys))
	_, err = admin.Exec(ctx, `CREATE ROLE custody_app LOGIN PASSWORD 'fixture_only'`)
	require.NoError(t, err)
	_, err = admin.Exec(ctx, `GRANT CONNECT ON DATABASE `+pgx.Identifier{physical.Database}.Sanitize()+` TO custody_app`)
	require.NoError(t, err)
	_, err = admin.Exec(ctx, `ALTER DATABASE `+pgx.Identifier{physical.Database}.Sanitize()+` WITH ALLOW_CONNECTIONS true`)
	require.NoError(t, err)
	config := admin.Config().Copy()
	config.ConnConfig.Database = physical.Database
	source, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	defer source.Close()
	_, err = source.Exec(ctx, `GRANT USAGE ON SCHEMA public TO custody_app;
GRANT SELECT,INSERT,UPDATE,DELETE ON ALL TABLES IN SCHEMA public TO custody_app;
GRANT USAGE,SELECT,UPDATE ON ALL SEQUENCES IN SCHEMA public TO custody_app;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO custody_app;
GRANT EXECUTE ON FUNCTION pg_catalog.pg_control_system() TO custody_app`)
	require.NoError(t, err)
	isolateWriterDatabases(t, admin, physical.Database)
	cfg := WriterConfig{Recovery: RecoveryConfig{ControlURL: controlURL, SourceAdminURL: adminURL, Request: request, Physical: physical}, WriterRole: "custody_app"}
	wrong := cfg
	wrong.Recovery.Physical.DatabaseOID++
	require.Error(t, RunWriterConfinement(ctx, wrong))
	wrongCA, err := url.Parse(adminURL)
	require.NoError(t, err)
	controlParsed, err := url.Parse(controlURL)
	require.NoError(t, err)
	query := wrongCA.Query()
	query.Set("sslrootcert", controlParsed.Query().Get("sslrootcert"))
	wrongCA.RawQuery = query.Encode()
	wrong = cfg
	wrong.Recovery.SourceAdminURL = wrongCA.String()
	require.Error(t, RunWriterConfinement(ctx, wrong))
	_, err = admin.Exec(ctx, `ALTER ROLE custody_app SUPERUSER`)
	require.NoError(t, err)
	require.Error(t, RunWriterConfinement(ctx, cfg))
	_, err = admin.Exec(ctx, `ALTER ROLE custody_app NOSUPERUSER`)
	require.NoError(t, err)
	require.NoError(t, RunWriterConfinement(ctx, cfg))
	binary := filepath.Join(t.TempDir(), "sourcecustody.exe")
	build := exec.CommandContext(ctx, "go", "build", "-o", binary, "../../cmd/sourcecustody")
	output, err := build.CombinedOutput()
	require.NoError(t, err, string(output))
	command := exec.CommandContext(ctx, binary, "inspect-writer-role")
	for _, item := range os.Environ() {
		if !strings.HasPrefix(strings.ToUpper(item), "PG") && !strings.HasPrefix(item, "CUSTODY_") {
			command.Env = append(command.Env, item)
		}
	}
	command.Env = append(command.Env, "CUSTODY_CONTROL_DATABASE_URL="+controlURL, "CUSTODY_SOURCE_ADMIN_DATABASE_URL="+adminURL,
		"CUSTODY_CONTROL_SYSTEM_ID="+request.ControlSystemID, "CUSTODY_SOURCE_SYSTEM_ID="+request.SourceSystemID,
		"CUSTODY_SOURCE_DATABASE="+request.Database, "CUSTODY_SOURCE_DATABASE_OID="+strconv.FormatUint(uint64(physical.DatabaseOID), 10),
		"CUSTODY_SOURCE_ID="+request.SourceID, "CUSTODY_EPOCH="+strconv.FormatInt(request.Epoch, 10), "CUSTODY_WRITER_ROLE=custody_app")
	output, err = command.CombinedOutput()
	require.NoError(t, err, string(output))
	var state string
	require.NoError(t, control.QueryRow(ctx, `SELECT state FROM user_custody_state WHERE singleton`).Scan(&state))
	require.Equal(t, "CLOSED", state, "inspection must not open capture")
}
