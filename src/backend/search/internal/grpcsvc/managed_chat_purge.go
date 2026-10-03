package grpcsvc

import (
	"context"
	"encoding/hex"
	"errors"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	searchv1 "voice.app/voice/search/v1"
	"voice/backend/pkg/principal"
	"voice/backend/search/internal/store"
)

func (s *SearchGRPC) PurgeManagedChatMessages(ctx context.Context, request *searchv1.PurgeManagedChatMessagesRequest) (*searchv1.PurgeManagedChatMessagesResponse, error) {
	if request == nil {
		return nil, status.Error(codes.InvalidArgument, "managed chat purge request is required")
	}
	operationID, err := uuid.Parse(request.GetOperationId())
	if err != nil || operationID == uuid.Nil || operationID.String() != request.GetOperationId() {
		return nil, status.Error(codes.InvalidArgument, "invalid managed chat purge operation")
	}
	chatID, err := uuid.Parse(request.GetChatId())
	if err != nil || chatID == uuid.Nil || chatID.String() != request.GetChatId() {
		return nil, status.Error(codes.InvalidArgument, "invalid managed chat purge target")
	}
	ids := make([]uuid.UUID, len(request.GetMessageIds()))
	for index, raw := range request.GetMessageIds() {
		id, parseErr := uuid.Parse(raw)
		if parseErr != nil || id == uuid.Nil || id.String() != raw || (index > 0 && request.GetMessageIds()[index-1] >= raw) {
			return nil, status.Error(codes.InvalidArgument, "managed chat purge message IDs must be canonical, sorted, and unique")
		}
		ids[index] = id
	}
	requestHash, err := principal.RequestHash(request)
	if err != nil || !strings.HasPrefix(requestHash, "sha256:") {
		return nil, status.Error(codes.InvalidArgument, "invalid managed chat purge request hash")
	}
	requestHashBytes, err := hex.DecodeString(strings.TrimPrefix(requestHash, "sha256:"))
	if err != nil || len(requestHashBytes) != 32 {
		return nil, status.Error(codes.InvalidArgument, "invalid managed chat purge request hash")
	}
	verified, ok := principal.FromContext(ctx)
	if !ok || verified.Kind != "service" || verified.Issuer != "messaging" || verified.Subject != "service:messaging" || verified.Audience != "search" ||
		verified.RPC != searchv1.SearchService_PurgeManagedChatMessages_FullMethodName || verified.RequestID != request.GetOperationId() || verified.RequestHash != requestHash ||
		verified.AccountID != "" || verified.ProfileID != "" || verified.SessionEpoch != 0 {
		return nil, status.Error(codes.Unauthenticated, "invalid Messaging principal binding")
	}
	if s == nil || s.ManagedChatPurger == nil {
		return nil, status.Error(codes.Unavailable, "managed chat Search purge owner is unavailable")
	}
	receipt, err := s.ManagedChatPurger.PurgeManagedChatMessages(ctx, operationID, chatID, ids, requestHashBytes)
	if err != nil {
		if errors.Is(err, store.ErrManagedChatSearchPurgeConflict) {
			return nil, status.Error(codes.FailedPrecondition, "managed chat Search purge operation conflicts")
		}
		return nil, status.Error(codes.Unavailable, "managed chat Search purge could not be completed")
	}
	if receipt == nil || receipt.OperationID != operationID || receipt.ChatID != chatID || receipt.ReceiptID == uuid.Nil || receipt.DeletedCount != uint64(len(ids)) ||
		len(receipt.MessageIDsHash) != 32 || string(receipt.MessageIDsHash) != string(store.ManagedChatMessageIDsHash(ids)) ||
		len(receipt.RequestHash) != 32 || string(receipt.RequestHash) != string(requestHashBytes) || receipt.CompletedAt.IsZero() {
		return nil, status.Error(codes.Unavailable, "managed chat Search purge owner returned incomplete evidence")
	}
	return &searchv1.PurgeManagedChatMessagesResponse{ReceiptId: receipt.ReceiptID.String(), OperationId: operationID.String(), ChatId: chatID.String(),
		DeletedCount: receipt.DeletedCount, MessageIdsSha256: receipt.MessageIDsHash, RequestSha256: receipt.RequestHash, CompletedAt: timestamppb.New(receipt.CompletedAt)}, nil
}
