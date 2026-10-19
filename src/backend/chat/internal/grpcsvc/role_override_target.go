package grpcsvc

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/pkg/roleoverrideprincipal"
)

type roleOverrideTargetStore interface {
	ValidateRoleOverrideTarget(context.Context, uuid.UUID, uuid.UUID) error
}

func (s *ChatGRPC) ValidateRoleOverrideTarget(ctx context.Context, req *chatv1.ValidateRoleOverrideTargetRequest) (*chatv1.ValidateRoleOverrideTargetResponse, error) {
	if err := roleoverrideprincipal.Require(ctx, "chat", req); err != nil {
		return nil, err
	}
	spaceID, err := roleoverrideprincipal.CanonicalID(req.GetSpaceId())
	if err != nil {
		return nil, err
	}
	chatID, err := roleoverrideprincipal.CanonicalID(req.GetChatId())
	if err != nil {
		return nil, err
	}
	if s == nil {
		return nil, status.Error(codes.Unavailable, "target validation unavailable")
	}
	targetStore, ok := s.DM.(roleOverrideTargetStore)
	if !ok {
		return nil, status.Error(codes.Unavailable, "target validation unavailable")
	}
	if err := targetStore.ValidateRoleOverrideTarget(ctx, spaceID, chatID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "target not found")
		}
		if ctx.Err() != nil {
			return nil, status.FromContextError(ctx.Err()).Err()
		}
		return nil, status.Error(codes.Unavailable, "target validation unavailable")
	}
	return &chatv1.ValidateRoleOverrideTargetResponse{}, nil
}
