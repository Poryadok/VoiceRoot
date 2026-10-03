package grpcsvc

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	messagingv1 "voice.app/voice/messaging/v1"
	"voice/backend/messaging/internal/store"
	"voice/backend/pkg/principal"
)

// GameMessageProcessor is the authority boundary behind the internal T15 RPC.
// Implementations must return a Messaging-owned sender profile only after
// current Auth binding, key status, chat membership, and File checks succeed.
type GameMessageProcessor interface {
	ProcessGameMessage(ctx context.Context, compactJWS, deviceAuthorityAssertion string) (*store.MessageRow, error)
}

// GameTombstoneProcessor appends a Messaging-owned signed terminal revision
// after the service principal and its exact protobuf request binding are checked.
type GameTombstoneProcessor interface {
	ProcessTombstoneGameMessage(ctx context.Context, request *messagingv1.TombstoneGameMessageRequest) error
}

func (s *MessagingGRPC) TombstoneGameMessage(ctx context.Context, req *messagingv1.TombstoneGameMessageRequest) (*messagingv1.TombstoneGameMessageResponse, error) {
	if req == nil || req.GetActionId() == "" || req.GetApplicationId() == "" || req.GetEnvironmentId() == "" || req.GetChatId() == "" || req.GetMessageId() == "" || req.GetReasonClass() == "" {
		return nil, status.Error(codes.InvalidArgument, "complete tombstone target and action are required")
	}
	if s == nil || s.GameTombstones == nil {
		return nil, status.Error(codes.FailedPrecondition, "game tombstone authority is unavailable")
	}
	verified, ok := principal.FromContext(ctx)
	requestHash, hashErr := principal.RequestHash(req)
	if !ok || hashErr != nil || verified.Kind != "service" || verified.Issuer != "moderation" || verified.Subject != "service:moderation" || verified.Audience != "messaging" ||
		verified.RPC != "/voice.messaging.v1.MessagingService/TombstoneGameMessage" || verified.RequestID == "" || verified.RequestHash != requestHash {
		return nil, status.Error(codes.PermissionDenied, "authenticated moderation service principal required")
	}
	if err := s.GameTombstones.ProcessTombstoneGameMessage(ctx, req); err != nil {
		if _, ok := status.FromError(err); ok {
			return nil, err
		}
		return nil, status.Error(codes.InvalidArgument, "game message tombstone rejected")
	}
	return &messagingv1.TombstoneGameMessageResponse{}, nil
}

func (s *MessagingGRPC) ApplyGameMessage(ctx context.Context, req *messagingv1.ApplyGameMessageRequest) (*messagingv1.ApplyGameMessageResponse, error) {
	if req == nil || req.GetCompactJws() == "" {
		return nil, status.Error(codes.InvalidArgument, "signed game message is required")
	}
	verified, ok := principal.FromContext(ctx)
	requestHash, hashErr := principal.RequestHash(req)
	if !ok || hashErr != nil || verified.Kind != "service" || verified.Issuer != "gateway" || verified.Subject != "service:gateway" || verified.Audience != "messaging" ||
		verified.RPC != messagingv1.MessagingService_ApplyGameMessage_FullMethodName || verified.RequestID == "" || verified.RequestHash != requestHash ||
		verified.AccountID != "" || verified.ProfileID != "" || verified.SessionEpoch != 0 {
		return nil, status.Error(codes.PermissionDenied, "authenticated Gateway service principal required")
	}
	if s == nil || s.GameMessages == nil {
		return nil, status.Error(codes.FailedPrecondition, "game message authority is unavailable")
	}
	row, err := s.GameMessages.ProcessGameMessage(ctx, req.GetCompactJws(), req.GetDeviceAuthorityAssertion())
	if err != nil {
		if _, ok := status.FromError(err); ok {
			return nil, err
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "game message not found")
		}
		return nil, status.Error(codes.InvalidArgument, "game message rejected")
	}
	if row == nil {
		return nil, status.Error(codes.Internal, "game message processor returned no result")
	}
	return &messagingv1.ApplyGameMessageResponse{Message: messageRowToProto(row, messagingv1.MessageKind_MESSAGE_KIND_UNSPECIFIED, "[]", false)}, nil
}
