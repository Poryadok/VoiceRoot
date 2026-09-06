package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	chatv1 "voice.app/voice/chat/v1"
	spacev1 "voice.app/voice/space/v1"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
)

func TestCreateComposeSpaceChannelDecodesCanonicalLinkedChat(t *testing.T) {
	const (
		spaceID  = "space-helper-test"
		nodeID   = "node-helper-test"
		linkedID = "channel-helper-test"
		token    = "test-access-token"
	)

	payload, err := (protojson.MarshalOptions{UseProtoNames: true}).Marshal(&spacev1.UpsertTreeNodeResponse{
		SpaceTreeNode: &spacev1.SpaceTreeNode{
			Id:      nodeID,
			SpaceId: spaceID,
			Kind:    "text_chat",
			LinkedChat: &chatv1.ChatRef{
				Id: linkedID,
			},
		},
	})
	require.NoError(t, err)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer "+token, r.Header.Get("Authorization"))
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/spaces/"+spaceID+"/chats":
			var request struct {
				Type string `json:"type"`
				Name string `json:"name"`
			}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
			require.Equal(t, "CHAT_TYPE_CHANNEL", request.Type)
			require.Equal(t, "canonical channel", request.Name)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, writeErr := w.Write(payload)
			require.NoError(t, writeErr)
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/spaces/"+spaceID+"/tree":
			// Deliberately empty: the legacy linked_chat_id fallback must not
			// hide a linked_chat returned by the canonical response.
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, writeErr := w.Write([]byte(`{"categories":[],"nodes":[]}`))
			require.NoError(t, writeErr)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	got := createComposeSpaceChannel(t, server.Client(), server.URL, token, spaceID, "canonical channel")
	require.Equal(t, linkedID, got)
}
