package apns_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"voice/backend/notification/internal/apns"
)

func TestConfigFromEnv_PrivateKeyAlias(t *testing.T) {
	t.Setenv("APNS_KEY_ID", "KEY123")
	t.Setenv("APNS_TEAM_ID", "TEAM123")
	t.Setenv("APNS_BUNDLE_ID", "com.voice.app")
	t.Setenv("APNS_AUTH_KEY", "")
	t.Setenv("APNS_PRIVATE_KEY", testAuthKeyPEM)
	t.Setenv("APNS_VOIP_TOPIC", "com.voice.app.voip")
	cfg, ok := apns.ConfigFromEnv()
	require.True(t, ok)
	require.Equal(t, "com.voice.app.voip", cfg.VoIPTopic)
}
