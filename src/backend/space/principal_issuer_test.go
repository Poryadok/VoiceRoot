package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"

	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"voice/backend/pkg/principal"
)

func TestSpacePrincipalIssuerConfig_LoadsCurrentAndNextKeys(t *testing.T) {
	dir := t.TempDir()
	writeSpacePrincipalKey(t, dir, "current")
	writeSpacePrincipalKey(t, dir, "next")
	t.Setenv("SPACE_PRINCIPAL_SIGNING_KEYS_DIR", dir)
	t.Setenv("SPACE_PRINCIPAL_ACTIVE_KID", "current")

	issuer, jwks, err := loadSpacePrincipalIssuerFromEnv()
	require.NoError(t, err)
	require.NotNil(t, issuer)
	require.Len(t, jwks.Keys, 2)
	require.Equal(t, "current", jwks.Keys[0].Kid)
	require.Equal(t, "next", jwks.Keys[1].Kid)

	document, err := json.Marshal(jwks)
	require.NoError(t, err)
	keys, err := principal.ParseJWKS(document)
	require.NoError(t, err)
	token, err := issuer.IssueService(principal.ServiceInput{
		Audience: "role", RPC: "/voice.role.v1.RoleService/ApplyOwnershipTransfer", RequestID: "request-1",
		RequestHash: "sha256:0000000000000000000000000000000000000000000000000000000000000000",
	})
	require.NoError(t, err)
	_, err = principal.VerifyService(context.Background(), token, principal.VerifyConfig{
		ExpectedIssuer: "space", ExpectedAudience: "role", ExpectedRPC: "/voice.role.v1.RoleService/ApplyOwnershipTransfer", ExpectedRequestID: "request-1",
		ExpectedRequestHash: "sha256:0000000000000000000000000000000000000000000000000000000000000000",
		KeyResolver:         func(_ context.Context, issuer, kid string) (*rsa.PublicKey, error) { return keys[kid], nil },
	})
	require.NoError(t, err)
}

func TestSpacePrincipalIssuerConfig_LoadsKubernetesProjectedSecretLayout(t *testing.T) {
	dir := t.TempDir()
	dataDir := filepath.Join(dir, "..data-123")
	require.NoError(t, os.Mkdir(dataDir, 0o700))
	writeSpacePrincipalKey(t, dataDir, "current")
	writeSpacePrincipalKey(t, dataDir, "next")
	for _, kid := range []string{"current", "next"} {
		require.NoError(t, os.Symlink(filepath.Join("..data-123", kid+".pem"), filepath.Join(dir, kid+".pem")))
	}
	// Kubernetes itself exposes `..data` as a service symlink beside the key links.
	require.NoError(t, os.Symlink("..data-123", filepath.Join(dir, "..data")))
	t.Setenv("SPACE_PRINCIPAL_SIGNING_KEYS_DIR", dir)
	t.Setenv("SPACE_PRINCIPAL_ACTIVE_KID", "current")

	issuer, jwks, err := loadSpacePrincipalIssuerFromEnv()
	require.NoError(t, err)
	require.NotNil(t, issuer)
	require.Len(t, jwks.Keys, 2)
}

func TestSpacePrincipalIssuerConfig_FailsClosedForInvalidConfiguration(t *testing.T) {
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
			t.Setenv("SPACE_PRINCIPAL_SIGNING_KEYS_DIR", tc.dir(t))
			t.Setenv("SPACE_PRINCIPAL_ACTIVE_KID", tc.kid)
			_, _, err := loadSpacePrincipalIssuerFromEnv()
			require.Error(t, err)
		})
	}
}

func TestSpacePrincipalIssuerConfig_RejectsLegacyAndNonRotationDirectories(t *testing.T) {
	t.Run("legacy config", func(t *testing.T) {
		t.Setenv("S2S_SIGNING_KID", "current")
		_, _, err := loadSpacePrincipalIssuerFromEnv()
		require.Error(t, err)
	})
	t.Run("legacy pem config", func(t *testing.T) {
		t.Setenv("S2S_SIGNING_KEY_PEM", "private-key")
		_, _, err := loadSpacePrincipalIssuerFromEnv()
		require.Error(t, err)
	})
	for _, count := range []int{1, 3} {
		t.Run("key count", func(t *testing.T) {
			dir := t.TempDir()
			for i := 0; i < count; i++ {
				writeSpacePrincipalKey(t, dir, "key"+string(rune('a'+i)))
			}
			t.Setenv("SPACE_PRINCIPAL_SIGNING_KEYS_DIR", dir)
			t.Setenv("SPACE_PRINCIPAL_ACTIVE_KID", "keya")
			_, _, err := loadSpacePrincipalIssuerFromEnv()
			require.Error(t, err)
		})
	}
}

func TestSpacePrincipalIssuerConfig_RejectsUnsafeKeyMaterial(t *testing.T) {
	t.Run("duplicate public key", func(t *testing.T) {
		dir := t.TempDir()
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)
		writeSpacePrincipalKeyValue(t, dir, "current", key)
		writeSpacePrincipalKeyValue(t, dir, "peer", key)
		assertSpacePrincipalConfigRejected(t, dir)
	})
	t.Run("rsa below 2048 bits", func(t *testing.T) {
		dir := t.TempDir()
		key, err := rsa.GenerateKey(rand.Reader, 1024)
		require.NoError(t, err)
		writeSpacePrincipalKeyValue(t, dir, "current", key)
		writeSpacePrincipalKey(t, dir, "peer")
		assertSpacePrincipalConfigRejected(t, dir)
	})
	t.Run("encrypted pem", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "current.pem"), pem.EncodeToMemory(&pem.Block{Type: "ENCRYPTED PRIVATE KEY", Bytes: []byte("encrypted")}), 0o600))
		writeSpacePrincipalKey(t, dir, "peer")
		assertSpacePrincipalConfigRejected(t, dir)
	})
	t.Run("extra entry", func(t *testing.T) {
		dir := t.TempDir()
		writeSpacePrincipalKey(t, dir, "current")
		writeSpacePrincipalKey(t, dir, "peer")
		require.NoError(t, os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("not a key"), 0o600))
		assertSpacePrincipalConfigRejected(t, dir)
	})
}

func writeSpacePrincipalKey(t *testing.T, dir, kid string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	writeSpacePrincipalKeyValue(t, dir, kid, key)
}

func writeSpacePrincipalKeyValue(t *testing.T, dir, kid string, key *rsa.PrivateKey) {
	t.Helper()
	encoded, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, kid+".pem"), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: encoded}), 0o600))
}

func assertSpacePrincipalConfigRejected(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("SPACE_PRINCIPAL_SIGNING_KEYS_DIR", dir)
	t.Setenv("SPACE_PRINCIPAL_ACTIVE_KID", "current")
	_, _, err := loadSpacePrincipalIssuerFromEnv()
	require.Error(t, err)
}

func TestSpacePrincipalIssuerConfig_RejectsSymlinkOutsideSecretDirectory(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	writeSpacePrincipalKey(t, outside, "current")
	writeSpacePrincipalKey(t, dir, "peer")
	require.NoError(t, os.Symlink(filepath.Join(outside, "current.pem"), filepath.Join(dir, "current.pem")))
	assertSpacePrincipalConfigRejected(t, dir)
}

func TestSpacePrincipalIssuerConfig_RejectsPKCS1AndTrailingPrivateMaterial(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	encoded, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	valid := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: encoded})
	for _, tc := range []struct {
		name string
		data []byte
	}{
		{"PKCS1", pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})},
		{"extra PEM block", append(append([]byte{}, valid...), valid...)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeSpacePrincipalKey(t, dir, "peer")
			require.NoError(t, os.WriteFile(filepath.Join(dir, "current.pem"), tc.data, 0o600))
			assertSpacePrincipalConfigRejected(t, dir)
		})
	}
}

func TestSpacePrincipalIssuerConfig_JWKSPublishesOnlyPublicRotationKeys(t *testing.T) {
	dir := t.TempDir()
	writeSpacePrincipalKey(t, dir, "current")
	writeSpacePrincipalKey(t, dir, "next")
	t.Setenv("SPACE_PRINCIPAL_SIGNING_KEYS_DIR", dir)
	for _, kid := range []string{"current", "next"} {
		t.Setenv("SPACE_PRINCIPAL_ACTIVE_KID", kid)
		issuer, jwks, err := loadSpacePrincipalIssuerFromEnv()
		require.NoError(t, err)
		document, err := json.Marshal(jwks)
		require.NoError(t, err)
		var raw struct {
			Keys []map[string]any `json:"keys"`
		}
		require.NoError(t, json.Unmarshal(document, &raw))
		require.Len(t, raw.Keys, 2)
		for _, key := range raw.Keys {
			require.Equal(t, "RSA", key["kty"])
			require.Equal(t, "sig", key["use"])
			require.Equal(t, "RS256", key["alg"])
			for _, private := range []string{"d", "p", "q", "dp", "dq", "qi", "oth"} {
				require.NotContains(t, key, private)
			}
		}
		keys, err := principal.ParseJWKS(document)
		require.NoError(t, err)
		token, err := issuer.IssueService(principal.ServiceInput{Audience: "role", RPC: "/voice.role.v1.RoleService/ApplyOwnershipTransfer", RequestID: "rotate", RequestHash: "sha256:0000000000000000000000000000000000000000000000000000000000000000"})
		require.NoError(t, err)
		_, err = principal.VerifyService(context.Background(), token, principal.VerifyConfig{
			ExpectedIssuer: "space", ExpectedAudience: "role", ExpectedRPC: "/voice.role.v1.RoleService/ApplyOwnershipTransfer", ExpectedRequestID: "rotate", ExpectedRequestHash: "sha256:0000000000000000000000000000000000000000000000000000000000000000",
			KeyResolver: func(_ context.Context, issuer, resolvedKID string) (*rsa.PublicKey, error) {
				require.Equal(t, kid, resolvedKID)
				return keys[resolvedKID], nil
			},
		})
		require.NoError(t, err)
	}
}
