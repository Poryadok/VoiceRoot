package apns_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"voice/backend/notification/internal/apns"
)

func TestNewHTTPSender_InvalidAuthKey(t *testing.T) {
	_, err := apns.NewHTTPSender(apns.Config{
		KeyID:      "KEY",
		TeamID:     "TEAM",
		BundleID:   "com.voice.app",
		AuthKeyPEM: "not-a-key",
	})
	require.Error(t, err)
}
