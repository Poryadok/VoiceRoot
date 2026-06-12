package apns_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/notification/internal/apns"
	"voice/backend/notification/internal/push"
	"voice/backend/notification/internal/store"
)

func TestHTTPSender_NilClient(t *testing.T) {
	sender := apns.NewHTTPSenderForTest(nil, "com.voice.app")
	err := sender.Send(context.Background(), uuid.New(), store.DeviceToken{Token: "tok"}, push.Payload{Body: "x"})
	require.Error(t, err)
}

func TestConfigFromEnv_AuthKeyPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "apns.p8")
	require.NoError(t, os.WriteFile(path, []byte(testAuthKeyPEM), 0o600))
	t.Setenv("APNS_KEY_ID", "KEY123")
	t.Setenv("APNS_TEAM_ID", "TEAM123")
	t.Setenv("APNS_BUNDLE_ID", "com.voice.app")
	t.Setenv("APNS_AUTH_KEY", "")
	t.Setenv("APNS_AUTH_KEY_PATH", path)
	cfg, ok := apns.ConfigFromEnv()
	require.True(t, ok)
	require.Equal(t, testAuthKeyPEM, cfg.AuthKeyPEM)
}

func TestAPNSErrorString(t *testing.T) {
	require.Equal(t, "apns: invalid token", apns.ErrInvalidToken.Error())
}
