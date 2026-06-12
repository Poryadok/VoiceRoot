package apns_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"voice/backend/notification/internal/apns"
	"voice/backend/notification/internal/push"
)

func TestBuildNotification_EmptyToken(t *testing.T) {
	_, err := apns.BuildNotification("com.voice.app", "", push.Payload{Body: "x"})
	require.Error(t, err)
}

func TestPayloadJSON_NilNotification(t *testing.T) {
	_, err := apns.PayloadJSON(nil)
	require.Error(t, err)
}

func TestBuildNotification_BodyOnly(t *testing.T) {
	n, err := apns.BuildNotification("com.voice.app", "tok", push.Payload{Body: "ping"})
	require.NoError(t, err)
	raw, err := apns.PayloadJSON(n)
	require.NoError(t, err)
	require.Contains(t, string(raw), "ping")
}
