//go:build integration && linux

package nativeinstaller

import (
	"bytes"
	"encoding/json"
	"net/url"
	"os"
	"os/exec"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

// The migration that prepares the private fixture is not part of inspection.
func TestProductionLegacy24InspectReadOnlyWithActiveClient(t *testing.T) {
	ctx, adminURL, driver, binary := legacy24OwnedDatabase(t)
	target, err := url.Parse(adminURL)
	require.NoError(t, err)
	target.Path = "/space_db"
	active, err := pgx.Connect(ctx, target.String())
	require.NoError(t, err)
	defer active.Close(ctx)
	before, err := observeLegacy24Origin(ctx, active)
	require.NoError(t, err)
	cmd := exec.CommandContext(ctx, binary, "-mode", "inspect-legacy24", "-migrations-dir", "../../../migrations/space_db", "-migrate", driver)
	cmd.Env = append(os.Environ(), "SPACE_NATIVE_ADMIN_URL="+adminURL)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	require.NoError(t, cmd.Run(), "actual inspect CLI failed: %s", stderr.String())
	require.Contains(t, stdout.String(), `"event":"inspected-legacy24"`)
	require.NotContains(t, stdout.String(), "original member")
	require.NotContains(t, stdout.String(), "11111111-1111-4111-8111-111111111111")
	var evidence struct {
		Target struct {
			DatabaseOwner string `json:"database_owner"`
		} `json:"target"`
	}
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &evidence))
	var actualOwner string
	require.NoError(t, active.QueryRow(ctx, `SELECT pg_get_userbyid(datdba) FROM pg_database WHERE datname=current_database()`).Scan(&actualOwner))
	require.Equal(t, actualOwner, evidence.Target.DatabaseOwner)
	after, err := observeLegacy24Origin(ctx, active)
	require.NoError(t, err)
	require.Equal(t, before.PreimageSHA256, after.PreimageSHA256)
	require.Equal(t, before.AccessSHA256, after.AccessSHA256)
	require.Equal(t, before.Target.DatabaseOID, after.Target.DatabaseOID)
}
