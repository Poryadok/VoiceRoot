//go:build linux

package nativeapp

import (
	"crypto/sha256"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"
	w "voice/backend/pkg/mmadmissionwire"
)

func TestNativeExecutableUsesActualRunningBinaryAndFixedArgv(t *testing.T) {
	raw, err := os.ReadFile("/proc/self/exe")
	require.NoError(t, err)
	d := descriptorFixture(t)
	d.AdmittedExecutableHash = w.Hash(sha256.Sum256(raw))
	d.AdmittedArgv = []string{"/service-native"}
	require.NoError(t, checkNativeExecutable(d, []string{"/service-native"}))
	d.AdmittedExecutableHash[0] ^= 1
	require.Error(t, checkNativeExecutable(d, []string{"/service-native"}))
	d.AdmittedExecutableHash = w.Hash(sha256.Sum256(raw))
	require.Error(t, checkNativeExecutable(d, []string{"/service-native", "--legacy"}))
	d.AdmittedArgv = []string{"/service"}
	require.Error(t, checkNativeExecutable(d, []string{"/service"}))
}

func TestNativeDescriptorRefusesWritableFilesystemAndSymlink(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "descriptor.json")
	require.NoError(t, os.WriteFile(path, []byte("{}"), 0444))
	_, err := readProtectedDescriptor(path)
	require.Error(t, err, "read-only mode alone does not establish a read-only installation mount")
	link := filepath.Join(dir, "descriptor-link.json")
	require.NoError(t, os.Symlink(path, link))
	_, err = readProtectedDescriptor(link)
	require.Error(t, err)
	_, err = readProtectedDescriptor(filepath.Join(dir, "absent"))
	require.Error(t, err)
}
