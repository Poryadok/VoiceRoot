package apns

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/sideshow/apns2"
	"github.com/stretchr/testify/require"

	"voice/backend/notification/internal/push"
	"voice/backend/notification/internal/store"
)

type topicCapture struct{ notification *apns2.Notification }

func (c *topicCapture) PushWithContext(_ apns2.Context, n *apns2.Notification) (*apns2.Response, error) {
	c.notification = n
	return &apns2.Response{StatusCode: 200}, nil
}

func TestVoIPSender_ConfiguredTopic(t *testing.T) {
	for _, tc := range []struct{ name, configured, want string }{
		{"explicit", "com.other.calls.voip", "com.other.calls.voip"},
		{"verbatim", "com.other.calls", "com.other.calls"},
		{"trimmed", "  com.other.calls.voip  ", "com.other.calls.voip"},
		{"fallback", "", "com.voice.app.voip"},
		{"blank", "  ", "com.voice.app.voip"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("APNS_KEY_ID", "test-key")
			t.Setenv("APNS_TEAM_ID", "test-team")
			t.Setenv("APNS_AUTH_KEY", "test-key-material")
			t.Setenv("APNS_BUNDLE_ID", "com.voice.app")
			t.Setenv("APNS_VOIP_TOPIC", tc.configured)
			cfg, ok := ConfigFromEnv()
			require.True(t, ok)
			client := &topicCapture{}
			sender := &HTTPVoIPSender{client: client, topic: voipTopic(cfg)}
			require.NoError(t, sender.Send(context.Background(), uuid.New(), store.DeviceToken{Token: "token"}, push.Payload{}))
			require.NotNil(t, client.notification)
			require.Equal(t, tc.want, client.notification.Topic)
			require.Equal(t, apns2.PushTypeVOIP, client.notification.PushType)
			require.NoError(t, NewHTTPSenderForTest(client, cfg.BundleID).Send(context.Background(), uuid.New(), store.DeviceToken{Token: "token"}, push.Payload{}))
			require.Equal(t, "com.voice.app", client.notification.Topic)
		})
	}
}
