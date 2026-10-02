package grpcsvc

import (
	"context"
	"encoding/hex"
	"errors"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	messagingv1 "voice.app/voice/messaging/v1"
	"voice/backend/pkg/principal"
)

type ManagedChatPurgeProcessor interface {
	PurgeManagedChatContent(context.Context, *messagingv1.PurgeManagedChatContentRequest) (*messagingv1.PurgeManagedChatContentResponse, error)
}

var ErrManagedChatPurgeConflict = errors.New("managed chat purge request conflicts with saved operation")

func (s *MessagingGRPC) PurgeManagedChatContent(ctx context.Context, request *messagingv1.PurgeManagedChatContentRequest) (*messagingv1.PurgeManagedChatContentResponse, error) {
	if request == nil || request.GetPurgeAfter() == nil || !request.GetPurgeAfter().IsValid() {
		return nil, status.Error(codes.InvalidArgument, "valid managed chat purge request required")
	}
	operationID, err := uuid.Parse(request.GetOperationId())
	if err != nil || operationID == uuid.Nil || operationID.String() != request.GetOperationId() {
		return nil, status.Error(codes.InvalidArgument, "invalid managed chat purge operation")
	}
	chatID, err := uuid.Parse(request.GetChatId())
	if err != nil || chatID == uuid.Nil || chatID.String() != request.GetChatId() {
		return nil, status.Error(codes.InvalidArgument, "invalid managed chat purge target")
	}
	verified, ok := principal.FromContext(ctx)
	requestHash, hashErr := principal.RequestHash(request)
	expectedRequestHash, decodeErr := hex.DecodeString(strings.TrimPrefix(requestHash, "sha256:"))
	if !ok || hashErr != nil || !strings.HasPrefix(requestHash, "sha256:") || decodeErr != nil || len(expectedRequestHash) != 32 || verified.Kind != "service" || verified.Issuer != "gameintegration" || verified.Subject != "service:gameintegration" ||
		verified.Audience != "messaging" || verified.RPC != messagingv1.MessagingService_PurgeManagedChatContent_FullMethodName ||
		verified.RequestID != request.GetOperationId() || verified.RequestHash != requestHash {
		return nil, status.Error(codes.Unauthenticated, "invalid GIS principal binding")
	}
	if s == nil || s.ManagedChatPurger == nil {
		return nil, status.Error(codes.Unavailable, "managed chat purge owner is unavailable")
	}
	receipt, err := s.ManagedChatPurger.PurgeManagedChatContent(ctx, proto.Clone(request).(*messagingv1.PurgeManagedChatContentRequest))
	if err != nil {
		if errors.Is(err, ErrManagedChatPurgeConflict) {
			return nil, status.Error(codes.FailedPrecondition, "managed chat purge operation conflicts")
		}
		if _, ok := status.FromError(err); ok {
			return nil, err
		}
		return nil, status.Error(codes.Unavailable, "managed chat purge could not be completed")
	}
	if receipt == nil || receipt.GetReceiptId() == "" || receipt.GetOperationId() != request.GetOperationId() ||
		receipt.GetChatId() != request.GetChatId() || receipt.GetPurgeAfter() == nil ||
		!receipt.GetPurgeAfter().IsValid() || !receipt.GetPurgeAfter().AsTime().Equal(request.GetPurgeAfter().AsTime()) ||
		len(receipt.GetFileReceiptSha256()) != 32 || len(receipt.GetSearchReceiptSha256()) != 32 ||
		len(receipt.GetRequestSha256()) != 32 || string(receipt.GetRequestSha256()) != string(expectedRequestHash) ||
		receipt.GetCompletedAt() == nil || !receipt.GetCompletedAt().IsValid() {
		return nil, status.Error(codes.Unavailable, "managed chat purge owner returned incomplete evidence")
	}
	return receipt, nil
}
