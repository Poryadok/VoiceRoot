package grpcsvc

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/messaging/internal/store"
	"voice/backend/role/permissions"
)

func (s *MessagingGRPC) checkReactionPermission(ctx context.Context, policy *store.ChatThreadPolicy, chatID, profileID uuid.UUID) error {
	if policy.SpaceID == nil {
		return nil
	}
	if isNilDependency(s.RolePermissions) {
		return status.Error(codes.Unavailable, "role service unavailable")
	}
	allowed, err := s.RolePermissions.HasChatPermission(ctx, *policy.SpaceID, profileID, chatID, permissions.TextChatAddReactions)
	if err != nil {
		return status.Error(codes.Unavailable, "role service unavailable")
	}
	if !allowed {
		return status.Error(codes.PermissionDenied, "missing TEXT_CHAT_ADD_REACTIONS")
	}
	return nil
}
