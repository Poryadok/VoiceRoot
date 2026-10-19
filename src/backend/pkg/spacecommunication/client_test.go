package spacecommunication

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestClientConfigAbsentPartialAndExplicitEmpty(t *testing.T) {
	values := map[string]string{}
	lookup := func(key string) (string, bool) { v, ok := values[key]; return v, ok }
	_, enabled, err := LoadClientConfig(lookup, "TEST_")
	require.NoError(t, err)
	require.False(t, enabled)
	values["TEST_CLIENT_KEY_FILE"] = ""
	_, enabled, err = LoadClientConfig(lookup, "TEST_")
	require.Error(t, err)
	require.True(t, enabled, "explicit empty configuration must not become absent")
	for _, key := range []string{"GRPC_ADDR", "TLS_CA_FILE", "CLIENT_CERT_FILE", "CLIENT_KEY_FILE"} {
		values["TEST_"+key] = "configured"
	}
	_, enabled, err = LoadClientConfig(lookup, "TEST_")
	require.NoError(t, err)
	require.True(t, enabled)
}
