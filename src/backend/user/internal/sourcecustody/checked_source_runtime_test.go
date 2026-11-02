package sourcecustody

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"voice/backend/pkg/integrationtest"
	"voice/backend/user/internal/identityassociation"
	"voice/backend/user/internal/testsupport/associationfixture"
)

func TestCheckedSourceConfigRequiresIndependentCompletePins(t *testing.T) {
	values := map[string]string{
		"CUSTODY_CONTROL_DATABASE_URL":      "postgres://owner:password@control.example:5432/control?sslmode=verify-full&sslrootcert=ca.pem",
		"CUSTODY_SOURCE_ADMIN_DATABASE_URL": "postgres://owner:password@source.example:5432/postgres?sslmode=verify-full&sslrootcert=ca.pem",
		"CUSTODY_CONTROL_SYSTEM_ID":         "123", "CUSTODY_SOURCE_SYSTEM_ID": "456", "CUSTODY_SOURCE_DATABASE": "voice_user", "CUSTODY_SOURCE_DATABASE_OID": "42", "CUSTODY_SOURCE_ID": "source", "CUSTODY_EPOCH": "1",
		"CUSTODY_HISTORY_GENERATION": "11111111-1111-4111-8111-111111111111", "CUSTODY_USER_SOURCE_GENERATION": "22222222-2222-4222-8222-222222222222", "CUSTODY_KEYSET_DIGEST": strings.Repeat("ab", 32), "CUSTODY_USER_KEY_MANIFEST": "manifest", "CUSTODY_USER_KEYS_DIR": "keys",
	}
	lookup := func(name string) (string, bool) { value, ok := values[name]; return value, ok }
	cfg, err := LoadCheckedSourceConfig(lookup)
	require.NoError(t, err)
	require.Equal(t, int64(1), cfg.Binding.Epoch)
	for _, name := range []string{"CUSTODY_SOURCE_DATABASE_OID", "CUSTODY_HISTORY_GENERATION", "CUSTODY_USER_SOURCE_GENERATION", "CUSTODY_KEYSET_DIGEST", "CUSTODY_USER_KEY_MANIFEST", "CUSTODY_USER_KEYS_DIR"} {
		t.Run(name, func(t *testing.T) {
			value := values[name]
			delete(values, name)
			defer func() { values[name] = value }()
			_, err := LoadCheckedSourceConfig(lookup)
			require.Error(t, err)
		})
	}
	values["CUSTODY_USER_SOURCE_GENERATION"] = values["CUSTODY_HISTORY_GENERATION"]
	_, err = LoadCheckedSourceConfig(lookup)
	require.Error(t, err)
}

func TestCheckedSourceCommandPreservesExistingIdentityOverTLS(t *testing.T) {
	if testing.Short() {
		t.Skip("requires two private TLS PostgreSQL servers")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	integrationtest.ConfigureDockerTesting()
	controlURL, control := recoveryTLSDatabase(t, ctx, "checked-control")
	adminURL, admin := recoveryTLSDatabase(t, ctx, "checked-source")
	t.Setenv("USER_ASSOCIATION_TEST_DSN", adminURL)
	db := associationfixture.New(t, 1)
	receipt, err := associationfixture.ReadInstallerReceipt(db.InstallerReceiptPath)
	require.NoError(t, err)
	keys, err := identityassociation.NewKeyset(identityassociation.KeysetSnapshot{Revision: 1, ActiveVersion: receipt.KeyVersion, Versions: map[uint32][32]byte{receipt.KeyVersion: receipt.KeyFingerprint}}, map[uint32][]byte{receipt.KeyVersion: receipt.UserKey})
	require.NoError(t, err)
	identities, err := identityassociation.NewFreshStore(ctx, db.Pool, keys, identityassociation.FreshSourcePin{HistoryGeneration: receipt.HistoryGeneration, UserSourceGeneration: receipt.UserSourceGeneration})
	require.NoError(t, err)
	require.NoError(t, identities.ActivateGuards(ctx))
	request := CreationRequest{SourceID: uuid.NewString(), Epoch: 1, Database: db.Pool.Config().ConnConfig.Database}
	require.NoError(t, control.QueryRow(ctx, `SELECT system_identifier::text FROM pg_control_system()`).Scan(&request.ControlSystemID))
	require.NoError(t, admin.QueryRow(ctx, `SELECT system_identifier::text FROM pg_control_system()`).Scan(&request.SourceSystemID))
	physical := PhysicalSource{SystemIdentifier: request.SourceSystemID, Database: request.Database}
	require.NoError(t, admin.QueryRow(ctx, `SELECT oid FROM pg_database WHERE datname=$1`, request.Database).Scan(&physical.DatabaseOID))
	digest, err := keys.CustodyDigest()
	require.NoError(t, err)
	binding := Binding{Epoch: request.Epoch, SourceID: request.SourceID, HistoryGeneration: receipt.HistoryGeneration, UserSourceGeneration: receipt.UserSourceGeneration, KeysetDigest: digest}
	require.NoError(t, QuarantineExistingSource(ctx, control, admin, request, physical))
	_, err = admin.Exec(ctx, `REVOKE CONNECT ON DATABASE `+pgx.Identifier{request.Database}.Sanitize()+` FROM PUBLIC`)
	require.NoError(t, err)
	dir := t.TempDir()
	keyDir := filepath.Join(dir, "keys")
	require.NoError(t, os.Mkdir(keyDir, 0700))
	require.NoError(t, os.WriteFile(filepath.Join(keyDir, "v1.key"), receipt.UserKey, 0600))
	fingerprint := sha256.Sum256(receipt.UserKey)
	manifest, err := json.Marshal(map[string]any{"purpose": "voice.user.identity-association.v1", "revision": 1, "active_version": receipt.KeyVersion, "versions": []any{map[string]any{"version": receipt.KeyVersion, "sha256": fmt.Sprintf("%x", fingerprint), "file": "v1.key"}}})
	require.NoError(t, err)
	manifestPath := filepath.Join(dir, "manifest.json")
	require.NoError(t, os.WriteFile(manifestPath, manifest, 0600))
	binary := filepath.Join(dir, "sourcecustody.exe")
	build := exec.CommandContext(ctx, "go", "build", "-o", binary, "../../cmd/sourcecustody")
	output, err := build.CombinedOutput()
	require.NoError(t, err, "%s", output)
	command := exec.CommandContext(ctx, binary, "check-existing-identity")
	for _, item := range os.Environ() {
		if !strings.HasPrefix(strings.ToUpper(item), "PG") && !strings.HasPrefix(item, "CUSTODY_") {
			command.Env = append(command.Env, item)
		}
	}
	command.Env = append(command.Env, "CUSTODY_CONTROL_DATABASE_URL="+controlURL, "CUSTODY_SOURCE_ADMIN_DATABASE_URL="+adminURL,
		"CUSTODY_CONTROL_SYSTEM_ID="+request.ControlSystemID, "CUSTODY_SOURCE_SYSTEM_ID="+request.SourceSystemID, "CUSTODY_SOURCE_DATABASE="+request.Database,
		"CUSTODY_SOURCE_DATABASE_OID="+strconv.FormatUint(uint64(physical.DatabaseOID), 10), "CUSTODY_SOURCE_ID="+request.SourceID, "CUSTODY_EPOCH=1",
		"CUSTODY_HISTORY_GENERATION="+binding.HistoryGeneration.String(), "CUSTODY_USER_SOURCE_GENERATION="+binding.UserSourceGeneration.String(),
		"CUSTODY_KEYSET_DIGEST="+fmt.Sprintf("%x", binding.KeysetDigest), "CUSTODY_USER_KEY_MANIFEST="+manifestPath, "CUSTODY_USER_KEYS_DIR="+keyDir)
	output, err = command.CombinedOutput()
	require.NoError(t, err, "%s", output)
	var phase, state string
	require.NoError(t, control.QueryRow(ctx, `SELECT phase FROM user_custody_creation WHERE singleton`).Scan(&phase))
	require.NoError(t, control.QueryRow(ctx, `SELECT state FROM user_custody_state WHERE singleton`).Scan(&state))
	require.Equal(t, "EXISTING_INITIALIZED_CLOSED", phase)
	require.Equal(t, "CLOSED", state)
	controller, err := Start(ctx, control, binding)
	require.NoError(t, err)
	_, _, err = controller.AcquireAttested(ctx, binding)
	require.Error(t, err)
	require.NoError(t, controller.Shutdown(ctx))
}
