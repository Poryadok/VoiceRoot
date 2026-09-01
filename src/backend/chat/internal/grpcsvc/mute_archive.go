package grpcsvc

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/chat/internal/authctx"

	chatv1 "voice.app/voice/chat/v1"
)

// ArchiveChat sets per-member is_archived (text-chat.md §Архивирование / скрытие).
func (s *ChatGRPC) ArchiveChat(ctx context.Context, req *chatv1.ArchiveChatRequest) (*chatv1.ArchiveChatResponse, error) {
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
	if err := s.DM.SetMemberArchived(ctx, chatID, profileID, req.GetArchived()); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "chat not found")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	if req.GetArchived() {
		if err := s.DM.RemoveQuickAccess(ctx, profileID, chatID); err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	return &chatv1.ArchiveChatResponse{}, nil
}

// MuteChat sets or clears per-member muted_until.
// When muted_until is omitted, the chat is unmuted (column cleared).
func (s *ChatGRPC) MuteChat(ctx context.Context, req *chatv1.MuteChatRequest) (*chatv1.MuteChatResponse, error) {
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
	var until *time.Time
	if ts := req.GetMutedUntil(); ts != nil {
		t := ts.AsTime().UTC()
		until = &t
	}
	if err := s.DM.SetMemberMutedUntil(ctx, chatID, profileID, until); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "chat not found")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &chatv1.MuteChatResponse{}, nil
}
