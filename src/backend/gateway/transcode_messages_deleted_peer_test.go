package main

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	chatv1 "voice.app/voice/chat/v1"
	messagingv1 "voice.app/voice/messaging/v1"
)

// TestTranscodeMessagesGet_DeletedDMPeerPreservesResponseAndOmitsClientChatType
// protects the public history boundary from making chat type authoritative.
// Messaging resolves the chat type server-side and returns the deleted-DM state
// alongside the ordinary persisted history; Gateway must pass both through.
func TestTranscodeMessagesGet_DeletedDMPeerPreservesResponseAndOmitsClientChatType(t *testing.T) {
	t.Parallel()

	grpcRec := &recordingDeletedPeerMessages{}
	conn, cleanup := startBufconnMessagingConn(t, grpcRec)
	t.Cleanup(cleanup)

	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{messaging: messagingv1.NewMessagingServiceClient(conn)}},
	})

	resp := performRequest(h, http.MethodGet, "/api/v1/messages?chat_id=dm-deleted-1", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})

	require.Equal(t, http.StatusOK, resp.Code, "body=%s", resp.Body.String())
	require.NotNil(t, grpcRec.get)
	require.Equal(t, "dm-deleted-1", grpcRec.get.GetChat().GetId())
	require.Nil(t, grpcRec.get.GetChat().Type, "public id-only history requests must not invent a chat type")

	var body struct {
		DmPeerState string `json:"dm_peer_state"`
		MessageList struct {
			Messages []struct {
				ID string `json:"id"`
			} `json:"messages"`
			NextCursor string `json:"next_cursor"`
			HasMore    bool   `json:"has_more"`
		} `json:"message_list"`
	}
	decodeJSON(t, resp.Body, &body)
	require.Equal(t, "DM_PEER_STATE_DELETED", body.DmPeerState)
	require.Len(t, body.MessageList.Messages, 1)
	require.Equal(t, "message-existing-1", body.MessageList.Messages[0].ID)
	require.Equal(t, "cursor-existing-2", body.MessageList.NextCursor)
	require.True(t, body.MessageList.HasMore)
}

// TestTranscodeMessagesSend_ForwardsClientProvidedChatType documents the trust
// boundary: a type supplied in public protobuf JSON is transport data only.
// Gateway does not resolve or enforce it; Messaging must use an authoritative
// server-side chat lookup before applying DM-only deleted-account policy.
func TestTranscodeMessagesSend_ForwardsClientProvidedChatType(t *testing.T) {
	t.Parallel()

	grpcRec := &recordingDeletedPeerMessages{}
	conn, cleanup := startBufconnMessagingConn(t, grpcRec)
	t.Cleanup(cleanup)

	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{messaging: messagingv1.NewMessagingServiceClient(conn)}},
	})

	resp := performRequest(h, http.MethodPost, "/api/v1/messages/send", `{"chat":{"id":"dm-deleted-1","type":"CHAT_TYPE_CHANNEL"},"content":"forged type"}`, map[string]string{
		"Authorization": "Bearer valid-user-token",
	})

	require.Equal(t, http.StatusOK, resp.Code, "body=%s", resp.Body.String())
	require.NotNil(t, grpcRec.send)
	require.Equal(t, "dm-deleted-1", grpcRec.send.GetChat().GetId())
	require.NotNil(t, grpcRec.send.GetChat().Type)
	require.Equal(t, chatv1.ChatType_CHAT_TYPE_CHANNEL, grpcRec.send.GetChat().GetType())
}

type recordingDeletedPeerMessages struct {
	messagingv1.UnimplementedMessagingServiceServer
	get  *messagingv1.GetMessagesRequest
	send *messagingv1.SendMessageRequest
}

func (s *recordingDeletedPeerMessages) GetMessages(_ context.Context, req *messagingv1.GetMessagesRequest) (*messagingv1.GetMessagesResponse, error) {
	s.get = req
	state := messagingv1.DmPeerState_DM_PEER_STATE_DELETED
	return &messagingv1.GetMessagesResponse{
		MessageList: &messagingv1.MessageList{
			Messages: []*messagingv1.Message{{
				Id:   "message-existing-1",
				Chat: &chatv1.ChatRef{Id: req.GetChat().GetId()},
			}},
			NextCursor: "cursor-existing-2",
			HasMore:    true,
		},
		DmPeerState: &state,
	}, nil
}

func (s *recordingDeletedPeerMessages) SendMessage(_ context.Context, req *messagingv1.SendMessageRequest) (*messagingv1.SendMessageResponse, error) {
	s.send = req
	return &messagingv1.SendMessageResponse{Message: &messagingv1.Message{
		Id:      "message-sent-1",
		Chat:    req.GetChat(),
		Content: req.GetContent(),
	}}, nil
}
