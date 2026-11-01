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
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/pkg/integrationtest"
)

func TestCreationRuntimeConfigRequiresCompleteInputs(t *testing.T) {
	env := map[string]string{
		"CUSTODY_CONTROL_DATABASE_URL":      "postgres://owner:password@control.example:5432/control?sslmode=verify-full&sslrootcert=ca.pem",
		"CUSTODY_SOURCE_ADMIN_DATABASE_URL": "postgres://owner:password@source.example:5432/postgres?sslmode=verify-full&sslrootcert=ca.pem",
		"CUSTODY_CONTROL_SYSTEM_ID":         "123", "CUSTODY_SOURCE_SYSTEM_ID": "456", "CUSTODY_SOURCE_DATABASE": "voice_source_test",
		"CUSTODY_SOURCE_ID": "c9a0874e-f626-4da9-8266-66de9f4e6fcb", "CUSTODY_EPOCH": "1",
		"CUSTODY_USER_MIGRATIONS_DIR": "migrations", "CUSTODY_USER_KEY_MANIFEST": "manifest.json", "CUSTODY_USER_KEYS_DIR": "keys",
	}
	lookup := func(key string) (string, bool) { value, ok := env[key]; return value, ok }
	_, err := LoadCreationConfig(lookup)
	require.NoError(t, err)
	for key, value := range env {
		delete(env, key)
		_, err = LoadCreationConfig(lookup)
		require.Error(t, err, key)
		env[key] = value
	}
	env["CUSTODY_EPOCH"] = "01"
	_, err = LoadCreationConfig(lookup)
	require.Error(t, err)
}

func TestCreationCommandInitializesClosedSourceOverTLS(t *testing.T) {
	if testing.Short() {
		t.Skip("requires two private TLS PostgreSQL servers")
	}
	ctx := context.Background()
	integrationtest.ConfigureDockerTesting()
	controlURL, control := recoveryTLSDatabase(t, ctx, "create-control")
	adminURL, admin := recoveryTLSDatabase(t, ctx, "create-source")
	request := CreationRequest{SourceID: uuid.NewString(), Epoch: 1, Database: "voice_source_test"}
	require.NoError(t, control.QueryRow(ctx, `SELECT system_identifier::text FROM pg_control_system()`).Scan(&request.ControlSystemID))
	require.NoError(t, admin.QueryRow(ctx, `SELECT system_identifier::text FROM pg_control_system()`).Scan(&request.SourceSystemID))
	dir, _ := migrationFixture(t)
	keyDir := t.TempDir()
	key := bytes.Repeat([]byte{0x71}, 32)
	fingerprint := sha256.Sum256(key)
	require.NoError(t, os.WriteFile(filepath.Join(keyDir, "key.bin"), key, 0600))
	manifest := filepath.Join(keyDir, "manifest.json")
	require.NoError(t, os.WriteFile(manifest, []byte(fmt.Sprintf(`{"purpose":"voice.user.identity-association.v1","revision":1,"active_version":1,"versions":[{"version":1,"sha256":"%x","file":"key.bin"}]}`, fingerprint)), 0600))
	require.Error(t, RunCreation(ctx, CreationConfig{ControlURL: controlURL, SourceAdminURL: adminURL, Request: request, MigrationsDir: filepath.Join(dir, "missing"), KeyManifest: manifest, KeysDir: keyDir}))
	cfg := CreationConfig{ControlURL: controlURL, SourceAdminURL: adminURL, Request: request, MigrationsDir: dir, KeyManifest: manifest + ".missing", KeysDir: keyDir}
	require.Error(t, RunCreation(ctx, cfg))
	var exists bool
	require.NoError(t, admin.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname=$1)`, request.Database).Scan(&exists))
	require.False(t, exists, "bad key material must be rejected before database creation")
	binary := filepath.Join(t.TempDir(), "sourcecustody.exe")
	build := exec.CommandContext(ctx, "go", "build", "-o", binary, "../../cmd/sourcecustody")
	output, err := build.CombinedOutput()
	require.NoError(t, err, string(output))
	command := exec.CommandContext(ctx, binary, "create-closed")
	command.Env = append(os.Environ(), "CUSTODY_CONTROL_DATABASE_URL="+controlURL, "CUSTODY_SOURCE_ADMIN_DATABASE_URL="+adminURL,
		"CUSTODY_CONTROL_SYSTEM_ID="+request.ControlSystemID, "CUSTODY_SOURCE_SYSTEM_ID="+request.SourceSystemID,
		"CUSTODY_SOURCE_DATABASE="+request.Database, "CUSTODY_SOURCE_ID="+request.SourceID, "CUSTODY_EPOCH="+strconv.FormatInt(request.Epoch, 10),
		"CUSTODY_USER_MIGRATIONS_DIR="+dir, "CUSTODY_USER_KEY_MANIFEST="+manifest, "CUSTODY_USER_KEYS_DIR="+keyDir)
	output, err = command.CombinedOutput()
	require.NoError(t, err, string(output))
	var phase, state string
	var allowed bool
	require.NoError(t, control.QueryRow(ctx, `SELECT phase FROM user_custody_creation WHERE singleton`).Scan(&phase))
	require.Equal(t, "INITIALIZED_CLOSED", phase)
	require.NoError(t, control.QueryRow(ctx, `SELECT state FROM user_custody_state WHERE singleton`).Scan(&state))
	require.Equal(t, "CLOSED", state)
	require.NoError(t, admin.QueryRow(ctx, `SELECT datallowconn FROM pg_database WHERE datname=$1`, request.Database).Scan(&allowed))
	require.False(t, allowed)
}
