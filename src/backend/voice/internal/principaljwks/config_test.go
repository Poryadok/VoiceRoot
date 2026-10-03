package principaljwks

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadFromEnvRequiresDistinctVoiceSignerAndTLSListener(t *testing.T) {
	values := map[string]string{}
	getenv := func(name string) string { return values[name] }
	config, enabled, err := LoadFromEnv(getenv)
	require.NoError(t, err)
	require.False(t, enabled)
	require.Empty(t, config)

	values["VOICE_PRINCIPAL_PRIVATE_KEY_FILE"] = filepath.Join(t.TempDir(), "voice.pem")
	_, enabled, err = LoadFromEnv(getenv)
	require.ErrorContains(t, err, "configured together")
	require.True(t, enabled)

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(values["VOICE_PRINCIPAL_PRIVATE_KEY_FILE"],
		pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}), 0600))
	values["VOICE_PRINCIPAL_KID"] = "voice-phase0"
	values["VOICE_PRINCIPAL_JWKS_TLS_CERT_FILE"] = "/run/secrets/voice-jwks.crt"
	values["VOICE_PRINCIPAL_JWKS_TLS_KEY_FILE"] = "/run/secrets/voice-jwks-tls.key"
	_, enabled, err = LoadFromEnv(getenv)
	require.ErrorContains(t, err, "next")
	require.True(t, enabled)
	nextKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	nextPath := filepath.Join(t.TempDir(), "voice-next.pem")
	require.NoError(t, os.WriteFile(nextPath,
		pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(nextKey)}), 0600))
	values["VOICE_PRINCIPAL_NEXT_PRIVATE_KEY_FILE"] = nextPath
	values["VOICE_PRINCIPAL_NEXT_KID"] = "voice-next"
	config, enabled, err = LoadFromEnv(getenv)
	require.NoError(t, err)
	require.True(t, enabled)
	require.Equal(t, ":8443", config.ListenAddr)
	require.Equal(t, "voice-phase0", config.KeyID)
	require.Equal(t, key.PublicKey, config.SigningKey.PublicKey)
	require.Equal(t, "voice-next", config.NextKeyID)
	require.Equal(t, nextKey.PublicKey, config.NextSigningKey.PublicKey)
	values["VOICE_PRINCIPAL_NEXT_KID"] = values["VOICE_PRINCIPAL_KID"]
	_, _, err = LoadFromEnv(getenv)
	require.ErrorContains(t, err, "distinct")
}
