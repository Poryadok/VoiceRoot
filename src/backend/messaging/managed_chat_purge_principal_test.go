package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"voice/backend/pkg/principal"
)

func TestLoadManagedChatPurgePrincipalRequiresCompleteRotationSetAndPublishesOnlyPublicKeys(t *testing.T) {
	t.Setenv("MESSAGING_PURGE_PRINCIPAL_SIGNING_KEYS_DIR", "")
	t.Setenv("MESSAGING_PURGE_PRINCIPAL_ACTIVE_KID", "")
	require.NoError(t, os.Unsetenv("MESSAGING_PURGE_PRINCIPAL_SIGNING_KEYS_DIR"))
	require.NoError(t, os.Unsetenv("MESSAGING_PURGE_PRINCIPAL_ACTIVE_KID"))
	_, _, enabled, err := loadManagedChatPurgePrincipal()
	require.NoError(t, err)
	require.False(t, enabled)

	directory := t.TempDir()
	for _, kid := range []string{"current", "next"} {
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)
		encoded, err := x509.MarshalPKCS8PrivateKey(key)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(directory, kid+".pem"), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: encoded}), 0o600))
	}
	t.Setenv("MESSAGING_PURGE_PRINCIPAL_SIGNING_KEYS_DIR", directory)
	t.Setenv("MESSAGING_PURGE_PRINCIPAL_ACTIVE_KID", "current")
	issuer, jwks, enabled, err := loadManagedChatPurgePrincipal()
	require.NoError(t, err)
	require.True(t, enabled)
	require.NotNil(t, issuer)
	require.NotNil(t, jwks)

	response := httptest.NewRecorder()
	jwks.ServeHTTP(response, httptest.NewRequest("GET", "/.well-known/principal-jwks.json", nil))
	require.Equal(t, 200, response.Code)
	keys, err := principal.ParseJWKS(response.Body.Bytes())
	require.NoError(t, err)
	require.Len(t, keys, 2)
	require.NotNil(t, keys["current"])
	require.NotNil(t, keys["next"])
}

func TestLoadManagedChatPurgePrincipalRejectsPartialOrInvalidConfiguration(t *testing.T) {
	t.Setenv("MESSAGING_PURGE_PRINCIPAL_SIGNING_KEYS_DIR", t.TempDir())
	t.Setenv("MESSAGING_PURGE_PRINCIPAL_ACTIVE_KID", "current")
	_, _, enabled, err := loadManagedChatPurgePrincipal()
	require.True(t, enabled)
	require.Error(t, err)

	t.Setenv("MESSAGING_PURGE_PRINCIPAL_SIGNING_KEYS_DIR", "")
	_, _, enabled, err = loadManagedChatPurgePrincipal()
	require.True(t, enabled)
	require.Error(t, err)
}
