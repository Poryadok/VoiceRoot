package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"voice/backend/pkg/principal"
)

func TestGatewayPrincipalIssuerConfig_LoadsCurrentAndNextKeys(t *testing.T) {
	dir := t.TempDir()
	writeGatewayPrincipalKey(t, dir, "current")
	writeGatewayPrincipalKey(t, dir, "next")
	t.Setenv("GATEWAY_PRINCIPAL_SIGNING_KEYS_DIR", dir)
	t.Setenv("GATEWAY_PRINCIPAL_ACTIVE_KID", "current")

	config, err := loadGatewayConfigFromEnvChecked()
	require.NoError(t, err)
	require.NotNil(t, config.principalIssuer)
	require.Len(t, config.principalJWKS.Keys, 2)
	require.Equal(t, "current", config.principalJWKS.Keys[0].Kid)
	require.Equal(t, "next", config.principalJWKS.Keys[1].Kid)

	document, err := json.Marshal(config.principalJWKS)
	require.NoError(t, err)
	keys, err := principal.ParseJWKS(document)
	require.NoError(t, err)
	token, err := config.principalIssuer.IssueService(principal.ServiceInput{
		Audience: "role", RPC: "/voice.role.v1.RoleService/CheckPermission", RequestID: "request-1",
		RequestHash: "sha256:0000000000000000000000000000000000000000000000000000000000000000",
	})
	require.NoError(t, err)
	_, err = principal.VerifyService(context.Background(), token, principal.VerifyConfig{
		ExpectedIssuer: "gateway", ExpectedAudience: "role", ExpectedRPC: "/voice.role.v1.RoleService/CheckPermission", ExpectedRequestID: "request-1",
		ExpectedRequestHash: "sha256:0000000000000000000000000000000000000000000000000000000000000000",
		KeyResolver:         func(_ context.Context, issuer, kid string) (*rsa.PublicKey, error) { return keys[kid], nil },
	})
	require.NoError(t, err)
}

func TestGatewayPrincipalIssuerConfig_FailsClosedForInvalidConfiguration(t *testing.T) {
	cases := []struct {
		name string
		dir  func(t *testing.T) string
		kid  string
	}{
		{name: "active kid without directory", dir: func(t *testing.T) string { return "" }, kid: "current"},
		{name: "directory without kid", dir: func(t *testing.T) string { return t.TempDir() }, kid: ""},
		{name: "missing current pem", dir: func(t *testing.T) string { return t.TempDir() }, kid: "current"},
		{name: "invalid active kid", dir: func(t *testing.T) string { return t.TempDir() }, kid: "../current"},
		{name: "invalid pem", dir: func(t *testing.T) string {
			dir := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(dir, "current.pem"), []byte("not a pem"), 0o600))
			return dir
		}, kid: "current"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("GATEWAY_PRINCIPAL_SIGNING_KEYS_DIR", tc.dir(t))
			t.Setenv("GATEWAY_PRINCIPAL_ACTIVE_KID", tc.kid)
			_, err := loadGatewayConfigFromEnvChecked()
			require.Error(t, err)
		})
	}
}

func TestGatewayPrincipalIssuerConfig_RejectsLegacyAndNonRotationDirectories(t *testing.T) {
	t.Run("legacy config", func(t *testing.T) {
		t.Setenv("S2S_SIGNING_KID", "current")
		_, err := loadGatewayConfigFromEnvChecked()
		require.Error(t, err)
	})
	t.Run("legacy pem config", func(t *testing.T) {
		t.Setenv("S2S_SIGNING_KEY_PEM", "private-key")
		_, err := loadGatewayConfigFromEnvChecked()
		require.Error(t, err)
	})
	for _, count := range []int{1, 3} {
		t.Run("key count", func(t *testing.T) {
			dir := t.TempDir()
			for i := 0; i < count; i++ {
				writeGatewayPrincipalKey(t, dir, "key"+string(rune('a'+i)))
			}
			t.Setenv("GATEWAY_PRINCIPAL_SIGNING_KEYS_DIR", dir)
			t.Setenv("GATEWAY_PRINCIPAL_ACTIVE_KID", "keya")
			_, err := loadGatewayConfigFromEnvChecked()
			require.Error(t, err)
		})
	}
}

func TestGatewayPrincipalJWKSWellKnown(t *testing.T) {
	dir := t.TempDir()
	writeGatewayPrincipalKey(t, dir, "current")
	writeGatewayPrincipalKey(t, dir, "next")
	t.Setenv("GATEWAY_PRINCIPAL_SIGNING_KEYS_DIR", dir)
	t.Setenv("GATEWAY_PRINCIPAL_ACTIVE_KID", "current")
	config, err := loadGatewayConfigFromEnvChecked()
	require.NoError(t, err)

	response := performRequest(newGateway(config), http.MethodGet, "/.well-known/voice-principal-jwks.json", "", nil)
	require.Equal(t, http.StatusOK, response.Code)
	require.Equal(t, "application/jwk-set+json", response.Header().Get("Content-Type"))
	var document principalJWKS
	decodeJSON(t, response.Body, &document)
	require.Len(t, document.Keys, 2)
	for _, key := range document.Keys {
		require.Equal(t, "RSA", key.Kty)
		require.Equal(t, "sig", key.Use)
		require.Equal(t, "RS256", key.Alg)
		require.NotEmpty(t, key.Kid)
		require.NotEmpty(t, key.N)
		require.NotEmpty(t, key.E)
	}

	notAllowed := performRequest(newGateway(config), http.MethodPost, "/.well-known/voice-principal-jwks.json", "", nil)
	require.Equal(t, http.StatusNotFound, notAllowed.Code)
}

func writeGatewayPrincipalKey(t *testing.T, dir, kid string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	encoded, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, kid+".pem"), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: encoded}), 0o600))
}
