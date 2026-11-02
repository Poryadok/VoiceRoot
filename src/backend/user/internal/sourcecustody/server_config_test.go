package sourcecustody

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func serverEnv() map[string]string {
	return map[string]string{
		"CUSTODY_CONTROL_DATABASE_URL":   "postgres://user:pass@127.0.0.1:5432/control?sslmode=verify-full&sslrootcert=ca.pem",
		"CUSTODY_LISTEN_ADDR":            "127.0.0.1:12345",
		"CUSTODY_TLS_CERT_FILE":          "server.pem",
		"CUSTODY_TLS_KEY_FILE":           "server.key",
		"CUSTODY_TLS_CLIENT_CA_FILE":     "client-ca.pem",
		"CUSTODY_USER_JWKS_URL":          "https://localhost:9443/jwks",
		"CUSTODY_USER_JWKS_CA_FILE":      "jwks-ca.pem",
		"CUSTODY_REDIS_ADDR":             "127.0.0.1:6379",
		"CUSTODY_REDIS_TLS_CA_FILE":      "redis-ca.pem",
		"CUSTODY_REDIS_TLS_SERVER_NAME":  "localhost",
		"CUSTODY_EPOCH":                  "1",
		"CUSTODY_SOURCE_ID":              "source-1",
		"CUSTODY_HISTORY_GENERATION":     "4e188484-8dac-457b-9131-c4ef52245791",
		"CUSTODY_USER_SOURCE_GENERATION": "93f666d1-4f9c-4c7a-9765-47f8d798c19e",
		"CUSTODY_KEYSET_DIGEST":          strings.Repeat("01", 32),
		"CUSTODY_HEARTBEAT":              "1s",
	}
}

func loadServerEnv(env map[string]string) (ServerConfig, error) {
	return LoadServerConfig(func(key string) (string, bool) { value, ok := env[key]; return value, ok })
}

func TestCustodyServerConfigRequiresCompleteAndSafeInputs(t *testing.T) {
	_, err := loadServerEnv(serverEnv())
	require.NoError(t, err)
	for key := range serverEnv() {
		t.Run(key, func(t *testing.T) {
			values := serverEnv()
			delete(values, key)
			_, err := loadServerEnv(values)
			require.Error(t, err)
			values[key] = " "
			_, err = loadServerEnv(values)
			require.Error(t, err)
		})
	}
	for _, tc := range []struct{ key, value string }{
		{"CUSTODY_CONTROL_DATABASE_URL", "postgres://user:pass@localhost:5432/control?sslmode=disable"},
		{"CUSTODY_CONTROL_DATABASE_URL", "postgres://user:pass@localhost:5432/control?sslmode=verify-full&sslmode=disable&sslrootcert=ca.pem"},
		{"CUSTODY_CONTROL_DATABASE_URL", "postgres://user:pass@localhost,other:5432/control?sslmode=verify-full&sslrootcert=ca.pem"},
		{"CUSTODY_USER_JWKS_URL", "http://localhost/jwks"},
		{"CUSTODY_USER_JWKS_URL", "https://user:password@localhost/jwks"},
		{"CUSTODY_USER_JWKS_URL", "https://localhost/jwks?token=secret"},
		{"CUSTODY_REDIS_ADDR", "localhost:bad"},
		{"CUSTODY_KEYSET_DIGEST", strings.Repeat("00", 32)},
		{"CUSTODY_KEYSET_DIGEST", strings.Repeat("AA", 32)},
		{"CUSTODY_EPOCH", "0"},
		{"CUSTODY_HEARTBEAT", "6s"},
	} {
		t.Run(tc.key+"/"+tc.value, func(t *testing.T) {
			values := serverEnv()
			values[tc.key] = tc.value
			_, err := loadServerEnv(values)
			require.Error(t, err)
		})
	}
}
