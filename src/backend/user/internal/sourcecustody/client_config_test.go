package sourcecustody

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func completeCustodyEnv() map[string]string {
	return map[string]string{
		"USER_CUSTODY_ADDR": "127.0.0.1:12345", "USER_CUSTODY_TLS_CA_FILE": "ca.pem",
		"USER_CUSTODY_TLS_SERVER_NAME": "localhost", "USER_CUSTODY_TLS_CERT_FILE": "client.pem",
		"USER_CUSTODY_TLS_KEY_FILE": "client.key", "USER_CUSTODY_SIGNING_KEYS_DIR": "keys",
		"USER_CUSTODY_ACTIVE_KID": "current", "USER_CUSTODY_HEARTBEAT_TIMEOUT": "250ms",
	}
}

func loadCustodyEnv(env map[string]string) (ClientConfig, bool, error) {
	return LoadClientConfig(func(name string) (string, bool) { value, ok := env[name]; return value, ok })
}

func TestCustodyClientConfigCompleteOrAbsent(t *testing.T) {
	_, enabled, err := loadCustodyEnv(nil)
	require.NoError(t, err)
	require.False(t, enabled)
	config, enabled, err := loadCustodyEnv(completeCustodyEnv())
	require.NoError(t, err)
	require.True(t, enabled)
	require.Equal(t, "127.0.0.1:12345", config.Address)
	require.Equal(t, "localhost", config.ServerName)
	require.Equal(t, 250*time.Millisecond, config.HeartbeatTimeout)
	for name := range completeCustodyEnv() {
		t.Run(name, func(t *testing.T) {
			for _, blank := range []string{"", "  "} {
				env := completeCustodyEnv()
				env[name] = blank
				_, _, err := loadCustodyEnv(env)
				require.Error(t, err)
			}
			env := completeCustodyEnv()
			delete(env, name)
			_, _, err := loadCustodyEnv(env)
			require.Error(t, err)
		})
	}
}

func TestCustodyClientConfigRejectsUnsafeValues(t *testing.T) {
	for _, tc := range []struct{ name, value string }{
		{"USER_CUSTODY_ADDR", "https://localhost:1234"}, {"USER_CUSTODY_ADDR", "user:pass@localhost:1234"},
		{"USER_CUSTODY_ADDR", "0.0.0.0:1234"}, {"USER_CUSTODY_ADDR", ":1234"},
		{"USER_CUSTODY_TLS_SERVER_NAME", " localhost"},
		{"USER_CUSTODY_ACTIVE_KID", "../current"}, {"USER_CUSTODY_ACTIVE_KID", strings.Repeat("a", 65)},
		{"USER_CUSTODY_HEARTBEAT_TIMEOUT", "0s"}, {"USER_CUSTODY_HEARTBEAT_TIMEOUT", "-1s"},
		{"USER_CUSTODY_HEARTBEAT_TIMEOUT", "11s"}, {"USER_CUSTODY_HEARTBEAT_TIMEOUT", "invalid"},
	} {
		t.Run(tc.name+"/"+tc.value, func(t *testing.T) {
			env := completeCustodyEnv()
			env[tc.name] = tc.value
			_, _, err := loadCustodyEnv(env)
			require.Error(t, err)
		})
	}
}

func testPKCS8Key(t *testing.T, bits int) (*rsa.PrivateKey, []byte) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, bits)
	require.NoError(t, err)
	der, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	return key, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
}

func writeCustodyKey(t *testing.T, dir, name string, data []byte) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), data, 0600))
}
