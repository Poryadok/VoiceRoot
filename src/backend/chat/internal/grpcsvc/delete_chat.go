package grpcsvc

import (
	"context"
	"errors"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/chat/internal/authctx"
	"voice/backend/chat/internal/store"

	chatv1 "voice.app/voice/chat/v1"
)

// DeleteChat soft-deletes a DM for the caller only (screen-controls §1.4 #7 / navigation.md).
// Groups/channels are rejected — use LeaveChat / space tree flows instead.
func (s *ChatGRPC) DeleteChat(ctx context.Context, req *chatv1.DeleteChatRequest) (*chatv1.DeleteChatResponse, error) {
	if s == nil || s.DM == nil {
		return nil, status.Error(codes.FailedPrecondition, "chat persistence not configured")
	}
	profileID, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	chatID, err := parseUUIDField("chat_id", req.GetChatId())
	if err != nil {
		return nil, err
	}
	row, err := s.DM.FindChatByID(ctx, chatID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if row == nil {
		return nil, status.Error(codes.NotFound, "chat not found")
	}
	if row.Type != "dm" {
		return nil, status.Error(codes.InvalidArgument, "delete chat is only supported for DMs")
	}
	member, err := s.DM.IsChatMember(ctx, chatID, profileID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !member {
		return nil, status.Error(codes.PermissionDenied, "not a chat member")
	}
	if err := s.DM.MarkChatDeletedForSelf(ctx, chatID, profileID); err != nil {
		if errors.Is(err, store.ErrDeleteChatDMOnly) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "chat not found")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	if s.ChatEvents != nil {
		if err := s.ChatEvents.PublishChatMemberChanged(ctx, chatID.String(), profileID.String(), "left"); err != nil {
			s.logPublishError(ctx, "chat.member_changed", err,
				slog.String("chat_id", chatID.String()),
				slog.String("profile_id", profileID.String()))
		}
	}
	return &chatv1.DeleteChatResponse{}, nil
}
