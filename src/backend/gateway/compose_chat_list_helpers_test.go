package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	chatv1 "voice.app/voice/chat/v1"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
)

func TestListComposeChatsDecodesCanonicalProtoJSON(t *testing.T) {
	const (
		chatID = "chat-canonical-protojson"
		// The canonical proto contract is int64; this catches narrowing to int32.
		unreadSize = int64(1 << 40)
	)

	payload, err := (protojson.MarshalOptions{UseProtoNames: true}).Marshal(&chatv1.ListChatsResponse{
		ChatList: &chatv1.ChatList{
			Items: []*chatv1.ChatListItem{{
				Chat:        &chatv1.Chat{Id: chatID},
				UnreadCount: unreadSize,
			}},
		},
	})
	require.NoError(t, err)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/api/v1/chats", r.URL.Path)
		require.Equal(t, "main", r.URL.Query().Get("inbox"))
		require.Equal(t, "Bearer test-access-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, writeErr := w.Write(payload)
		require.NoError(t, writeErr)
	}))
	defer server.Close()

	items := listComposeChats(t, server.Client(), server.URL, "test-access-token", "main")
	require.Len(t, items, 1)
	require.Equal(t, chatID, items[0].ChatID)
	require.Equal(t, unreadSize, int64(items[0].UnreadCount))
}
