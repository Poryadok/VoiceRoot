//go:build linux

package main

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
)

// This opt-in check exercises the unchanged signer against root-owned regular
// files presented to the same nonroot UID used by the Messaging image. It is
// not proof of Kubernetes subPath behavior or a live rollout.
func TestManagedChatPurgeMountedKeysNonroot(t *testing.T) {
	dir := os.Getenv("VOICE_MESSAGING_MOUNT_TEST_DIR")
	if dir == "" {
		t.Skip("requires isolated mounted-key fixture")
	}
	require.Equal(t, 65532, os.Geteuid())
	for _, kid := range []string{"current", "next"} {
		info, err := os.Lstat(filepath.Join(dir, kid+".pem"))
		require.NoError(t, err)
		require.True(t, info.Mode().IsRegular())
		require.Equal(t, os.FileMode(0o444), info.Mode().Perm())
		stat, ok := info.Sys().(*syscall.Stat_t)
		require.True(t, ok)
		require.Equal(t, uint32(0), stat.Uid)
	}
	t.Setenv(managedChatPurgeSigningKeysDirEnv, dir)
	t.Setenv(managedChatPurgeActiveKIDEnv, "current")
	issuer, jwks, enabled, err := loadManagedChatPurgePrincipal()
	require.NoError(t, err)
	require.True(t, enabled)
	require.NotNil(t, issuer)
	require.NotNil(t, jwks)
}
