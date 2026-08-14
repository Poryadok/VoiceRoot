package fcm_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"voice/backend/notification/internal/fcm"
)

func TestConfigFromEnv_DeployNames(t *testing.T) {
	t.Setenv("FCM_CREDENTIALS_JSON", "")
	t.Setenv("FCM_PROJECT_ID", "voice-deploy")
	t.Setenv("FCM_SERVICE_ACCOUNT_JSON", `{"project_id":"voice-deploy","private_key":"x","client_email":"a@b.c"}`)
	cfg, ok := fcm.ConfigFromEnv()
	require.True(t, ok)
	require.Equal(t, "voice-deploy", cfg.ProjectID)
}
