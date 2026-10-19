package releasepolicy

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOwnerRuntimeBindsSnapshotAndRejectsUnsafeConfig(t *testing.T) {
	data, err := os.ReadFile("testdata/free-invite-alpha.json")
	require.NoError(t, err)
	sum := sha256.Sum256(data)
	path := filepath.Join(t.TempDir(), "snapshot.json")
	env := map[string]string{"VOICE_RELEASE_PROFILE": Alpha, "VOICE_RELEASE_POLICY_FILE": filepath.Join(t.TempDir(), "policy.json"), "VOICE_RELEASE_POLICY_SHA256": hex.EncodeToString(sum[:]), "VOICE_RELEASE_SNAPSHOT_FILE": path}
	lookup := func(k string) (string, bool) { v, ok := env[k]; return v, ok }
	runtime, err := LoadOwnerRuntime(lookup, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", func(string) ([]byte, error) { return data, nil })
	require.NoError(t, err)
	require.False(t, runtime.Policy().Allows("paid"))
	require.Error(t, runtime.WriteSnapshot("user", "node", nil, []string{"user.custom_status.v1"}, []string{"grpc:/UpdatePresence"}, map[string]string{"db": "postgres"}))
	require.NoFileExists(t, path)
	require.NoError(t, runtime.WriteSnapshot("user", "node", runtime.Policy(), []string{"user.custom_status.v1"}, []string{"grpc:/UpdatePresence"}, map[string]string{"db": "postgres"}))
	bytes, err := os.ReadFile(path)
	require.NoError(t, err)
	var snap map[string]any
	require.NoError(t, json.Unmarshal(bytes, &snap))
	require.Equal(t, "user", snap["service_id"])
	require.Equal(t, env["VOICE_RELEASE_POLICY_SHA256"], snap["policy_sha256"])
	info, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0600), info.Mode().Perm())
	require.Error(t, runtime.WriteSnapshot("user", "node", runtime.Policy(), []string{"user.custom_status.v1"}, []string{"grpc:/UpdatePresence"}, map[string]string{"db": "postgres"}), "never overwrite evidence")
	delete(env, "VOICE_RELEASE_SNAPSHOT_FILE")
	_, err = LoadOwnerRuntime(lookup, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", func(string) ([]byte, error) { return data, nil })
	require.Error(t, err)
	env["VOICE_RELEASE_PROFILE"] = Development
	_, err = LoadOwnerRuntime(lookup, "", nil)
	require.Error(t, err)
	env = map[string]string{"VOICE_RELEASE_PROFILE": Development}
	runtime, err = LoadOwnerRuntime(lookup, "", nil)
	require.NoError(t, err)
	require.True(t, runtime.Policy().Allows("paid"))
}
