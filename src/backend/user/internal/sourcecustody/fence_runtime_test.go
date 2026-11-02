package sourcecustody

import (
	"context"
	"encoding/hex"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"voice/backend/pkg/integrationtest"
)

func TestFenceRuntimeConfigRequiresCompleteBinding(t *testing.T) {
	env := map[string]string{
		"CUSTODY_CONTROL_DATABASE_URL":      "postgres://owner:password@control.example:5432/control?sslmode=verify-full&sslrootcert=ca.pem",
		"CUSTODY_SOURCE_ADMIN_DATABASE_URL": "postgres://owner:password@source.example:5432/postgres?sslmode=verify-full&sslrootcert=ca.pem",
		"CUSTODY_CONTROL_SYSTEM_ID":         "123", "CUSTODY_SOURCE_SYSTEM_ID": "456", "CUSTODY_SOURCE_DATABASE": "voice_source_test", "CUSTODY_SOURCE_DATABASE_OID": "12345",
		"CUSTODY_SOURCE_ID": "c9a0874e-f626-4da9-8266-66de9f4e6fcb", "CUSTODY_EPOCH": "1", "CUSTODY_HISTORY_GENERATION": "644ba66a-a9f2-4bc1-b4ee-1e7d429f970c",
		"CUSTODY_USER_SOURCE_GENERATION": "24025444-818f-49b1-9dd3-a0f2796882dc", "CUSTODY_KEYSET_DIGEST": "0101010101010101010101010101010101010101010101010101010101010101",
	}
	lookup := func(key string) (string, bool) { value, ok := env[key]; return value, ok }
	_, err := LoadFenceConfig(lookup)
	require.NoError(t, err)
	for key, value := range env {
		delete(env, key)
		_, err = LoadFenceConfig(lookup)
		require.Error(t, err, key)
		env[key] = value
	}
	env["CUSTODY_KEYSET_DIGEST"] = "0000000000000000000000000000000000000000000000000000000000000000"
	_, err = LoadFenceConfig(lookup)
	require.Error(t, err)
	env["CUSTODY_KEYSET_DIGEST"] = "0101010101010101010101010101010101010101010101010101010101010101"
	env["CUSTODY_USER_SOURCE_GENERATION"] = env["CUSTODY_HISTORY_GENERATION"]
	_, err = LoadFenceConfig(lookup)
	require.Error(t, err)
}

func TestFenceCommandClosesAdmissionAndSourceOverTLS(t *testing.T) {
	if testing.Short() {
		t.Skip("requires two private TLS PostgreSQL servers")
	}
	ctx := context.Background()
	integrationtest.ConfigureDockerTesting()
	controlURL, control := recoveryTLSDatabase(t, ctx, "fence-control")
	sourceURL, source := recoveryTLSDatabase(t, ctx, "fence-source")
	_, err := source.Exec(ctx, `CREATE TABLE protected_data(value text NOT NULL); INSERT INTO protected_data VALUES('existing')`)
	require.NoError(t, err)
	adminParsed, err := url.Parse(sourceURL)
	require.NoError(t, err)
	adminParsed.Path = "/postgres"
	adminURL := adminParsed.String()
	admin, err := pgxpool.New(ctx, adminURL)
	require.NoError(t, err)
	defer admin.Close()
	binding := custodyBinding()
	request := CreationRequest{SourceID: binding.SourceID, Epoch: binding.Epoch, Database: "custody_test"}
	require.NoError(t, control.QueryRow(ctx, `SELECT system_identifier::text FROM pg_control_system()`).Scan(&request.ControlSystemID))
	require.NoError(t, admin.QueryRow(ctx, `SELECT system_identifier::text FROM pg_control_system()`).Scan(&request.SourceSystemID))
	physical := PhysicalSource{SystemIdentifier: request.SourceSystemID, Database: request.Database}
	require.NoError(t, admin.QueryRow(ctx, `SELECT oid FROM pg_database WHERE datname=$1`, request.Database).Scan(&physical.DatabaseOID))
	_, err = control.Exec(ctx, `CREATE TABLE user_custody_state(singleton boolean PRIMARY KEY,state text NOT NULL,epoch bigint NOT NULL,source_id text NOT NULL,history_generation uuid NOT NULL,user_source_generation uuid NOT NULL,keyset_digest bytea NOT NULL);
CREATE TABLE user_custody_physical_source(source_id text NOT NULL,epoch bigint NOT NULL,system_identifier text NOT NULL,database_name text NOT NULL,database_oid oid NOT NULL,PRIMARY KEY(source_id,epoch))`)
	require.NoError(t, err)
	_, err = control.Exec(ctx, `INSERT INTO user_custody_state VALUES(true,'OPEN',$1,$2,$3,$4,$5)`, binding.Epoch, binding.SourceID, binding.HistoryGeneration, binding.UserSourceGeneration, binding.KeysetDigest[:])
	require.NoError(t, err)
	_, err = control.Exec(ctx, `INSERT INTO user_custody_physical_source VALUES($1,$2,$3,$4,$5)`, binding.SourceID, binding.Epoch, physical.SystemIdentifier, physical.Database, physical.DatabaseOID)
	require.NoError(t, err)
	config := FenceConfig{Recovery: RecoveryConfig{ControlURL: controlURL, SourceAdminURL: adminURL, Request: request, Physical: physical}, Binding: binding}
	wrong := config
	wrong.Recovery.Request.ControlSystemID = "1"
	require.Error(t, RunFence(ctx, wrong))
	var allowed bool
	require.NoError(t, admin.QueryRow(ctx, `SELECT datallowconn FROM pg_database WHERE oid=$1`, physical.DatabaseOID).Scan(&allowed))
	require.True(t, allowed)
	wrong = config
	wrong.Recovery.Physical.DatabaseOID++
	require.Error(t, RunFence(ctx, wrong))
	require.NoError(t, admin.QueryRow(ctx, `SELECT datallowconn FROM pg_database WHERE oid=$1`, physical.DatabaseOID).Scan(&allowed))
	require.True(t, allowed, "a mismatched physical source must not be fenced")
	var closedState string
	require.NoError(t, control.QueryRow(ctx, `SELECT state FROM user_custody_state WHERE singleton`).Scan(&closedState))
	require.Equal(t, "CLOSED", closedState, "an attempted transition closes capture even on source mismatch")
	binary := filepath.Join(t.TempDir(), "sourcecustody.exe")
	build := exec.CommandContext(ctx, "go", "build", "-o", binary, "../../cmd/sourcecustody")
	output, err := build.CombinedOutput()
	require.NoError(t, err, string(output))
	command := exec.CommandContext(ctx, binary, "fence-current")
	command.Env = append(os.Environ(), "CUSTODY_CONTROL_DATABASE_URL="+controlURL, "CUSTODY_SOURCE_ADMIN_DATABASE_URL="+adminURL,
		"CUSTODY_CONTROL_SYSTEM_ID="+request.ControlSystemID, "CUSTODY_SOURCE_SYSTEM_ID="+request.SourceSystemID,
		"CUSTODY_SOURCE_DATABASE="+request.Database, "CUSTODY_SOURCE_DATABASE_OID="+strconv.FormatUint(uint64(physical.DatabaseOID), 10),
		"CUSTODY_SOURCE_ID="+binding.SourceID, "CUSTODY_EPOCH="+strconv.FormatInt(binding.Epoch, 10),
		"CUSTODY_HISTORY_GENERATION="+binding.HistoryGeneration.String(), "CUSTODY_USER_SOURCE_GENERATION="+binding.UserSourceGeneration.String(), "CUSTODY_KEYSET_DIGEST="+hex.EncodeToString(binding.KeysetDigest[:]))
	output, err = command.CombinedOutput()
	require.NoError(t, err, string(output))
	var state string
	require.NoError(t, control.QueryRow(ctx, `SELECT state FROM user_custody_state WHERE singleton`).Scan(&state))
	require.Equal(t, "CLOSED", state)
	require.NoError(t, admin.QueryRow(ctx, `SELECT datallowconn FROM pg_database WHERE oid=$1`, physical.DatabaseOID).Scan(&allowed))
	require.False(t, allowed)
	_, err = admin.Exec(ctx, `ALTER DATABASE custody_test WITH ALLOW_CONNECTIONS true`)
	require.NoError(t, err)
	probe, err := pgxpool.New(ctx, sourceURL)
	require.NoError(t, err)
	defer probe.Close()
	var value string
	require.NoError(t, probe.QueryRow(ctx, `SELECT value FROM protected_data`).Scan(&value))
	require.Equal(t, "existing", value)
}
