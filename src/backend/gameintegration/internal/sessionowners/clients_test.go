package sessionowners

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadFromEnvDisablesWhenAbsentAndRejectsPartialOwnerIdentity(t *testing.T) {
	values := map[string]string{}
	getenv := func(name string) string { return values[name] }
	config, enabled, err := LoadFromEnv(getenv)
	require.NoError(t, err)
	require.False(t, enabled)
	require.Empty(t, config)

	values["GIS_ROLE_GRPC_ADDR"] = "role:9091"
	_, enabled, err = LoadFromEnv(getenv)
	require.ErrorContains(t, err, "configured together")
	require.True(t, enabled)

	values["GIS_CHAT_GRPC_ADDR"] = "chat:9092"
	values["GIS_CHAT_TLS_CA_FILE"] = "/run/ca.crt"
	values["GIS_CHAT_CLIENT_CERT_FILE"] = "/run/chat-client.crt"
	values["GIS_CHAT_CLIENT_KEY_FILE"] = "/run/chat-client.key"
	_, enabled, err = LoadFromEnv(getenv)
	require.ErrorContains(t, err, "configured together")
	require.True(t, enabled)
}
