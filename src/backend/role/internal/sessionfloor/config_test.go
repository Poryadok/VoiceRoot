package sessionfloor

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRoleFloorConfigurationRequiresCompleteExplicitTLS(t *testing.T) {
	names := []string{"ROLE_AUTH_SESSION_FLOOR_GRPC_ADDR", "ROLE_AUTH_SESSION_FLOOR_TLS_CA_FILE", "ROLE_AUTH_SESSION_FLOOR_TLS_SERVER_NAME", "ROLE_AUTH_SESSION_FLOOR_CLIENT_CERT_FILE", "ROLE_AUTH_SESSION_FLOOR_CLIENT_KEY_FILE"}
	for _, name := range names {
		old, present := os.LookupEnv(name)
		t.Cleanup(func() {
			if present {
				_ = os.Setenv(name, old)
			} else {
				_ = os.Unsetenv(name)
			}
		})
		require.NoError(t, os.Unsetenv(name))
	}
	_, enabled, err := ConfigFromEnv()
	require.NoError(t, err)
	require.False(t, enabled)
	t.Setenv(names[0], "")
	_, enabled, err = ConfigFromEnv()
	require.Error(t, err, "an explicitly blank setting must not silently disable freshness")
	require.True(t, enabled)
	for i, value := range []string{"auth:9091", "ca.pem", "auth.internal", "client.pem", "client.key"} {
		t.Setenv(names[i], value)
	}
	config, enabled, err := ConfigFromEnv()
	require.NoError(t, err)
	require.True(t, enabled)
	issuer, _ := roleFloorIssuer(t)
	for _, name := range names {
		old := os.Getenv(name)
		t.Setenv(name, " "+old)
		_, enabled, err := ConfigFromEnv()
		require.Error(t, err)
		require.True(t, enabled)
		t.Setenv(name, old)
	}
	_, _, err = config.Dial(nil)
	require.Error(t, err)
	_, _, err = (Config{}).Dial(issuer)
	require.Error(t, err)
	_, _, err = config.Dial(issuer)
	require.Error(t, err, "missing trust/key files must fail before constructing a client")
}
