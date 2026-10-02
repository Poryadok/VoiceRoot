package main

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestManagedChatPurgeRuntimeConfigRequiresCompleteMutualTLSOwnerSet(t *testing.T) {
	cfg := managedChatPurgeRuntimeConfig{}
	require.False(t, cfg.configured())
	require.Error(t, cfg.validate())
	partial := managedChatPurgeRuntimeConfig{FileAddr: "file:9094"}
	require.True(t, partial.configured())
	require.Error(t, partial.validate())
	cfg = managedChatPurgeRuntimeConfig{
		JWKSListen: ":8448", JWKSCertFile: "server.crt", JWKSKeyFile: "server.key", ClientCAFile: "client-ca.crt",
		ClientCertFile: "client.crt", ClientKeyFile: "client.key", FileAddr: "file:9094", FileCA: "file-ca.crt", FileServerName: "file",
		SearchAddr: "search:9094", SearchCA: "search-ca.crt", SearchServerName: "search",
	}
	require.NoError(t, cfg.validate())
	cfg.SearchServerName = " "
	require.Error(t, cfg.validate())
}

func TestManagedChatPurgeRuntimeRejectsOwnerConfigWithoutSigner(t *testing.T) {
	for _, name := range []string{
		"MESSAGING_PURGE_PRINCIPAL_JWKS_LISTEN", "MESSAGING_PURGE_PRINCIPAL_TLS_CERT_FILE",
		"MESSAGING_PURGE_PRINCIPAL_TLS_KEY_FILE", "MESSAGING_PURGE_PRINCIPAL_CLIENT_CA_FILE",
		"MESSAGING_PURGE_PRINCIPAL_CLIENT_CERT_FILE", "MESSAGING_PURGE_PRINCIPAL_CLIENT_KEY_FILE",
		"FILE_MANAGED_CHAT_PURGE_GRPC_ADDR", "FILE_MANAGED_CHAT_PURGE_TLS_CA_FILE",
		"FILE_MANAGED_CHAT_PURGE_TLS_SERVER_NAME", "SEARCH_MANAGED_CHAT_PURGE_GRPC_ADDR",
		"SEARCH_MANAGED_CHAT_PURGE_TLS_CA_FILE", "SEARCH_MANAGED_CHAT_PURGE_TLS_SERVER_NAME",
		managedChatPurgeSigningKeysDirEnv, managedChatPurgeActiveKIDEnv,
	} {
		t.Setenv(name, "")
	}
	require.NoError(t, os.Unsetenv(managedChatPurgeSigningKeysDirEnv))
	require.NoError(t, os.Unsetenv(managedChatPurgeActiveKIDEnv))
	t.Setenv("FILE_MANAGED_CHAT_PURGE_GRPC_ADDR", "file:9094")
	runtime, err := newManagedChatPurgeRuntime(context.Background(), nil)
	require.Nil(t, runtime)
	require.ErrorContains(t, err, "requires the signing-key configuration")
}
