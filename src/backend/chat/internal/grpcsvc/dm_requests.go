package grpcsvc

import (
	"context"
	"errors"

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
	if err := s.DM.SetInboxBucket(ctx, chatID, profileID, bucket); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return status.Error(codes.NotFound, "chat not found")
		}
		return status.Error(codes.Internal, err.Error())
	}
	return nil
}
