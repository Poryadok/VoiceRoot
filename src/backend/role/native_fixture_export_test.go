//go:build role_space_native

package main

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"voice/backend/role/testdata/nativefixture"
)

func holdPortableNativeFixture(t *testing.T, ctx context.Context, manifest nativefixture.Manifest, ca, clientCert, clientKey []byte, botKey *rsa.PrivateKey) {
	t.Helper()
	dir := os.Getenv("ROLE_NATIVE_FIXTURE_EXPORT_DIR")
	if dir == "" {
		return
	}
	require.Equal(t, "linux", runtime.GOOS, "portable listeners run only in the caller-owned Linux runner")
	require.True(t, filepath.IsAbs(dir))
	require.EqualValues(t, 25, manifest.RoleSchema)
	require.EqualValues(t, 44, manifest.SpaceSchema)
	require.NotEmpty(t, manifest.SpaceServingDSN)
	require.NotEmpty(t, manifest.RoleDatabaseSystemID)
	require.NotEqual(t, manifest.RoleDatabaseSystemID, manifest.SpaceDatabaseSystemID, "Role and Space are independent actual PostgreSQL clusters")
	manifest.RoleSource = os.Getenv("ROLE_NATIVE_FIXTURE_ROLE_SOURCE")
	_, err := hex.DecodeString(manifest.RoleSource)
	require.NoError(t, err)
	require.Len(t, manifest.RoleSource, 40, "explicit immutable caller source binding required")
	require.Equal(t, strings.ToLower(manifest.RoleSource), manifest.RoleSource)
	require.NoError(t, os.MkdirAll(filepath.Dir(dir), 0700))
	require.NoError(t, os.Mkdir(dir, 0700), "fresh export directory required; never overwrite another fixture")
	manifest.SchemaVersion, manifest.FixtureOnly = 1, true
	manifest.ExpiresAt = time.Now().Add(5 * time.Minute).UTC()
	if deadline, ok := ctx.Deadline(); ok && deadline.Before(manifest.ExpiresAt) {
		manifest.ExpiresAt = deadline
	}
	manifest.StopFile = filepath.Join(dir, "stop")
	manifest.TLSCAFile = filepath.Join(dir, "role-ca.pem")
	manifest.TLSClientCertFile = filepath.Join(dir, "role-client.pem")
	manifest.TLSClientKeyFile = filepath.Join(dir, "role-client-key.pem")
	manifest.BotSignerKeyFile = filepath.Join(dir, "bot-fixture-key.pem")
	botDER, err := x509.MarshalPKCS8PrivateKey(botKey)
	require.NoError(t, err)
	files := map[string][]byte{manifest.TLSCAFile: ca, manifest.TLSClientCertFile: clientCert, manifest.TLSClientKeyFile: clientKey, manifest.BotSignerKeyFile: pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: botDER})}
	ready := filepath.Join(dir, "ready.json")
	defer func() {
		_ = os.Remove(ready)
		for path := range files {
			_ = os.Remove(path)
		}
	}()
	for path, data := range files {
		require.NoError(t, os.WriteFile(path, data, 0600))
	}
	raw, err := json.MarshalIndent(manifest, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(ready+".tmp", raw, 0600))
	require.NoError(t, os.Rename(ready+".tmp", ready), "publish only after all real positive protocol checks and credential files")
	t.Logf("private native fixture ready: %s; bounded stop file: %s", ready, manifest.StopFile)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	timeout := time.NewTimer(time.Until(manifest.ExpiresAt))
	defer timeout.Stop()
	for {
		select {
		case <-ctx.Done():
			t.Fatalf("portable fixture context ended before consumer stop: %v", ctx.Err())
		case <-timeout.C:
			t.Fatal("portable fixture bounded lifetime expired before consumer stop")
		case <-ticker.C:
			_, err := os.Stat(manifest.StopFile)
			if err == nil {
				return
			}
			require.True(t, os.IsNotExist(err), "cannot observe owned consumer stop file")
		}
	}
}
