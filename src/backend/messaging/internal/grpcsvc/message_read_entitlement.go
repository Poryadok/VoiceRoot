package grpcsvc

import (
	"context"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type messageReadEntitlement interface {
	MessageReadEntitled(ctx context.Context, chatID, profileID uuid.UUID, createdAt time.Time) (bool, error)
}

func (s *MessagingGRPC) requireMessageReadEntitlement(ctx context.Context, chatID, profileID uuid.UUID, createdAt time.Time) error {
	allowed, err := s.messageReadEntitled(ctx, chatID, profileID, createdAt)
	if err != nil {
		return err
	}
	if !allowed {
		return status.Error(codes.NotFound, "message not found")
	}
	return nil
}

func (s *MessagingGRPC) messageReadEntitled(ctx context.Context, chatID, profileID uuid.UUID, createdAt time.Time) (bool, error) {
	if s == nil || s.ChatGuard == nil || createdAt.IsZero() {
		return false, status.Error(codes.Unavailable, "message history entitlement unavailable")
	}
	checker, ok := s.ChatGuard.(messageReadEntitlement)
	if !ok {
		return false, status.Error(codes.Unavailable, "message history entitlement unavailable")
	}
	allowed, err := checker.MessageReadEntitled(ctx, chatID, profileID, createdAt)
	if err != nil {
		return false, status.Error(codes.Unavailable, "message history entitlement unavailable")
	}
	return allowed, nil
}
