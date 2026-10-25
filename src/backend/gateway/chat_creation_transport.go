package main

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	chatv1 "voice.app/voice/chat/v1"
)

func isChatCreationRequest(r *http.Request) bool {
	return r != nil && r.Method == http.MethodPost && strings.TrimSuffix(r.URL.Path, "/") == "/api/v1/chats"
}

func (t *transcoder) chatCreationContext(r *http.Request, request *chatv1.CreateChatRequest) (context.Context, error) {
	requestID := request.GetRequestId()
	if requestID != "" && !canonicalLifecycleUUID(requestID) {
		return nil, status.Error(codes.InvalidArgument, "request_id must be a canonical UUID")
	}
	if requestID == "" {
		// A transport binding is required even for an unkeyed business request.
		// Keep the protobuf unchanged: unkeyed retries must not gain idempotency.
		requestID = uuid.NewString()
	}
	return t.auditInitiatingContext(r, request, chatv1.ChatService_CreateChat_FullMethodName, requestID)
}
