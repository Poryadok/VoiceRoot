package fcm_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"voice/backend/notification/internal/fcm"
)

func TestConfigFromEnv_Valid(t *testing.T) {
	t.Setenv("FCM_CREDENTIALS_JSON", `{"project_id":"voice-test","private_key":"x","client_email":"a@b.c"}`)
	cfg, ok := fcm.ConfigFromEnv()
	require.True(t, ok)
	require.Equal(t, "voice-test", cfg.ProjectID)
	require.Contains(t, string(cfg.CredentialsJSON), "voice-test")
}

func TestConfigFromEnv_Missing(t *testing.T) {
	os.Unsetenv("FCM_CREDENTIALS_JSON")
	_, ok := fcm.ConfigFromEnv()
	require.False(t, ok)
}

func TestConfigFromEnv_InvalidJSON(t *testing.T) {
	t.Setenv("FCM_CREDENTIALS_JSON", "not-json")
	_, ok := fcm.ConfigFromEnv()
	require.False(t, ok)
}

func TestConfigFromEnv_MissingProjectID(t *testing.T) {
	t.Setenv("FCM_CREDENTIALS_JSON", `{"type":"service_account"}`)
	_, ok := fcm.ConfigFromEnv()
	require.False(t, ok)
}
