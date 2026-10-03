package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"voice/backend/messaging/internal/store"
	"voice/backend/pkg/integrationtest"
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

func TestManagedChatPurgeRuntimeSchemaGatePrecedesTLSAndListener(t *testing.T) {
	if testing.Short() {
		t.Skip("requires isolated PostgreSQL")
	}
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "messaging_db", "")
	directory := t.TempDir()
	for _, kid := range []string{"current", "next"} {
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)
		encoded, err := x509.MarshalPKCS8PrivateKey(key)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(directory, kid+".pem"), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: encoded}), 0o600))
	}
	t.Setenv(managedChatPurgeSigningKeysDirEnv, directory)
	t.Setenv(managedChatPurgeActiveKIDEnv, "current")
	for name, value := range map[string]string{
		"MESSAGING_PURGE_PRINCIPAL_JWKS_LISTEN":      "127.0.0.1:0",
		"MESSAGING_PURGE_PRINCIPAL_TLS_CERT_FILE":    "missing-server.crt",
		"MESSAGING_PURGE_PRINCIPAL_TLS_KEY_FILE":     "missing-server.key",
		"MESSAGING_PURGE_PRINCIPAL_CLIENT_CA_FILE":   "missing-ca.crt",
		"MESSAGING_PURGE_PRINCIPAL_CLIENT_CERT_FILE": "missing-client.crt",
		"MESSAGING_PURGE_PRINCIPAL_CLIENT_KEY_FILE":  "missing-client.key",
		"FILE_MANAGED_CHAT_PURGE_GRPC_ADDR":          "file:9094",
		"FILE_MANAGED_CHAT_PURGE_TLS_CA_FILE":        "missing-file-ca.crt",
		"FILE_MANAGED_CHAT_PURGE_TLS_SERVER_NAME":    "file",
		"SEARCH_MANAGED_CHAT_PURGE_GRPC_ADDR":        "search:9094",
		"SEARCH_MANAGED_CHAT_PURGE_TLS_CA_FILE":      "missing-search-ca.crt",
		"SEARCH_MANAGED_CHAT_PURGE_TLS_SERVER_NAME":  "search",
	} {
		t.Setenv(name, value)
	}
	runtime, err := newManagedChatPurgeRuntime(ctx, pool)
	require.Nil(t, runtime)
	require.ErrorIs(t, err, store.ErrAttachmentIntentSchema, "unmigrated enabled runtime must fail before reading TLS files or opening its listener")
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
