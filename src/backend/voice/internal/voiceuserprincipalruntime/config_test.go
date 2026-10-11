package voiceuserprincipalruntime

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSharedJWKSSettingsDoNotEnableVoiceUserListener(t *testing.T) {
	const prefix = "VOICE_USER_PRINCIPAL_"
	keys := []string{prefix + "GRPC_LISTEN", prefix + "TLS_CERT_FILE", prefix + "TLS_KEY_FILE",
		prefix + "CLIENT_CA_FILE", prefix + "REPLAY_REDIS_ADDR", prefix + "REPLAY_REDIS_PASSWORD"}
	previous := make(map[string]string)
	wasSet := make(map[string]bool)
	for _, key := range keys {
		value, ok := os.LookupEnv(key)
		previous[key], wasSet[key] = value, ok
		_ = os.Unsetenv(key)
	}
	t.Cleanup(func() {
		for _, key := range keys {
			if wasSet[key] {
				_ = os.Setenv(key, previous[key])
			} else {
				_ = os.Unsetenv(key)
			}
		}
	})
	t.Setenv("S2S_JWKS_URLS_JSON", `{"gateway":"https://gateway.invalid/jwks"}`)
	t.Setenv("S2S_JWKS_CA_FILE", "unused")
	_, enabled, err := LoadFromEnv()
	require.NoError(t, err)
	require.False(t, enabled)
}

func TestConfigRejectsMissingAuthFloorAdjacentListenerMaterial(t *testing.T) {
	cfg := Config{ListenAddr: ":9092", ReplayAddr: "redis:6379", TLSCertFile: "voice.crt",
		TLSKeyFile: "voice.key", ClientCAFile: "gateway-ca.pem", JWKSURL: "https://gateway.invalid/jwks",
		JWKSCAFile: "root.pem", JWKSClientCertFile: "voice-client.crt", JWKSClientKeyFile: "voice-client.key",
		RefreshAfter: 30, HardExpiry: 60, UnknownKIDCooldown: 5}
	require.NoError(t, cfg.validate())
	cfg.ClientCAFile = ""
	require.Error(t, cfg.validate())
	cfg.ClientCAFile = "gateway-ca.pem"
	cfg.JWKSURL = "http://gateway.invalid/jwks"
	require.Error(t, cfg.validate())
}
