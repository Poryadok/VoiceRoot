package grpcsvc

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	chatv1 "voice.app/voice/chat/v1"
	rolev1 "voice.app/voice/role/v1"

	"voice/backend/role/internal/authctx"
	"voice/backend/role/permissions"
)

func (s *RoleGRPC) requireManageRoles(ctx context.Context, spaceID, actor uuid.UUID) error {
	mask, err := permissions.MaskFor(permissions.SpaceManageRoles)
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}
	eff, err := s.Store.GetEffectiveMask(ctx, spaceID, actor, nil, nil)
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}
	if eff&mask == 0 {
		return status.Error(codes.PermissionDenied, "SPACE_MANAGE_ROLES required")
	}
	return nil
}

func (s *RoleGRPC) UpdateRole(ctx context.Context, req *rolev1.UpdateRoleRequest) (*rolev1.UpdateRoleResponse, error) {
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.FailedPrecondition, "role persistence not configured")
	}
	actor, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	roleID, err := parseUUIDField("role_id", req.GetRoleId())
	if err != nil {
		return nil, err
	}
	row, err := s.Store.GetRoleByID(ctx, roleID)
	if err != nil || row == nil {
		return nil, status.Error(codes.NotFound, "role not found")
	}
	if err := s.requireManageRoles(ctx, row.SpaceID, actor); err != nil {
		return nil, err
	}
	can, err := s.Store.CanEditRole(ctx, row.SpaceID, actor, roleID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !can {
		return nil, status.Error(codes.PermissionDenied, "cannot edit this role")
	}
	var name *string
	if req.Name != nil {
		n := strings.TrimSpace(req.GetName())
		if n == "" {
			return nil, status.Error(codes.InvalidArgument, "name cannot be empty")
		}
		name = &n
	}
	updated, err := s.Store.UpdateRole(ctx, roleID, name, req.PermissionsMask, req.Position)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if s.Events != nil {
		_ = s.Events.PublishRoleUpdated(ctx, row.SpaceID.String(), roleID.String(), []string{"name", "permissions", "position"})
	}
	return &rolev1.UpdateRoleResponse{Role: roleRowToProto(updated)}, nil
}

func (s *RoleGRPC) DeleteRole(ctx context.Context, req *rolev1.DeleteRoleRequest) (*rolev1.DeleteRoleResponse, error) {
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.FailedPrecondition, "role persistence not configured")
	}
	actor, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	roleID, err := parseUUIDField("role_id", req.GetRoleId())
	if err != nil {
		return nil, err
	}
	row, err := s.Store.GetRoleByID(ctx, roleID)
	if err != nil || row == nil {
		return nil, status.Error(codes.NotFound, "role not found")
	}
	if err := s.requireManageRoles(ctx, row.SpaceID, actor); err != nil {
		return nil, err
	}
	can, err := s.Store.CanEditRole(ctx, row.SpaceID, actor, roleID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !can {
		return nil, status.Error(codes.PermissionDenied, "cannot delete this role")
	}
	if err := s.Store.DeleteRole(ctx, roleID); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if s.Events != nil {
		_ = s.Events.PublishRoleDeleted(ctx, row.SpaceID.String(), roleID.String())
	}
	return &rolev1.DeleteRoleResponse{}, nil
}

func (s *RoleGRPC) ReorderRoles(ctx context.Context, req *rolev1.ReorderRolesRequest) (*rolev1.ReorderRolesResponse, error) {
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.FailedPrecondition, "role persistence not configured")
	}
	actor, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	spaceID, err := parseUUIDField("space_id", req.GetSpaceId())
	if err != nil {
		return nil, err
	}
	if err := s.requireManageRoles(ctx, spaceID, actor); err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, 0, len(req.GetOrderedRoleIds()))
	for _, raw := range req.GetOrderedRoleIds() {
		id, err := parseUUIDField("role_id", raw)
		if err != nil {
			return nil, err
		}
		can, err := s.Store.CanEditRole(ctx, spaceID, actor, id)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		if !can {
			return nil, status.Error(codes.PermissionDenied, "cannot reorder roles above your hierarchy")
		}
		ids = append(ids, id)
	}
	if err := s.Store.ReorderRoles(ctx, spaceID, ids); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &rolev1.ReorderRolesResponse{}, nil
}

func (s *RoleGRPC) RemoveChatOverride(ctx context.Context, req *rolev1.RemoveChatOverrideRequest) (*rolev1.RemoveChatOverrideResponse, error) {
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.FailedPrecondition, "role persistence not configured")
	}
	actor, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	spaceID, err := parseUUIDField("space_id", req.GetSpaceId())
	if err != nil {
		return nil, err
	}
	if err := s.requireManageRoles(ctx, spaceID, actor); err != nil {
		return nil, err
	}
	if req.GetChat() == nil || strings.TrimSpace(req.GetChat().GetId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "chat is required")
	}
	chatID, err := parseUUIDField("chat_id", req.GetChat().GetId())
	if err != nil {
		return nil, err
	}
	roleID, err := parseUUIDField("role_id", req.GetRoleId())
	if err != nil {
		return nil, err
	}
	if err := s.Store.RemoveChatOverride(ctx, chatID, roleID); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &rolev1.RemoveChatOverrideResponse{}, nil
}

func (s *RoleGRPC) GetChatOverrides(ctx context.Context, req *rolev1.GetChatOverridesRequest) (*rolev1.GetChatOverridesResponse, error) {
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.FailedPrecondition, "role persistence not configured")
	}
	spaceID, err := parseUUIDField("space_id", req.GetSpaceId())
	if err != nil {
		return nil, err
	}
	var chatID *uuid.UUID
	if req.GetFilterChat() != nil && strings.TrimSpace(req.GetFilterChat().GetId()) != "" {
		id, err := parseUUIDField("chat_id", req.GetFilterChat().GetId())
		if err != nil {
			return nil, err
		}
		chatID = &id
	}
	rows, err := s.Store.ListChatOverrides(ctx, spaceID, chatID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	out := make([]*rolev1.PermissionOverride, 0, len(rows))
	for _, row := range rows {
		out = append(out, &rolev1.PermissionOverride{
			Chat:      &chatv1.ChatRef{Id: row.ChatID.String()},
			RoleId:    row.RoleID.String(),
			RoleName:  row.RoleName,
			AllowMask: row.Allow,
			DenyMask:  row.Deny,
		})
	}
	return &rolev1.GetChatOverridesResponse{OverrideList: &rolev1.OverrideList{Overrides: out}}, nil
}

func (s *RoleGRPC) SetVoiceRoomOverride(ctx context.Context, req *rolev1.SetVoiceRoomOverrideRequest) (*rolev1.SetVoiceRoomOverrideResponse, error) {
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.FailedPrecondition, "role persistence not configured")
	}
	actor, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	spaceID, err := parseUUIDField("space_id", req.GetSpaceId())
	if err != nil {
		return nil, err
	}
	if err := s.requireManageRoles(ctx, spaceID, actor); err != nil {
		return nil, err
	}
	roleID, err := parseUUIDField("role_id", req.GetRoleId())
	if err != nil {
		return nil, err
	}
	row, err := s.Store.GetRoleByID(ctx, roleID)
	if err != nil || row == nil || row.SpaceID != spaceID {
		return nil, status.Error(codes.NotFound, "role not found")
	}
	voiceRoomID, err := parseUUIDField("voice_room_id", req.GetVoiceRoomId())
	if err != nil {
		return nil, err
	}
	if err := s.Store.SetVoiceRoomOverride(ctx, voiceRoomID, roleID, req.GetAllowMask(), req.GetDenyMask()); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if s.Events != nil {
		_ = s.Events.PublishVoiceOverrideSet(ctx, voiceRoomID.String(), roleID.String())
	}
	return &rolev1.SetVoiceRoomOverrideResponse{}, nil
}

func (s *RoleGRPC) RemoveVoiceRoomOverride(ctx context.Context, req *rolev1.RemoveVoiceRoomOverrideRequest) (*rolev1.RemoveVoiceRoomOverrideResponse, error) {
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.FailedPrecondition, "role persistence not configured")
	}
	actor, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	spaceID, err := parseUUIDField("space_id", req.GetSpaceId())
	if err != nil {
		return nil, err
	}
	if err := s.requireManageRoles(ctx, spaceID, actor); err != nil {
		return nil, err
	}
	roleID, err := parseUUIDField("role_id", req.GetRoleId())
	if err != nil {
		return nil, err
	}
	voiceRoomID, err := parseUUIDField("voice_room_id", req.GetVoiceRoomId())
	if err != nil {
		return nil, err
	}
	if err := s.Store.RemoveVoiceRoomOverride(ctx, voiceRoomID, roleID); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &rolev1.RemoveVoiceRoomOverrideResponse{}, nil
}

func (s *RoleGRPC) GetVoiceRoomOverrides(ctx context.Context, req *rolev1.GetVoiceRoomOverridesRequest) (*rolev1.GetVoiceRoomOverridesResponse, error) {
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.FailedPrecondition, "role persistence not configured")
	}
	spaceID, err := parseUUIDField("space_id", req.GetSpaceId())
	if err != nil {
		return nil, err
	}
	var voiceRoomID *uuid.UUID
	if req.VoiceRoomId != nil && strings.TrimSpace(req.GetVoiceRoomId()) != "" {
		id, err := parseUUIDField("voice_room_id", req.GetVoiceRoomId())
		if err != nil {
			return nil, err
		}
		voiceRoomID = &id
	}
	rows, err := s.Store.ListVoiceRoomOverrides(ctx, spaceID, voiceRoomID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	out := make([]*rolev1.PermissionOverride, 0, len(rows))
	for _, row := range rows {
		vid := row.VoiceRoomID.String()
		out = append(out, &rolev1.PermissionOverride{
			VoiceRoomId: &vid,
			RoleId:      row.RoleID.String(),
			RoleName:    row.RoleName,
			AllowMask:   row.Allow,
			DenyMask:    row.Deny,
		})
	}
	return &rolev1.GetVoiceRoomOverridesResponse{OverrideList: &rolev1.OverrideList{Overrides: out}}, nil
}

func (s *RoleGRPC) SetDefaultJoinRole(ctx context.Context, req *rolev1.SetDefaultJoinRoleRequest) (*rolev1.SetDefaultJoinRoleResponse, error) {
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.FailedPrecondition, "role persistence not configured")
	}
	actor, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	spaceID, err := parseUUIDField("space_id", req.GetSpaceId())
	if err != nil {
		return nil, err
	}
	if err := s.requireManageRoles(ctx, spaceID, actor); err != nil {
		return nil, err
	}
	roleID, err := parseUUIDField("role_id", req.GetRoleId())
	if err != nil {
		return nil, err
	}
	if err := s.Store.SetDefaultJoinRole(ctx, spaceID, roleID); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &rolev1.SetDefaultJoinRoleResponse{}, nil
}

func (s *RoleGRPC) GetDefaultJoinRole(ctx context.Context, req *rolev1.GetDefaultJoinRoleRequest) (*rolev1.GetDefaultJoinRoleResponse, error) {
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.FailedPrecondition, "role persistence not configured")
	}
	spaceID, err := parseUUIDField("space_id", req.GetSpaceId())
	if err != nil {
		return nil, err
	}
	row, err := s.Store.GetDefaultJoinRole(ctx, spaceID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if row == nil {
		return nil, status.Error(codes.NotFound, "default join role not found")
	}
	return &rolev1.GetDefaultJoinRoleResponse{Role: roleRowToProto(row)}, nil
}
