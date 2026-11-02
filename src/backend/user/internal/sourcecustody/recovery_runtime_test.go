package sourcecustody

import (
	"context"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"voice/backend/pkg/integrationtest"
)

func recoveryTLSDatabase(t *testing.T, ctx context.Context, label string) (string, *pgxpool.Pool) {
	t.Helper()
	dir := t.TempDir()
	pki := processTLS(t, dir, label)
	configFile := filepath.Join(dir, "postgres.conf")
	require.NoError(t, os.WriteFile(configFile, []byte("listen_addresses='*'\nssl=on\nssl_ca_file='/tmp/testcontainers-go/postgres/ca_cert.pem'\nssl_cert_file='/tmp/testcontainers-go/postgres/server.cert'\nssl_key_file='/tmp/testcontainers-go/postgres/server.key'\n"), 0600))
	pg, err := postgres.Run(ctx, "postgres:16-alpine", postgres.BasicWaitStrategies(), postgres.WithDatabase("custody_test"), postgres.WithUsername("fixture"), postgres.WithPassword("fixture_only"), postgres.WithConfigFile(configFile), postgres.WithSSLCert(pki.caFile, pki.serverCert, pki.serverKey))
	if pg != nil {
		t.Cleanup(func() {
			cleanup, stop := context.WithTimeout(context.Background(), 30*time.Second)
			defer stop()
			require.NoError(t, pg.Terminate(cleanup))
		})
	}
	require.NoError(t, err)
	raw, err := pg.ConnectionString(ctx)
	require.NoError(t, err)
	u, err := url.Parse(raw)
	require.NoError(t, err)
	_, port, err := net.SplitHostPort(u.Host)
	require.NoError(t, err)
	u.Host = net.JoinHostPort("127.0.0.1", port)
	q := u.Query()
	q.Set("sslmode", "verify-full")
	q.Set("sslrootcert", pki.caFile)
	u.RawQuery = q.Encode()
	pool, err := pgxpool.New(ctx, u.String())
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	require.NoError(t, pool.Ping(ctx))
	return u.String(), pool
}

func TestRunRecoveryUsesVerifiedTLSConnections(t *testing.T) {
	if testing.Short() {
		t.Skip("requires two private TLS PostgreSQL servers")
	}
	ctx := context.Background()
	integrationtest.ConfigureDockerTesting()
	controlURL, control := recoveryTLSDatabase(t, ctx, "control")
	adminURL, admin := recoveryTLSDatabase(t, ctx, "source")
	request := CreationRequest{SourceID: uuid.NewString(), Epoch: 1, Database: "voice_source_test"}
	require.NoError(t, control.QueryRow(ctx, `SELECT system_identifier::text FROM pg_control_system()`).Scan(&request.ControlSystemID))
	require.NoError(t, admin.QueryRow(ctx, `SELECT system_identifier::text FROM pg_control_system()`).Scan(&request.SourceSystemID))
	physical, err := CreateClosedSource(ctx, control, admin, request)
	require.NoError(t, err)
	_, err = control.Exec(ctx, `UPDATE user_custody_creation SET phase='MIGRATING' WHERE singleton`)
	require.NoError(t, err)
	_, err = admin.Exec(ctx, `ALTER DATABASE voice_source_test WITH ALLOW_CONNECTIONS true`)
	require.NoError(t, err)
	wrongCA, err := url.Parse(adminURL)
	require.NoError(t, err)
	controlParsed, err := url.Parse(controlURL)
	require.NoError(t, err)
	query := wrongCA.Query()
	query.Set("sslrootcert", controlParsed.Query().Get("sslrootcert"))
	wrongCA.RawQuery = query.Encode()
	require.Error(t, RunRecovery(ctx, RecoveryConfig{ControlURL: controlURL, SourceAdminURL: wrongCA.String(), Request: request, Physical: physical}))
	var allowed bool
	require.NoError(t, admin.QueryRow(ctx, `SELECT datallowconn FROM pg_database WHERE oid=$1`, physical.DatabaseOID).Scan(&allowed))
	require.True(t, allowed, "an untrusted TLS source must not be changed")
	binary := filepath.Join(t.TempDir(), "sourcecustody.exe")
	build := exec.CommandContext(ctx, "go", "build", "-o", binary, "../../cmd/sourcecustody")
	output, err := build.CombinedOutput()
	require.NoError(t, err, string(output))
	command := exec.CommandContext(ctx, binary, "recover-initialization")
	command.Env = append(os.Environ(), "CUSTODY_CONTROL_DATABASE_URL="+controlURL, "CUSTODY_SOURCE_ADMIN_DATABASE_URL="+adminURL,
		"CUSTODY_CONTROL_SYSTEM_ID="+request.ControlSystemID, "CUSTODY_SOURCE_SYSTEM_ID="+request.SourceSystemID,
		"CUSTODY_SOURCE_DATABASE="+request.Database, "CUSTODY_SOURCE_DATABASE_OID="+strconv.FormatUint(uint64(physical.DatabaseOID), 10),
		"CUSTODY_SOURCE_ID="+request.SourceID, "CUSTODY_EPOCH="+strconv.FormatInt(request.Epoch, 10))
	output, err = command.CombinedOutput()
	require.NoError(t, err, string(output))
	require.NoError(t, admin.QueryRow(ctx, `SELECT datallowconn FROM pg_database WHERE oid=$1`, physical.DatabaseOID).Scan(&allowed))
	require.False(t, allowed)
}

func TestQuarantineCommandPreservesExistingSourceOverTLS(t *testing.T) {
	if testing.Short() {
		t.Skip("requires two private TLS PostgreSQL servers")
	}
	ctx := context.Background()
	integrationtest.ConfigureDockerTesting()
	controlURL, control := recoveryTLSDatabase(t, ctx, "quarantine-control")
	sourceURL, source := recoveryTLSDatabase(t, ctx, "quarantine-source")
	_, err := source.Exec(ctx, `CREATE TABLE preserved_user_data(value text NOT NULL); INSERT INTO preserved_user_data VALUES('existing')`)
	require.NoError(t, err)
	adminParsed, err := url.Parse(sourceURL)
	require.NoError(t, err)
	adminParsed.Path = "/postgres"
	adminURL := adminParsed.String()
	admin, err := pgxpool.New(ctx, adminURL)
	require.NoError(t, err)
	defer admin.Close()
	request := CreationRequest{SourceID: uuid.NewString(), Epoch: 1, Database: "custody_test"}
	require.NoError(t, control.QueryRow(ctx, `SELECT system_identifier::text FROM pg_control_system()`).Scan(&request.ControlSystemID))
	require.NoError(t, admin.QueryRow(ctx, `SELECT system_identifier::text FROM pg_control_system()`).Scan(&request.SourceSystemID))
	var oid uint32
	require.NoError(t, admin.QueryRow(ctx, `SELECT oid FROM pg_database WHERE datname=$1`, request.Database).Scan(&oid))
	binary := filepath.Join(t.TempDir(), "sourcecustody.exe")
	build := exec.CommandContext(ctx, "go", "build", "-o", binary, "../../cmd/sourcecustody")
	output, err := build.CombinedOutput()
	require.NoError(t, err, string(output))
	command := exec.CommandContext(ctx, binary, "quarantine-existing")
	command.Env = append(os.Environ(), "CUSTODY_CONTROL_DATABASE_URL="+controlURL, "CUSTODY_SOURCE_ADMIN_DATABASE_URL="+adminURL,
		"CUSTODY_CONTROL_SYSTEM_ID="+request.ControlSystemID, "CUSTODY_SOURCE_SYSTEM_ID="+request.SourceSystemID,
		"CUSTODY_SOURCE_DATABASE="+request.Database, "CUSTODY_SOURCE_DATABASE_OID="+strconv.FormatUint(uint64(oid), 10),
		"CUSTODY_SOURCE_ID="+request.SourceID, "CUSTODY_EPOCH="+strconv.FormatInt(request.Epoch, 10))
	output, err = command.CombinedOutput()
	require.NoError(t, err, string(output))
	var allowed bool
	require.NoError(t, admin.QueryRow(ctx, `SELECT datallowconn FROM pg_database WHERE oid=$1`, oid).Scan(&allowed))
	require.False(t, allowed)
	var phase string
	require.NoError(t, control.QueryRow(ctx, `SELECT phase FROM user_custody_creation WHERE singleton`).Scan(&phase))
	require.Equal(t, "QUARANTINED_EXISTING", phase)
	_, err = admin.Exec(ctx, `ALTER DATABASE custody_test WITH ALLOW_CONNECTIONS true`)
	require.NoError(t, err)
	probe, err := pgxpool.New(ctx, sourceURL)
	require.NoError(t, err)
	defer probe.Close()
	var value string
	require.NoError(t, probe.QueryRow(ctx, `SELECT value FROM preserved_user_data`).Scan(&value))
	require.Equal(t, "existing", value)
}

func TestRecoveryRuntimeConfigRequiresCompletePhysicalPins(t *testing.T) {
	env := map[string]string{
		"CUSTODY_CONTROL_DATABASE_URL":      "postgres://owner:password@control.example:5432/control?sslmode=verify-full&sslrootcert=ca.pem",
		"CUSTODY_SOURCE_ADMIN_DATABASE_URL": "postgres://owner:password@source.example:5432/postgres?sslmode=verify-full&sslrootcert=ca.pem",
		"CUSTODY_CONTROL_SYSTEM_ID":         "123", "CUSTODY_SOURCE_SYSTEM_ID": "456",
		"CUSTODY_SOURCE_DATABASE": "voice_source_test", "CUSTODY_SOURCE_DATABASE_OID": "12345",
		"CUSTODY_SOURCE_ID": "c9a0874e-f626-4da9-8266-66de9f4e6fcb", "CUSTODY_EPOCH": "1",
	}
	lookup := func(key string) (string, bool) { value, ok := env[key]; return value, ok }
	cfg, err := LoadRecoveryConfig(lookup)
	require.NoError(t, err)
	require.Equal(t, uint32(12345), cfg.Physical.DatabaseOID)
	for key, value := range env {
		delete(env, key)
		_, err = LoadRecoveryConfig(lookup)
		require.Error(t, err, key)
		env[key] = value
	}
	env["CUSTODY_SOURCE_DATABASE_OID"] = "012345"
	_, err = LoadRecoveryConfig(lookup)
	require.Error(t, err)
	env["CUSTODY_SOURCE_DATABASE_OID"] = "12345"
	env["CUSTODY_SOURCE_ADMIN_DATABASE_URL"] = "postgres://owner@source.example:5432/postgres?sslmode=disable"
	_, err = LoadRecoveryConfig(lookup)
	require.Error(t, err)
}
