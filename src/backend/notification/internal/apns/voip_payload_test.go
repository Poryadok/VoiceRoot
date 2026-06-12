package apns_test

import (
	"testing"

	"github.com/sideshow/apns2"
	"github.com/stretchr/testify/require"

	"voice/backend/notification/internal/apns"
	"voice/backend/notification/internal/push"
)

func TestBuildVoIPNotification_TopicAndPushType(t *testing.T) {
	n, err := apns.BuildVoIPNotification("com.voice.app", "device-tok", push.Payload{
		Data: map[string]string{
			"type":                 "incoming_call",
			"room_id":              "room-1",
			"chat_id":              "chat-1",
			"initiator_profile_id": "init-1",
			"callee_profile_id":    "callee-1",
			"media_kind":           "audio",
			"livekit_room_name":    "lk-room",
			"expires_at":           "2026-06-12T12:00:00Z",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "device-tok", n.DeviceToken)
	require.Equal(t, "com.voice.app.voip", n.Topic)
	require.Equal(t, apns2.PushTypeVOIP, n.PushType)
	require.Equal(t, apns2.PriorityHigh, n.Priority)

	raw, err := apns.PayloadJSON(n)
	require.NoError(t, err)
	require.Contains(t, string(raw), `"type":"incoming_call"`)
	require.Contains(t, string(raw), `"room_id":"room-1"`)
}

func TestBuildVoIPNotification_RequiresToken(t *testing.T) {
	_, err := apns.BuildVoIPNotification("com.voice.app", "", push.Payload{})
	require.Error(t, err)
}

func TestBuildVoIPNotification_RequiresBundleID(t *testing.T) {
	_, err := apns.BuildVoIPNotification("", "tok", push.Payload{})
	require.Error(t, err)
}
