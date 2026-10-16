package main

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	chatv1 "voice.app/voice/chat/v1"
	commonv1 "voice.app/voice/common/v1"
	messagingv1 "voice.app/voice/messaging/v1"
)

func TestTranscodeScheduledMessageLifecycle(t *testing.T) {
	t.Parallel()

	grpcRec := &recordingScheduledMessages{}
	conn, cleanup := startBufconnMessagingConn(t, grpcRec)
	t.Cleanup(cleanup)
	proxyCalled := false
	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{messaging: messagingv1.NewMessagingServiceClient(conn)}},
		restUpstreams: map[string]http.Handler{
			"messages": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				proxyCalled = true
				w.WriteHeader(http.StatusAccepted)
			}),
		},
	})
	headers := map[string]string{
		"Authorization":       "Bearer valid-user-token",
		"X-Voice-User-Id":     "forged-account",
		"X-Voice-Profile-Id":  "forged-profile",
	}

	list := performRequest(h, http.MethodGet, "/api/v1/messages/scheduled?chat_id=chat-1&cursor=page-2&page_size=17&profile_id=forged-profile", "", headers)
	require.Equal(t, http.StatusOK, list.Code, list.Body.String())
	require.Equal(t, "chat-1", grpcRec.list.GetChat().GetId())
	require.Equal(t, chatv1.ChatType_CHAT_TYPE_UNSPECIFIED, grpcRec.list.GetChat().GetType())
	require.Equal(t, &commonv1.CursorPageRequest{Cursor: "page-2", PageSize: 17}, grpcRec.list.GetPage())
	require.Equal(t, []string{"account-1"}, grpcRec.listMetadata.Get("x-voice-user-id"))
	require.Equal(t, []string{"profile-1"}, grpcRec.listMetadata.Get("x-voice-profile-id"))
	var listed map[string]any
	require.NoError(t, json.Unmarshal(list.Body.Bytes(), &listed))
	require.Contains(t, listed, "scheduled_messages")
	require.Equal(t, "cursor-next", listed["page"].(map[string]any)["next_cursor"])

	updated := performRequest(h, http.MethodPatch, "/api/v1/messages/scheduled/path-id", `{"scheduled_message_id":"forged-id","sender_profile_id":"forged-profile","payload":{"content":"updated"},"scheduled_at":"2026-10-10T12:00:00Z"}`, headers)
	require.Equal(t, http.StatusOK, updated.Code, updated.Body.String())
	require.Equal(t, "path-id", grpcRec.update.GetScheduledMessageId())
	require.Equal(t, "updated", grpcRec.update.GetPayload().GetContent())
	require.Equal(t, timestamppb.New(time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)), grpcRec.update.GetScheduledAt())
	require.Equal(t, []string{"profile-1"}, grpcRec.updateMetadata.Get("x-voice-profile-id"))
	var updateBody map[string]any
	require.NoError(t, json.Unmarshal(updated.Body.Bytes(), &updateBody))
	require.Contains(t, updateBody, "scheduled_message")

	cancelled := performRequest(h, http.MethodDelete, "/api/v1/messages/scheduled/path-id", "", headers)
	require.Equal(t, http.StatusNoContent, cancelled.Code, cancelled.Body.String())
	require.Equal(t, "path-id", grpcRec.cancel.GetScheduledMessageId())
	require.Equal(t, []string{"profile-1"}, grpcRec.cancelMetadata.Get("x-voice-profile-id"))

	sentNow := performRequest(h, http.MethodPost, "/api/v1/messages/scheduled/path-id/send-now", "", headers)
	require.Equal(t, http.StatusOK, sentNow.Code, sentNow.Body.String())
	require.Equal(t, "path-id", grpcRec.sendNow.GetScheduledMessageId())
	require.Equal(t, []string{"profile-1"}, grpcRec.sendNowMetadata.Get("x-voice-profile-id"))
	require.Contains(t, sentNow.Body.String(), `"message"`)
	require.False(t, proxyCalled, "scheduled requests must use Messaging gRPC")
}

func TestTranscodeScheduledMessageCreatePreservesUnion(t *testing.T) {
	t.Parallel()
	grpcRec := &recordingScheduledMessages{}
	conn, cleanup := startBufconnMessagingConn(t, grpcRec)
	t.Cleanup(cleanup)
	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"}},
		transcoder:  &transcoder{clients: grpcClients{messaging: messagingv1.NewMessagingServiceClient(conn)}},
	})
	resp := performRequest(h, http.MethodPost, "/api/v1/messages/send", `{"chat":{"id":"chat-2","type":"CHAT_TYPE_DM"},"sender_profile_id":"forged-profile","content":"later","client_message_id":"client-1","send_when_online":true}`, map[string]string{
		"Authorization":      "Bearer valid-user-token",
		"X-Voice-Profile-Id": "forged-profile",
	})
	require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
	require.Equal(t, "chat-2", grpcRec.send.GetChat().GetId())
	require.Equal(t, "client-1", grpcRec.send.GetClientMessageId())
	require.Equal(t, "later", grpcRec.send.GetContent())
	require.True(t, grpcRec.send.GetSendWhenOnline())
	require.Equal(t, []string{"profile-1"}, grpcRec.sendMetadata.Get("x-voice-profile-id"))
	var body map[string]any
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
	require.Contains(t, body, "scheduled_message")
	require.NotContains(t, body, "message")
}

func TestTranscodeScheduledCancelMapsMessagingErrors(t *testing.T) {
	t.Parallel()
	grpcRec := &recordingScheduledMessages{cancelErr: status.Error(codes.FailedPrecondition, "failed_precondition")}
	conn, cleanup := startBufconnMessagingConn(t, grpcRec)
	t.Cleanup(cleanup)
	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"}},
		transcoder:  &transcoder{clients: grpcClients{messaging: messagingv1.NewMessagingServiceClient(conn)}},
	})
	resp := performRequest(h, http.MethodDelete, "/api/v1/messages/scheduled/schedule-1", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Equal(t, http.StatusPreconditionFailed, resp.Code, resp.Body.String())
}

type recordingScheduledMessages struct {
	messagingv1.UnimplementedMessagingServiceServer
	list             *messagingv1.ListScheduledMessagesRequest
	update           *messagingv1.UpdateScheduledMessageRequest
	cancel           *messagingv1.CancelScheduledMessageRequest
	sendNow          *messagingv1.SendScheduledMessageNowRequest
	send             *messagingv1.SendMessageRequest
	listMetadata     metadata.MD
	updateMetadata   metadata.MD
	cancelMetadata   metadata.MD
	sendNowMetadata  metadata.MD
	sendMetadata     metadata.MD
	cancelErr        error
}

func incomingMetadata(ctx context.Context) metadata.MD {
	md, _ := metadata.FromIncomingContext(ctx)
	return md
}

func (s *recordingScheduledMessages) ListScheduledMessages(ctx context.Context, req *messagingv1.ListScheduledMessagesRequest) (*messagingv1.ListScheduledMessagesResponse, error) {
	s.list, s.listMetadata = req, incomingMetadata(ctx)
	return &messagingv1.ListScheduledMessagesResponse{
		ScheduledMessages: []*messagingv1.ScheduledMessage{{Id: "scheduled-1"}},
		Page:              &commonv1.CursorPageResponse{NextCursor: "cursor-next", HasMore: true},
	}, nil
}

func (s *recordingScheduledMessages) UpdateScheduledMessage(ctx context.Context, req *messagingv1.UpdateScheduledMessageRequest) (*messagingv1.UpdateScheduledMessageResponse, error) {
	s.update, s.updateMetadata = req, incomingMetadata(ctx)
	return &messagingv1.UpdateScheduledMessageResponse{ScheduledMessage: &messagingv1.ScheduledMessage{Id: req.GetScheduledMessageId()}}, nil
}

func (s *recordingScheduledMessages) CancelScheduledMessage(ctx context.Context, req *messagingv1.CancelScheduledMessageRequest) (*messagingv1.CancelScheduledMessageResponse, error) {
	s.cancel, s.cancelMetadata = req, incomingMetadata(ctx)
	return &messagingv1.CancelScheduledMessageResponse{}, s.cancelErr
}

func (s *recordingScheduledMessages) SendScheduledMessageNow(ctx context.Context, req *messagingv1.SendScheduledMessageNowRequest) (*messagingv1.SendScheduledMessageNowResponse, error) {
	s.sendNow, s.sendNowMetadata = req, incomingMetadata(ctx)
	return &messagingv1.SendScheduledMessageNowResponse{Message: &messagingv1.Message{Id: "sent-message-1"}}, nil
}

func (s *recordingScheduledMessages) SendMessage(ctx context.Context, req *messagingv1.SendMessageRequest) (*messagingv1.SendMessageResponse, error) {
	s.send, s.sendMetadata = req, incomingMetadata(ctx)
	return &messagingv1.SendMessageResponse{ScheduledMessage: &messagingv1.ScheduledMessage{Id: "scheduled-created-1"}}, nil
}
