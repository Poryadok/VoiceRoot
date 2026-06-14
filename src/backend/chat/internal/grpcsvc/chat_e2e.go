package grpcsvc

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/chat/internal/authctx"

	chatv1 "voice.app/voice/chat/v1"
)

func (s *ChatGRPC) EnableChatE2E(ctx context.Context, req *chatv1.EnableChatE2ERequest) (*chatv1.EnableChatE2EResponse, error) {
	if err := s.setChatE2E(ctx, req.GetChatId(), true); err != nil {
		return nil, err
	}
	return &chatv1.EnableChatE2EResponse{}, nil
}

func (s *ChatGRPC) DisableChatE2E(ctx context.Context, req *chatv1.DisableChatE2ERequest) (*chatv1.DisableChatE2EResponse, error) {
	if err := s.setChatE2E(ctx, req.GetChatId(), false); err != nil {
		return nil, err
	}
	return &chatv1.DisableChatE2EResponse{}, nil
}

func (s *ChatGRPC) setChatE2E(ctx context.Context, chatRaw string, enabled bool) error {
	if s == nil || s.DM == nil {
		return status.Error(codes.FailedPrecondition, "chat persistence not configured")
	}
	caller, ok := authctx.ProfileID(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "missing profile")
	}
	chatID, err := parseUUIDField("chat_id", chatRaw)
	if err != nil {
		return err
	}
	row, err := s.DM.FindChatByID(ctx, chatID)
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}
	if row == nil {
		return status.Error(codes.NotFound, "chat not found")
	}
	if row.Type != "dm" {
		return status.Error(codes.FailedPrecondition, "e2e is dm-only")
	}
	member, err := s.DM.IsChatMember(ctx, chatID, caller)
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}
	if !member {
		return status.Error(codes.PermissionDenied, "not a chat member")
	}
	if err := s.DM.SetChatE2EEnabled(ctx, chatID, enabled); err != nil {
		return status.Error(codes.Internal, err.Error())
	}
	return nil
}
