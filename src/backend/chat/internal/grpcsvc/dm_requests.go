package grpcsvc

import (
	"context"
	"errors"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/chat/internal/authctx"

	chatv1 "voice.app/voice/chat/v1"
)

func (s *ChatGRPC) AcceptDMRequest(ctx context.Context, req *chatv1.AcceptDMRequestRequest) (*chatv1.AcceptDMRequestResponse, error) {
	if err := s.setRequestInbox(ctx, req.GetChatId(), "main"); err != nil {
		return nil, err
	}
	return &chatv1.AcceptDMRequestResponse{}, nil
}

func (s *ChatGRPC) DeclineDMRequest(ctx context.Context, req *chatv1.DeclineDMRequestRequest) (*chatv1.DeclineDMRequestResponse, error) {
	if err := s.setRequestInbox(ctx, req.GetChatId(), "declined"); err != nil {
		return nil, err
	}
	return &chatv1.DeclineDMRequestResponse{}, nil
}

func (s *ChatGRPC) setRequestInbox(ctx context.Context, rawChatID, bucket string) error {
	if s == nil || s.DM == nil {
		return status.Error(codes.FailedPrecondition, "chat persistence not configured")
	}
	profileID, ok := authctx.ProfileID(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "missing profile")
	}
	chatID, err := parseUUIDField("chat_id", rawChatID)
	if err != nil {
		return err
	}
	members, err := s.DM.ListChatMembers(ctx, chatID)
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}
	if err := s.DM.SetInboxBucket(ctx, chatID, profileID, bucket); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return status.Error(codes.NotFound, "chat not found")
		}
		return status.Error(codes.Internal, err.Error())
	}
	if s.ChatEvents != nil {
		for _, member := range members {
			if err := s.ChatEvents.PublishChatMemberChanged(ctx, chatID.String(), member.ProfileID.String(), "inbox_bucket_changed"); err != nil {
				s.logPublishError(ctx, "chat.member_changed", err, slog.String("chat_id", chatID.String()), slog.String("profile_id", member.ProfileID.String()))
			}
		}
	}
	return nil
}
