package fcm_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"voice/backend/notification/internal/fcm"
)

func TestNoopSender_DegradedWithoutCredentials(t *testing.T) {
	os.Unsetenv("FCM_CREDENTIALS_JSON")
	_, ok := fcm.ConfigFromEnv()
	require.False(t, ok)
}
