package sourcecustody

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"voice/backend/pkg/integrationtest"
)

func TestAdoptionConfigRequiresCompleteCanonicalPins(t *testing.T) {
	values := map[string]string{
		"CUSTODY_CONTROL_DATABASE_URL":      "postgres://owner:password@control.example:5432/control?sslmode=verify-full&sslrootcert=ca.pem",
		"CUSTODY_SOURCE_ADMIN_DATABASE_URL": "postgres://owner:password@source.example:5432/postgres?sslmode=verify-full&sslrootcert=ca.pem",
		"CUSTODY_CONTROL_SYSTEM_ID":         "123", "CUSTODY_SOURCE_SYSTEM_ID": "456", "CUSTODY_SOURCE_DATABASE": "voice_user", "CUSTODY_SOURCE_DATABASE_OID": "42", "CUSTODY_SOURCE_ID": "source", "CUSTODY_EPOCH": "1",
		"CUSTODY_HISTORY_GENERATION": "11111111-1111-4111-8111-111111111111", "CUSTODY_USER_SOURCE_GENERATION": "22222222-2222-4222-8222-222222222222", "CUSTODY_KEYSET_DIGEST": strings.Repeat("ab", 32), "CUSTODY_USER_KEY_MANIFEST": "manifest", "CUSTODY_USER_KEYS_DIR": "keys",
		"CUSTODY_ADOPTION_RECEIPT_ID": "33333333-3333-4333-8333-333333333333", "CUSTODY_ADOPTION_EVIDENCE_DIGEST": strings.Repeat("cd", 32),
	}
	lookup := func(name string) (string, bool) { v, ok := values[name]; return v, ok }
	_, err := LoadAdoptionConfig(lookup)
	require.NoError(t, err)
	for name, value := range values {
		delete(values, name)
		_, err = LoadAdoptionConfig(lookup)
		require.Error(t, err, name)
		values[name] = value
	}
	for name, bad := range map[string]string{"CUSTODY_ADOPTION_RECEIPT_ID": uuid.Nil.String(), "CUSTODY_ADOPTION_EVIDENCE_DIGEST": strings.Repeat("00", 32), "CUSTODY_CONTROL_DATABASE_URL": "postgres://owner:password@control.example/control?sslmode=disable", "CUSTODY_SOURCE_ADMIN_DATABASE_URL": "postgres://owner:password@source.example/postgres?sslmode=require"} {
		good := values[name]
		values[name] = bad
		_, err = LoadAdoptionConfig(lookup)
		require.Error(t, err, name)
		values[name] = good
	}
	values["CUSTODY_ADOPTION_EVIDENCE_DIGEST"] = strings.Repeat("CD", 32)
	_, err = LoadAdoptionConfig(lookup)
	require.Error(t, err)
}

func TestAdoptionCommandReturnsClosedAndRetryHoldOverTLS(t *testing.T) {
	if testing.Short() {
		t.Skip("requires two private TLS PostgreSQL servers")
	}
	ctx := context.Background()
	integrationtest.ConfigureDockerTesting()
	controlURL, control := recoveryTLSDatabase(t, ctx, "adoption-cli-control")
	adminURL, admin := recoveryTLSDatabase(t, ctx, "adoption-cli-source")
	request := CreationRequest{SourceID: uuid.NewString(), Epoch: 1, Database: "voice_adoption_cli"}
	require.NoError(t, control.QueryRow(ctx, `SELECT system_identifier::text FROM pg_control_system()`).Scan(&request.ControlSystemID))
	require.NoError(t, admin.QueryRow(ctx, `SELECT system_identifier::text FROM pg_control_system()`).Scan(&request.SourceSystemID))
	physical, _, _ := prepareExistingMigration(t, ctx, control, admin, request, 22)
	_, keys := migrationFixture(t)
	pool := adoptionProbe(t, ctx, admin, physical, true)
	binding := Binding{SourceID: request.SourceID, Epoch: request.Epoch}
	require.NoError(t, pool.QueryRow(ctx, `SELECT history_generation,user_source_generation FROM identity_meta WHERE singleton`).Scan(&binding.HistoryGeneration, &binding.UserSourceGeneration))
	binding.KeysetDigest, _ = keys.CustodyDigest()
	pool.Close()
	_, err := admin.Exec(ctx, `ALTER DATABASE `+pgx.Identifier{physical.Database}.Sanitize()+` WITH ALLOW_CONNECTIONS false`)
	require.NoError(t, err)
	dir := t.TempDir()
	key := bytes.Repeat([]byte{0x71}, 32)
	fingerprint := sha256.Sum256(key)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "key.bin"), key, 0600))
	manifest := filepath.Join(dir, "manifest.json")
	require.NoError(t, os.WriteFile(manifest, []byte(fmt.Sprintf(`{"purpose":"voice.user.identity-association.v1","revision":1,"active_version":1,"versions":[{"version":1,"sha256":"%x","file":"key.bin"}]}`, fingerprint)), 0600))
	binary := filepath.Join(dir, "sourcecustody.exe")
	build := exec.CommandContext(ctx, "go", "build", "-o", binary, "../../cmd/sourcecustody")
	output, err := build.CombinedOutput()
	require.NoError(t, err, "%s", output)
	var env []string
	for _, item := range os.Environ() {
		if !strings.HasPrefix(strings.ToUpper(item), "PG") && !strings.HasPrefix(item, "CUSTODY_") {
			env = append(env, item)
		}
	}
	env = append(env, "CUSTODY_CONTROL_DATABASE_URL="+controlURL, "CUSTODY_SOURCE_ADMIN_DATABASE_URL="+adminURL, "CUSTODY_CONTROL_SYSTEM_ID="+request.ControlSystemID, "CUSTODY_SOURCE_SYSTEM_ID="+request.SourceSystemID, "CUSTODY_SOURCE_DATABASE="+request.Database, "CUSTODY_SOURCE_DATABASE_OID="+strconv.FormatUint(uint64(physical.DatabaseOID), 10), "CUSTODY_SOURCE_ID="+request.SourceID, "CUSTODY_EPOCH=1", "CUSTODY_HISTORY_GENERATION="+binding.HistoryGeneration.String(), "CUSTODY_USER_SOURCE_GENERATION="+binding.UserSourceGeneration.String(), "CUSTODY_KEYSET_DIGEST="+fmt.Sprintf("%x", binding.KeysetDigest), "CUSTODY_USER_KEY_MANIFEST="+manifest, "CUSTODY_USER_KEYS_DIR="+dir, "CUSTODY_ADOPTION_RECEIPT_ID="+uuid.NewString(), "CUSTODY_ADOPTION_EVIDENCE_DIGEST="+strings.Repeat("ab", 32))
	command := exec.CommandContext(ctx, binary, "adopt-existing-pristine")
	command.Env = env
	output, err = command.CombinedOutput()
	require.NoError(t, err, "%s", output)
	require.Equal(t, "CLOSED", strings.TrimSpace(string(output)))
	command = exec.CommandContext(ctx, binary, "adopt-existing-pristine")
	command.Env = env
	output, err = command.CombinedOutput()
	require.Error(t, err)
	require.Contains(t, string(output), "HOLD")
	var phase, state string
	var allowed bool
	require.NoError(t, control.QueryRow(ctx, `SELECT phase FROM user_custody_adoption WHERE singleton`).Scan(&phase))
	require.Equal(t, "ADOPTED_CLOSED", phase)
	require.NoError(t, control.QueryRow(ctx, `SELECT state FROM user_custody_state WHERE singleton`).Scan(&state))
	require.Equal(t, "CLOSED", state)
	require.NoError(t, admin.QueryRow(ctx, `SELECT datallowconn FROM pg_database WHERE oid=$1`, physical.DatabaseOID).Scan(&allowed))
	require.False(t, allowed)
	// Reuse this owned TLS server for the existing pristine kernel regressions.
	// Those tests otherwise skip without USER_ASSOCIATION_TEST_DSN. Their
	// independently created databases are dropped by the existing test fixture.
	native := exec.CommandContext(ctx, "go", "test", "-race", "../identityassociation", "-run", "TestNativeFreshBootstrap|TestPristineProvisioning", "-count=1", "-v", "-timeout=90s")
	native.Env = append(os.Environ(), "USER_ASSOCIATION_TEST_DSN="+adminURL)
	output, err = native.CombinedOutput()
	require.NoError(t, err, "%s", output)
	require.NotContains(t, string(output), "SKIP", "native helper regressions must execute")
	require.Contains(t, string(output), "PASS: TestNativeFreshBootstrapRequiresBothPinnedGenerationsAndPristineSource")
	require.Contains(t, string(output), "PASS: TestNativeFreshBootstrapConcurrentCallersCommitOnce")
}
