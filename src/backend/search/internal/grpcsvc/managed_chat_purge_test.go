package grpcsvc

import (
	"context"
	"encoding/hex"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	searchv1 "voice.app/voice/search/v1"
	"voice/backend/pkg/principal"
	"voice/backend/search/internal/store"
)

type managedChatSearchPurgeSpy struct {
	called bool
	err    error
}

func (s *managedChatSearchPurgeSpy) PurgeManagedChatMessages(_ context.Context, operationID, chatID uuid.UUID, ids []uuid.UUID, requestHash []byte) (*store.ManagedChatSearchPurgeReceipt, error) {
	s.called = true
	if s.err != nil {
		return nil, s.err
	}
	return &store.ManagedChatSearchPurgeReceipt{OperationID: operationID, ChatID: chatID, ReceiptID: uuid.New(), DeletedCount: uint64(len(ids)), MessageIDsHash: store.ManagedChatMessageIDsHash(ids), RequestHash: requestHash, CompletedAt: time.Now().UTC()}, nil
}

func TestPurgeManagedChatMessagesRequiresMessagingPrincipalBoundToExactRequest(t *testing.T) {
	request := &searchv1.PurgeManagedChatMessagesRequest{OperationId: uuid.NewString(), ChatId: uuid.NewString()}
	messageID := uuid.New()
	request.MessageIds = []string{messageID.String()}
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	method := searchv1.SearchService_PurgeManagedChatMessages_FullMethodName
	spy := &managedChatSearchPurgeSpy{}
	service := &SearchGRPC{ManagedChatPurger: spy}
	wrong := principal.WithVerified(context.Background(), principal.Principal{Kind: "service", Issuer: "space", Subject: "service:space", Audience: "search", RPC: method, RequestID: request.GetOperationId(), RequestHash: hash})
	_, err = service.PurgeManagedChatMessages(wrong, request)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	require.False(t, spy.called)

	valid := principal.WithVerified(context.Background(), principal.Principal{Kind: "service", Issuer: "messaging", Subject: "service:messaging", Audience: "search", RPC: method, RequestID: request.GetOperationId(), RequestHash: hash})
	response, err := service.PurgeManagedChatMessages(valid, request)
	require.NoError(t, err)
	require.Equal(t, request.GetOperationId(), response.GetOperationId())
	require.Equal(t, request.GetChatId(), response.GetChatId())
	require.Equal(t, uint64(1), response.GetDeletedCount())
	require.Equal(t, 32, len(response.GetMessageIdsSha256()))
	expectedHash, err := hex.DecodeString(hash[len("sha256:"):])
	require.NoError(t, err)
	require.Equal(t, expectedHash, response.GetRequestSha256())
}

func TestPurgeManagedChatMessagesFailsClosedWhenStoreUnavailable(t *testing.T) {
	request := &searchv1.PurgeManagedChatMessagesRequest{OperationId: uuid.NewString(), ChatId: uuid.NewString()}
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	service := &SearchGRPC{ManagedChatPurger: &managedChatSearchPurgeSpy{err: context.DeadlineExceeded}}
	ctx := principal.WithVerified(context.Background(), principal.Principal{Kind: "service", Issuer: "messaging", Subject: "service:messaging", Audience: "search", RPC: searchv1.SearchService_PurgeManagedChatMessages_FullMethodName, RequestID: request.GetOperationId(), RequestHash: hash})
	_, err = service.PurgeManagedChatMessages(ctx, request)
	require.Equal(t, codes.Unavailable, status.Code(err))
}
