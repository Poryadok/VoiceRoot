package apns_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"voice/backend/notification/internal/apns"
)

func TestConfigFromEnv_Missing(t *testing.T) {
	t.Setenv("APNS_KEY_ID", "")
	t.Setenv("APNS_TEAM_ID", "")
	t.Setenv("APNS_BUNDLE_ID", "")
	t.Setenv("APNS_AUTH_KEY", "")
	_, ok := apns.ConfigFromEnv()
	require.False(t, ok)
}

func TestConfigFromEnv_Present(t *testing.T) {
	t.Setenv("APNS_KEY_ID", "KEY123")
	t.Setenv("APNS_TEAM_ID", "TEAM123")
	t.Setenv("APNS_BUNDLE_ID", "com.voice.app")
	t.Setenv("APNS_AUTH_KEY", testAuthKeyPEM)
	t.Setenv("APNS_PRODUCTION", "false")
	cfg, ok := apns.ConfigFromEnv()
	require.True(t, ok)
	require.Equal(t, "KEY123", cfg.KeyID)
	require.Equal(t, "TEAM123", cfg.TeamID)
	require.Equal(t, "com.voice.app", cfg.BundleID)
	require.False(t, cfg.Production)
}

const testAuthKeyPEM = `-----BEGIN PRIVATE KEY-----
MIGHAgEAMBMGByqGSM49AgEGCCqGSM49AwEHBG0wawIBAQQg7eht6v9I0fswD0S3
Z0ZjVY9m0aHGuFxgpG+aTKlGujChRANCAASI4R0Yl7HfCV6k3Hxq0Q9s8T0o1Y2m
3KvQz8J0n0Yl7HfCV6k3Hxq0Q9s8T0o1Y2m3KvQz8J0n0Y
-----END PRIVATE KEY-----`
