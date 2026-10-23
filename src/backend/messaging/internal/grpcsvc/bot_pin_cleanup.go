package grpcsvc

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	messagingv1 "voice.app/voice/messaging/v1"
	"voice/backend/messaging/internal/botpinprincipal"
	"voice/backend/messaging/internal/store"
)

type BotPinCleanupStore interface {
	ApplyVerifiedBotChatPinRemoval(context.Context, botpinprincipal.VerifiedCleanup) (*messagingv1.ApplyBotChatPinRemovalResponse, error)
}

// This handler is registered only on the dedicated signed Bot/mTLS listener.
// Ordinary MessagingGRPC intentionally retains no alternate cleanup method.
type BotPinCleanupGRPC struct {
	messagingv1.UnimplementedMessagingServiceServer
	Pins BotPinCleanupStore
}

func (s *BotPinCleanupGRPC) ApplyBotChatPinRemoval(ctx context.Context, request *messagingv1.ApplyBotChatPinRemovalRequest) (*messagingv1.ApplyBotChatPinRemovalResponse, error) {
	capability, err := botpinprincipal.Require(ctx, request)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "verified Bot cleanup capability required")
	}
	if s == nil || s.Pins == nil {
		return nil, status.Error(codes.Unavailable, "Bot cleanup store unavailable")
	}
	response, err := s.Pins.ApplyVerifiedBotChatPinRemoval(ctx, capability)
	switch {
	case err == nil:
		return response, nil
	case errors.Is(err, botpinprincipal.ErrInvalidProof):
		return nil, status.Error(codes.Unauthenticated, "Bot cleanup proof expired or changed")
	case errors.Is(err, store.ErrBotPinRemovalBinding):
		return nil, status.Error(codes.InvalidArgument, "invalid Bot cleanup source binding")
	case errors.Is(err, store.ErrBotPinRemovalConflict):
		return nil, status.Error(codes.FailedPrecondition, "Bot cleanup source conflicts with retained authority")
	default:
		return nil, status.Error(codes.Unavailable, "Bot cleanup participant unavailable")
	}
}
