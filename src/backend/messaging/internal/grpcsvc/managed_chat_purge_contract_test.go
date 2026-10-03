package grpcsvc

import (
	"context"
	"encoding/hex"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
	messagingv1 "voice.app/voice/messaging/v1"
	"voice/backend/pkg/principal"
)

func TestManagedChatPurgeIsExposedOnMessagingService(t *testing.T) {
	var found bool
	for _, method := range messagingv1.MessagingService_ServiceDesc.Methods {
		if method.MethodName == "PurgeManagedChatContent" {
			found = true
			break
		}
	}
	require.True(t, found, "GIS-owned retention must reach the Messaging data owner")
}

type managedChatPurgeSpy struct {
	request *messagingv1.PurgeManagedChatContentRequest
	err     error
}

func (s *managedChatPurgeSpy) PurgeManagedChatContent(_ context.Context, request *messagingv1.PurgeManagedChatContentRequest) (*messagingv1.PurgeManagedChatContentResponse, error) {
	s.request = request
	if s.err != nil {
		return nil, s.err
	}
	hash, _ := principal.RequestHash(request)
	hashBytes, _ := hex.DecodeString(strings.TrimPrefix(hash, "sha256:"))
	return &messagingv1.PurgeManagedChatContentResponse{
		ReceiptId: uuid.NewString(), OperationId: request.GetOperationId(), ChatId: request.GetChatId(),
		PurgeAfter: request.GetPurgeAfter(), FileReceiptSha256: make([]byte, 32), SearchReceiptSha256: make([]byte, 32),
		RequestSha256: hashBytes, CompletedAt: timestamppb.Now(),
	}, nil
}

func TestPurgeManagedChatContentRequiresRequestBoundGISPrincipal(t *testing.T) {
	request := &messagingv1.PurgeManagedChatContentRequest{
		OperationId: uuid.NewString(), ChatId: uuid.NewString(),
		PurgeAfter: timestamppb.New(time.Now().UTC().Add(-time.Hour)),
	}
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	valid := principal.Principal{Kind: "service", Issuer: "gameintegration", Subject: "service:gameintegration",
		Audience: "messaging", RPC: messagingv1.MessagingService_PurgeManagedChatContent_FullMethodName,
		RequestID: request.GetOperationId(), RequestHash: hash}
	processor := &managedChatPurgeSpy{}
	server := &MessagingGRPC{ManagedChatPurger: processor}

	wrong := principal.WithVerified(context.Background(), principal.Principal{Kind: "service", Issuer: "bot", Subject: "service:bot",
		Audience: "messaging", RPC: valid.RPC, RequestID: valid.RequestID, RequestHash: hash})
	_, err = server.PurgeManagedChatContent(wrong, request)
	require.Error(t, err)
	require.Nil(t, processor.request)

	validCtx := principal.WithVerified(context.Background(), valid)
	response, err := server.PurgeManagedChatContent(validCtx, request)
	require.NoError(t, err)
	require.Equal(t, request.GetOperationId(), response.GetOperationId())
	require.Equal(t, request, processor.request)
}
