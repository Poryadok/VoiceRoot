package grpcsvc

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"voice/backend/role/internal/authctx"
	"voice/backend/role/permissions"
	"voice/backend/role/internal/store"

	rolev1 "voice.app/voice/role/v1"
)

func parseUUIDField(field, raw string) (uuid.UUID, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return uuid.Nil, status.Errorf(codes.InvalidArgument, "%s is required", field)
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, status.Errorf(codes.InvalidArgument, "invalid %s", field)
	}
	return id, nil
}

func roleRowToProto(r *store.RoleRow) *rolev1.Role {
	if r == nil {
		return nil
	}
	return &rolev1.Role{
		Id:              r.ID.String(),
		SpaceId:         r.SpaceID.String(),
		Name:            r.Name,
		PermissionsMask: r.PermissionsMask,
		Position:        r.Position,
		Managed:         r.Managed,
		CreatedAt:       timestamppb.Now(),
	}
}

func (s *RoleGRPC) BootstrapSpaceRoles(ctx context.Context, req *rolev1.BootstrapSpaceRolesRequest) (*rolev1.BootstrapSpaceRolesResponse, error) {
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.FailedPrecondition, "role persistence not configured")
	}
	spaceID, err := parseUUIDField("space_id", req.GetSpaceId())
	if err != nil {
		return nil, err
	}
	ownerID, err := parseUUIDField("owner_profile_id", req.GetOwnerProfileId())
	if err != nil {
		return nil, err
	}
	if err := s.Store.BootstrapSpaceRoles(ctx, spaceID, ownerID); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if s.Events != nil {
		roles, _ := s.Store.ListRoles(ctx, spaceID)
		for _, r := range roles {
			_ = s.Events.PublishRoleCreated(ctx, spaceID.String(), r.ID.String(), r.Name)
		}
	}
	return &rolev1.BootstrapSpaceRolesResponse{}, nil
}

func (s *RoleGRPC) ListRoles(ctx context.Context, req *rolev1.ListRolesRequest) (*rolev1.ListRolesResponse, error) {
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.FailedPrecondition, "role persistence not configured")
	}
	spaceID, err := parseUUIDField("space_id", req.GetSpaceId())
	if err != nil {
		return nil, err
	}
	rows, err := s.Store.ListRoles(ctx, spaceID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	out := make([]*rolev1.Role, 0, len(rows))
	for i := range rows {
		out = append(out, roleRowToProto(&rows[i]))
	}
	return &rolev1.ListRolesResponse{RoleList: &rolev1.RoleList{Roles: out}}, nil
}

func (s *RoleGRPC) CreateRole(ctx context.Context, req *rolev1.CreateRoleRequest) (*rolev1.CreateRoleResponse, error) {
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
	name := strings.TrimSpace(req.GetName())
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	row, err := s.Store.CreateCustomRole(ctx, spaceID, name, req.GetPermissionsMask(), req.GetPosition())
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if s.Events != nil {
		_ = s.Events.PublishRoleCreated(ctx, spaceID.String(), row.ID.String(), row.Name)
	}
	return &rolev1.CreateRoleResponse{Role: roleRowToProto(row)}, nil
}

func (s *RoleGRPC) AssignRole(ctx context.Context, req *rolev1.AssignRoleRequest) (*rolev1.AssignRoleResponse, error) {
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
	profileID, err := parseUUIDField("profile_id", req.GetProfileId())
	if err != nil {
		return nil, err
	}
	roleID, err := parseUUIDField("role_id", req.GetRoleId())
	if err != nil {
		return nil, err
	}
	can, err := s.Store.CanManageRole(ctx, spaceID, actor, roleID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !can {
		return nil, status.Error(codes.PermissionDenied, "cannot assign this role")
	}
	if err := s.Store.AssignMemberRole(ctx, spaceID, profileID, roleID, actor); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if s.Events != nil {
		_ = s.Events.PublishRoleAssigned(ctx, spaceID.String(), profileID.String(), roleID.String())
	}
	return &rolev1.AssignRoleResponse{}, nil
}

func (s *RoleGRPC) RevokeRole(ctx context.Context, req *rolev1.RevokeRoleRequest) (*rolev1.RevokeRoleResponse, error) {
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
	profileID, err := parseUUIDField("profile_id", req.GetProfileId())
	if err != nil {
		return nil, err
	}
	roleID, err := parseUUIDField("role_id", req.GetRoleId())
	if err != nil {
		return nil, err
	}
	can, err := s.Store.CanManageRole(ctx, spaceID, actor, roleID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !can {
		return nil, status.Error(codes.PermissionDenied, "cannot revoke this role")
	}
	if err := s.Store.RevokeMemberRole(ctx, spaceID, profileID, roleID); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if s.Events != nil {
		_ = s.Events.PublishRoleRevoked(ctx, spaceID.String(), profileID.String(), roleID.String())
	}
	return &rolev1.RevokeRoleResponse{}, nil
}

func (s *RoleGRPC) GetMemberRoles(ctx context.Context, req *rolev1.GetMemberRolesRequest) (*rolev1.GetMemberRolesResponse, error) {
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.FailedPrecondition, "role persistence not configured")
	}
	spaceID, err := parseUUIDField("space_id", req.GetSpaceId())
	if err != nil {
		return nil, err
	}
	profileID, err := parseUUIDField("profile_id", req.GetProfileId())
	if err != nil {
		return nil, err
	}
	rows, err := s.Store.GetMemberRoles(ctx, spaceID, profileID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	out := make([]*rolev1.Role, 0, len(rows))
	for i := range rows {
		out = append(out, roleRowToProto(&rows[i]))
	}
	return &rolev1.GetMemberRolesResponse{RoleList: &rolev1.RoleList{Roles: out}}, nil
}

func (s *RoleGRPC) CheckPermission(ctx context.Context, req *rolev1.CheckPermissionRequest) (*rolev1.CheckPermissionResponse, error) {
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.FailedPrecondition, "role persistence not configured")
	}
	spaceID, err := parseUUIDField("space_id", req.GetSpaceId())
	if err != nil {
		return nil, err
	}
	profileID, err := parseUUIDField("profile_id", req.GetProfileId())
	if err != nil {
		return nil, err
	}
	var chatID, voiceRoomID *uuid.UUID
	if req.GetChat() != nil && strings.TrimSpace(req.GetChat().GetId()) != "" {
		id, err := parseUUIDField("chat_id", req.GetChat().GetId())
		if err != nil {
			return nil, err
		}
		chatID = &id
	}
	if req.VoiceRoomId != nil && strings.TrimSpace(req.GetVoiceRoomId()) != "" {
		id, err := parseUUIDField("voice_room_id", req.GetVoiceRoomId())
		if err != nil {
			return nil, err
		}
		voiceRoomID = &id
	}
	mask, err := s.Store.GetEffectiveMask(ctx, spaceID, profileID, chatID, voiceRoomID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	allowed, err := permissions.HasPermission(mask, req.GetPermissionName())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	return &rolev1.CheckPermissionResponse{Allowed: allowed}, nil
}

func (s *RoleGRPC) SetChatOverride(ctx context.Context, req *rolev1.SetChatOverrideRequest) (*rolev1.SetChatOverrideResponse, error) {
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
	row, err := s.Store.GetRoleByID(ctx, roleID)
	if err != nil || row == nil || row.SpaceID != spaceID {
		return nil, status.Error(codes.NotFound, "role not found")
	}
	if err := s.Store.SetChatOverride(ctx, chatID, roleID, req.GetAllowMask(), req.GetDenyMask()); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if s.Events != nil {
		_ = s.Events.PublishChatOverrideSet(ctx, chatID.String(), roleID.String())
	}
	return &rolev1.SetChatOverrideResponse{}, nil
}

func (s *RoleGRPC) GetEffectivePermissions(ctx context.Context, req *rolev1.GetEffectivePermissionsRequest) (*rolev1.GetEffectivePermissionsResponse, error) {
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.FailedPrecondition, "role persistence not configured")
	}
	spaceID, err := parseUUIDField("space_id", req.GetSpaceId())
	if err != nil {
		return nil, err
	}
	profileID, err := parseUUIDField("profile_id", req.GetProfileId())
	if err != nil {
		return nil, err
	}
	var chatID, voiceRoomID *uuid.UUID
	if req.GetChat() != nil && strings.TrimSpace(req.GetChat().GetId()) != "" {
		id, err := parseUUIDField("chat_id", req.GetChat().GetId())
		if err != nil {
			return nil, err
		}
		chatID = &id
	}
	if req.VoiceRoomId != nil && strings.TrimSpace(req.GetVoiceRoomId()) != "" {
		id, err := parseUUIDField("voice_room_id", req.GetVoiceRoomId())
		if err != nil {
			return nil, err
		}
		voiceRoomID = &id
	}
	mask, err := s.Store.GetEffectiveMask(ctx, spaceID, profileID, chatID, voiceRoomID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &rolev1.GetEffectivePermissionsResponse{
		PermissionSet: &rolev1.PermissionSet{
			EffectiveMask:   mask,
			PermissionNames: permissions.NamesFor(mask),
		},
	}, nil
}
