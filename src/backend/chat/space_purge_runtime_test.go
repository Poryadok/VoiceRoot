package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSpacePurgeOwnerConfigRequiresCompleteMutualTLSConfiguration(t *testing.T) {
	config := spacePurgeOwnerConfigFromEnv(func(name string) string {
		return map[string]string{
			"CHAT_SPACE_PURGE_MESSAGING_GRPC_ADDR":       "messaging:9090",
			"CHAT_SPACE_PURGE_MESSAGING_TLS_CA_FILE":     "messaging-ca.pem",
			"CHAT_SPACE_PURGE_MESSAGING_TLS_SERVER_NAME": "messaging.internal",
			"CHAT_SPACE_PURGE_FILE_GRPC_ADDR":            "file:9090",
			"CHAT_SPACE_PURGE_FILE_TLS_CA_FILE":          "file-ca.pem",
			"CHAT_SPACE_PURGE_FILE_TLS_SERVER_NAME":      "file.internal",
			"CHAT_SPACE_PURGE_CLIENT_CERT_FILE":          "chat-client.pem",
			"CHAT_SPACE_PURGE_CLIENT_KEY_FILE":           "chat-client-key.pem",
		}[name]
	})
	require.True(t, config.configured())
	require.NoError(t, config.validate())

	config.FileServerName = ""
	require.ErrorContains(t, config.validate(), "CHAT_SPACE_PURGE_FILE_TLS_SERVER_NAME")
}

func TestSpacePurgeOwnerConfigIsNotConfiguredWhenUnset(t *testing.T) {
	config := spacePurgeOwnerConfigFromEnv(func(string) string { return "" })
	require.False(t, config.configured())
	require.Error(t, config.validate())
}
