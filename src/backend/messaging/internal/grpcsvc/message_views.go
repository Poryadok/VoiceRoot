package grpcsvc

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	chatv1 "voice.app/voice/chat/v1"
	messagingv1 "voice.app/voice/messaging/v1"
	"voice/backend/messaging/internal/authctx"
	"voice/backend/messaging/internal/store"
)

func (s *MessagingGRPC) RecordMessageView(ctx context.Context, req *messagingv1.RecordMessageViewRequest) (*messagingv1.RecordMessageViewResponse, error) {
	profile, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	if s == nil || s.Messages == nil {
		return nil, status.Error(codes.FailedPrecondition, "messaging persistence not configured")
	}
	chat, err := parseUUIDField("chat_id", req.GetChatId())
	if err != nil {
		return nil, err
	}
	id, err := parseUUIDField("message_id", req.GetMessageId())
	if err != nil {
		return nil, err
	}
	if isNilDependency(s.ChatGuard) {
		return nil, status.Error(codes.Unavailable, "chat membership unavailable")
	}
	if err = s.ChatGuard.EnsureMember(ctx, chat, profile); err != nil {
		if errors.Is(err, store.ErrNotChatMember) {
			return nil, status.Error(codes.PermissionDenied, "not a chat member")
		}
		return nil, status.Error(codes.Unavailable, "chat membership unavailable")
	}
	kind, err := s.resolveAuthoritativeChatType(ctx, chat, profile)
	if err != nil {
		return nil, err
	}
	if kind == chatv1.ChatType_CHAT_TYPE_DM {
		return nil, status.Error(codes.InvalidArgument, "message views require group or channel")
	}
	message, err := s.GetMessage(ctx, &messagingv1.GetMessageRequest{MessageId: id.String()})
	if err != nil {
		return nil, err
	}
	if message.GetMessage().GetChat().GetId() != chat.String() {
		return nil, status.Error(codes.NotFound, "message not found")
	}
	policy, err := s.loadMutationChatPolicy(ctx, chat)
	if err != nil {
		return nil, err
	}
	count, err := s.Messages.RecordMessageView(ctx, chat, policy.SpaceID, id, profile)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, status.Error(codes.NotFound, "message not found")
	}
	if err != nil {
		return nil, status.Error(codes.Unavailable, "message views unavailable")
	}
	return &messagingv1.RecordMessageViewResponse{ViewCount: count}, nil
}

func (s *MessagingGRPC) qualifyingReadViews(ctx context.Context, chat, profile, through uuid.UUID) ([]uuid.UUID, error) {
	kind, err := s.resolveAuthoritativeChatType(ctx, chat, profile)
	if err != nil {
		return nil, err
	}
	if kind == chatv1.ChatType_CHAT_TYPE_DM {
		return nil, nil
	}
	// The cursor target itself must be visible, just like an individual viewport view.
	target, err := s.GetMessage(ctx, &messagingv1.GetMessageRequest{MessageId: through.String()})
	if err != nil {
		return nil, err
	}
	if target.GetMessage().GetChat().GetId() != chat.String() {
		return nil, status.Error(codes.NotFound, "message not found")
	}
	after := uuid.Nil
	ids := []uuid.UUID{}
	for {
		candidates, err := s.Messages.MessageViewCandidates(ctx, chat, profile, through, after)
		if err != nil {
			return nil, status.Error(codes.Unavailable, "message views unavailable")
		}
		if len(candidates) == 0 {
			break
		}
		after = candidates[len(candidates)-1].ID
		done := len(candidates) < 200
		candidates, err = s.filterBlockedHistoryRows(ctx, profile, candidates)
		if err != nil {
			return nil, err
		}
		for _, m := range candidates {
			allowed, err := s.messageReadEntitled(ctx, chat, profile, m.CreatedAt)
			if err != nil {
				return nil, err
			}
			if allowed {
				ids = append(ids, m.ID)
			}
		}
		if done {
			break
		}
	}
	return ids, nil
}
