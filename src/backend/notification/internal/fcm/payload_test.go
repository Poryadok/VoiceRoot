package fcm_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"voice/backend/notification/internal/fcm"
	"voice/backend/notification/internal/push"
)

func TestBuildFCMMessage_DataAndNotification(t *testing.T) {
	msg, err := fcm.BuildFCMMessage("tok-1", push.Payload{
		Title: "Hi",
		Body:  "Preview",
		Data: map[string]string{
			"type":    "new_message",
			"chat_id": "chat-1",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "tok-1", msg.Token)
	require.Equal(t, "Hi", msg.Notification.Title)
	require.Equal(t, "Preview", msg.Notification.Body)
	require.Equal(t, "new_message", msg.Data["type"])
}

func TestBuildFCMMessage_CollapseTag(t *testing.T) {
	msg, err := fcm.BuildFCMMessage("tok-2", push.Payload{
		Body:        "One",
		CollapseTag: "push:group:abc:chat-1",
		Data:        map[string]string{"type": "mention"},
	})
	require.NoError(t, err)
	require.Equal(t, "push:group:abc:chat-1", msg.Android.CollapseKey)
	require.Equal(t, "push:group:abc:chat-1", msg.Android.Notification.Tag)
	require.Equal(t, "push:group:abc:chat-1", msg.Webpush.Headers["Topic"])
}

func TestBuildFCMMessage_CounterBody(t *testing.T) {
	msg, err := fcm.BuildFCMMessage("tok-3", push.Payload{
		Body:    "Latest",
		Counter: 3,
		Data:    map[string]string{"type": "new_message"},
	})
	require.NoError(t, err)
	require.Equal(t, "Latest and 2 more messages", msg.Notification.Body)
}

func TestBuildFCMMessage_EmptyToken(t *testing.T) {
	_, err := fcm.BuildFCMMessage("", push.Payload{Body: "x"})
	require.Error(t, err)
}
