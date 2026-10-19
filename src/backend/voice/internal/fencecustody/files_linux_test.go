//go:build linux

package fencecustody

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/voice/internal/retirementproof"
)

func TestProtectedFileStoreAndActualUnprivilegedReader(t *testing.T) {
	if os.Getenv("FENCE_CUSTODY_READER_CHILD") == "1" {
		require.Equal(t, 1000, os.Geteuid())
		root := os.Getenv("FENCE_CUSTODY_TEST_ROOT")
		b, err := readInstalledFile(root, "receipt.json", false)
		require.NoError(t, err)
		require.Equal(t, []byte(`{"receipt":"retained"}`), b)
		_, err = os.ReadFile(filepath.Join(root, "secret.dsn"))
		require.Error(t, err, "actual OS permission denies secret read")
		require.Error(t, os.WriteFile(filepath.Join(root, "forged.json"), []byte("forged"), 0444))
		require.ErrorIs(t, writeInstalledFile(root, "forged.json", []byte("forged")), retirementproof.ErrUnavailable)
		require.Empty(t, os.Getenv("PARTY_COVERAGE_TEST_POSTGRES"), "admin fixture environment is not inherited")
		return
	}
	require.Zero(t, os.Geteuid(), "installer fixture must run in the owned Linux root runner")
	root := "/run/rudeus191-fencer-" + uuid.NewString()
	require.NoError(t, os.Mkdir(root, 0755))
	t.Cleanup(func() { require.NoError(t, os.RemoveAll(root)) })
	raw := []byte(`{"receipt":"retained"}`)
	require.NoError(t, writeInstalledFile(root, "receipt.json", raw))
	require.NoError(t, writeInstalledFile(root, "receipt.json", raw), "exact lost acknowledgement retry")
	require.ErrorIs(t, writeInstalledFile(root, "receipt.json", []byte("changed")), retirementproof.ErrConflict)
	got, err := readInstalledFile(root, "receipt.json", false)
	require.NoError(t, err)
	require.Equal(t, raw, got)
	require.NoError(t, os.WriteFile(filepath.Join(root, "secret.dsn"), []byte("private installer credential fixture"), 0400))
	_, err = readInstalledFile(root, "secret.dsn", true)
	require.NoError(t, err)
	t.Run("symlink", func(t *testing.T) {
		require.NoError(t, os.Symlink(filepath.Join(root, "receipt.json"), filepath.Join(root, "alias.json")))
		_, err := readInstalledFile(root, "alias.json", false)
		require.ErrorIs(t, err, retirementproof.ErrUnavailable)
	})
	t.Run("writable store", func(t *testing.T) {
		require.NoError(t, os.Chmod(root, 0777))
		_, err := readInstalledFile(root, "receipt.json", false)
		require.ErrorIs(t, err, retirementproof.ErrUnavailable)
		require.NoError(t, os.Chmod(root, 0755))
	})
	t.Run("writable receipt", func(t *testing.T) {
		path := filepath.Join(root, "receipt.json")
		require.NoError(t, os.Chmod(path, 0666))
		_, err := readInstalledFile(root, "receipt.json", false)
		require.ErrorIs(t, err, retirementproof.ErrUnavailable)
		require.NoError(t, os.Chmod(path, 0444))
	})
	t.Run("actual distinct process", func(t *testing.T) {
		binary, err := os.Executable()
		require.NoError(t, err)
		code, err := os.ReadFile(binary)
		require.NoError(t, err)
		child := filepath.Join(root, "reader-test")
		require.NoError(t, os.WriteFile(child, code, 0555))
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, child, "-test.run=^TestProtectedFileStoreAndActualUnprivilegedReader$", "-test.v")
		cmd.Env = []string{"FENCE_CUSTODY_READER_CHILD=1", "FENCE_CUSTODY_TEST_ROOT=" + root, "PATH=/usr/bin:/bin"}
		cmd.Dir = root
		cmd.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: 1000, Gid: 1000}}
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, "%s", output)
	})
}
