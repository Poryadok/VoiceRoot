package grpcsvc

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/chat/internal/authctx"
	"voice/backend/chat/internal/store"
)

type chatPermissionStore interface {
	GetChatPermissions(context.Context, uuid.UUID, uuid.UUID) (store.ChatPermissions, error)
	SetChatMemberPostAsChat(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, bool) error
	AddGroupMembersAs(context.Context, uuid.UUID, uuid.UUID, []uuid.UUID, []uuid.UUID) ([]uuid.UUID, error)
	UpdateGroupChatWithPermissions(context.Context, uuid.UUID, uuid.UUID, *string, *string, *string, *int32, *bool, *bool, *bool, *bool) (*store.ChatRow, error)
}

func (s *ChatGRPC) permissionStore() (chatPermissionStore, error) {
	if s == nil || s.DM == nil {
		return nil, status.Error(codes.FailedPrecondition, "chat permission persistence not configured")
	}
	p, ok := s.DM.(chatPermissionStore)
	if !ok {
		return nil, status.Error(codes.FailedPrecondition, "chat permission persistence not configured")
	}
	return p, nil
}

func chatPermissionError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, store.ErrChatPermissionScope), errors.Is(err, store.ErrManagedChatPrincipalRequired):
		return status.Error(codes.FailedPrecondition, "ordinary chat permissions unavailable for this scope")
	case errors.Is(err, store.ErrRoleChangeForbidden), errors.Is(err, store.ErrNotGroupMember):
		return status.Error(codes.PermissionDenied, "chat permission denied")
	case errors.Is(err, store.ErrRoleChangeInvalid):
		return status.Error(codes.InvalidArgument, "target must be an ordinary current member")
	case errors.Is(err, pgx.ErrNoRows):
		return status.Error(codes.NotFound, "chat or member not found")
	default:
		return status.Error(codes.Unavailable, "chat permission authority unavailable")
	}
}

func (s *ChatGRPC) GetChatPermissions(ctx context.Context, req *chatv1.GetChatPermissionsRequest) (*chatv1.GetChatPermissionsResponse, error) {
	actor, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	id, err := parseUUIDField("chat_id", req.GetChatId())
	if err != nil {
		return nil, err
	}
	if s == nil || s.DM == nil {
		return nil, status.Error(codes.FailedPrecondition, "chat persistence not configured")
	}
	st, err := s.permissionStore()
	if err != nil {
		return nil, err
	}
	p, err := st.GetChatPermissions(ctx, id, actor)
	if err != nil {
		return nil, chatPermissionError(err)
	}
	return &chatv1.GetChatPermissionsResponse{CanPostAsChat: p.CanPostAsChat, CanAddMembers: p.CanAddMembers, CanManagePermissions: p.CanManagePermissions, MemberAdditionsEnabled: p.MemberAdditionsEnabled, ExplicitPostAsChat: p.ExplicitPostAsChat}, nil
}

func (s *ChatGRPC) SetChatMemberPostAsChat(ctx context.Context, req *chatv1.SetChatMemberPostAsChatRequest) (*chatv1.SetChatMemberPostAsChatResponse, error) {
	actor, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	id, err := parseUUIDField("chat_id", req.GetChatId())
	if err != nil {
		return nil, err
	}
	target, err := parseUUIDField("target_profile_id", req.GetTargetProfileId())
	if err != nil {
		return nil, err
	}
	if s == nil || s.DM == nil {
		return nil, status.Error(codes.FailedPrecondition, "chat persistence not configured")
	}
	if err := authctx.RequireRegular(ctx); err != nil {
		return nil, err
	}
	st, err := s.permissionStore()
	if err != nil {
		return nil, err
	}
	if err = st.SetChatMemberPostAsChat(ctx, id, actor, target, req.GetAllowed()); err != nil {
		return nil, chatPermissionError(err)
	}
	return &chatv1.SetChatMemberPostAsChatResponse{}, nil
}
