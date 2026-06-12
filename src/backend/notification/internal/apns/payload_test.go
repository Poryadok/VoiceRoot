package apns_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"voice/backend/notification/internal/apns"
	"voice/backend/notification/internal/push"
)

func TestBuildNotification_AlertAndData(t *testing.T) {
	n, err := apns.BuildNotification("com.voice.app", "abc123", push.Payload{
		Title: "New message",
		Body:  "Hello",
		Data: map[string]string{
			"type":    "new_message",
			"chat_id": "chat-1",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "abc123", n.DeviceToken)
	require.Equal(t, "com.voice.app", n.Topic)

	raw, err := apns.PayloadJSON(n)
	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(raw, &doc))
	aps := doc["aps"].(map[string]any)
	alert := aps["alert"].(map[string]any)
	require.Equal(t, "New message", alert["title"])
	require.Equal(t, "Hello", alert["body"])
	require.Equal(t, "new_message", doc["type"])
	require.Equal(t, "chat-1", doc["chat_id"])
}

func TestBuildNotification_CollapseIDAndCounter(t *testing.T) {
	n, err := apns.BuildNotification("com.voice.app", "tok", push.Payload{
		Title:       "Chat",
		Body:        "Vasya",
		CollapseTag: "push:group:profile:chat-1",
		Counter:     5,
		Data:        map[string]string{"type": "new_message"},
	})
	require.NoError(t, err)
	require.Equal(t, "push:group:profile:chat-1", n.CollapseID)

	raw, err := apns.PayloadJSON(n)
	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(raw, &doc))
	aps := doc["aps"].(map[string]any)
	alert := aps["alert"].(map[string]any)
	require.Equal(t, "Vasya and 4 more messages", alert["body"])
	require.Equal(t, "push:group:profile:chat-1", aps["thread-id"])
}
