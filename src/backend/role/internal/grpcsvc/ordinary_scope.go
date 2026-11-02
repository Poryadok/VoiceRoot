package grpcsvc

import (
	"context"
	"errors"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/principal"
	"voice/backend/role/internal/principalgrpc"
	"voice/backend/role/internal/roleevents"
	"voice/backend/role/internal/store"
	"voice/backend/role/permissions"
)

// ordinaryEventBuffer defers every ordinary domain event until the database
// transaction that produced it has committed successfully.
type ordinaryEventBuffer struct {
	target roleevents.Publisher
	events []func() error
}

func (b *ordinaryEventBuffer) add(event func() error) error {
	if b.target != nil {
		b.events = append(b.events, event)
	}
	return nil
}

func (b *ordinaryEventBuffer) flush() {
	for _, event := range b.events {
		_ = event()
	}
}

func (b *ordinaryEventBuffer) PublishRoleCreated(ctx context.Context, spaceID, roleID, name string) error {
	return b.add(func() error { return b.target.PublishRoleCreated(ctx, spaceID, roleID, name) })
}

func (b *ordinaryEventBuffer) PublishRoleUpdated(ctx context.Context, spaceID, roleID string, changedFields []string) error {
	fields := append([]string(nil), changedFields...)
	return b.add(func() error { return b.target.PublishRoleUpdated(ctx, spaceID, roleID, fields) })
}

func (b *ordinaryEventBuffer) PublishRoleDeleted(ctx context.Context, spaceID, roleID string) error {
	return b.add(func() error { return b.target.PublishRoleDeleted(ctx, spaceID, roleID) })
}

func (b *ordinaryEventBuffer) PublishRoleAssigned(ctx context.Context, spaceID, profileID, roleID string) error {
	return b.add(func() error { return b.target.PublishRoleAssigned(ctx, spaceID, profileID, roleID) })
}

func (b *ordinaryEventBuffer) PublishRoleRevoked(ctx context.Context, spaceID, profileID, roleID string) error {
	return b.add(func() error { return b.target.PublishRoleRevoked(ctx, spaceID, profileID, roleID) })
}

func (b *ordinaryEventBuffer) PublishChatOverrideSet(ctx context.Context, spaceID, chatID, roleID string) error {
	return b.add(func() error { return b.target.PublishChatOverrideSet(ctx, spaceID, chatID, roleID) })
}

func (b *ordinaryEventBuffer) PublishChatOverrideRemoved(ctx context.Context, spaceID, chatID, roleID string) error {
	return b.add(func() error { return b.target.PublishChatOverrideRemoved(ctx, spaceID, chatID, roleID) })
}

func (b *ordinaryEventBuffer) PublishVoiceOverrideSet(ctx context.Context, spaceID, voiceRoomID, roleID string) error {
	return b.add(func() error { return b.target.PublishVoiceOverrideSet(ctx, spaceID, voiceRoomID, roleID) })
}

func (b *ordinaryEventBuffer) PublishVoiceOverrideRemoved(ctx context.Context, spaceID, voiceRoomID, roleID string) error {
	return b.add(func() error { return b.target.PublishVoiceOverrideRemoved(ctx, spaceID, voiceRoomID, roleID) })
}

func ordinaryScopeError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, store.ErrSpaceFrozen) || errors.Is(err, store.ErrSpaceRetired) || errors.Is(err, store.ErrScopeUnavailable) {
		return status.Error(codes.Unavailable, err.Error())
	}
	if errors.Is(err, store.ErrRoleNotFound) {
		return status.Error(codes.NotFound, "role not found")
	}
	return err
}

func ordinaryStoreError(err error) error {
	if errors.Is(err, store.ErrSpaceFrozen) || errors.Is(err, store.ErrSpaceRetired) || errors.Is(err, store.ErrScopeUnavailable) {
		return err
	}
	return status.Error(codes.Internal, err.Error())
}

func ordinarySpaceRPC[T any](ctx context.Context, s *RoleGRPC, rawSpaceID string, call func(context.Context, *RoleGRPC) (T, error), targets ...string) (T, error) {
	var zero T
	if s == nil || s.Store == nil {
		return zero, status.Error(codes.FailedPrecondition, "role persistence not configured")
	}
	if s.Store.Pool == nil {
		return zero, status.Error(codes.Internal, "role persistence not configured")
	}
	spaceID, err := parseUUIDField("space_id", rawSpaceID)
	if err != nil {
		return zero, err
	}
	buffer := &ordinaryEventBuffer{target: s.Events}
	var response T
	err = withNativeOrdinary(ctx, s, spaceID, targets, func(callCtx context.Context, scopedStore *store.RoleStore) error {
		scoped := *s
		scoped.Store = scopedStore
		scoped.Events = buffer
		var callErr error
		response, callErr = call(callCtx, &scoped)
		return callErr
	})
	if err != nil {
		return zero, ordinaryScopeError(err)
	}
	buffer.flush()
	return response, nil
}

func ordinaryRoleRPC[T any](ctx context.Context, s *RoleGRPC, rawRoleID string, call func(context.Context, *RoleGRPC) (T, error)) (T, error) {
	var zero T
	if s == nil || s.Store == nil {
		return zero, status.Error(codes.Unavailable, "Role persistence unavailable")
	}
	roleID, err := parseUUIDField("role_id", rawRoleID)
	if err != nil {
		return zero, err
	}
	role, err := s.Store.GetRoleByID(ctx, roleID)
	if err != nil {
		return zero, ordinaryScopeError(err)
	}
	if role == nil {
		return zero, status.Error(codes.NotFound, "role not found")
	}
	return ordinarySpaceRPC(ctx, s, role.SpaceID.String(), call)
}
func (s *RoleGRPC) BootstrapSpaceRoles(ctx context.Context, req *rolev1.BootstrapSpaceRolesRequest) (*rolev1.BootstrapSpaceRolesResponse, error) {
	return ordinarySpaceRPC(ctx, s, req.GetSpaceId(), func(callCtx context.Context, scoped *RoleGRPC) (*rolev1.BootstrapSpaceRolesResponse, error) {
		return scoped.bootstrapSpaceRoles(callCtx, req)
	})
}

func (s *RoleGRPC) ListRoles(ctx context.Context, req *rolev1.ListRolesRequest) (*rolev1.ListRolesResponse, error) {
	return ordinarySpaceRPC(ctx, s, req.GetSpaceId(), func(callCtx context.Context, scoped *RoleGRPC) (*rolev1.ListRolesResponse, error) {
		return scoped.listRoles(callCtx, req)
	})
}

func (s *RoleGRPC) CreateRole(ctx context.Context, req *rolev1.CreateRoleRequest) (*rolev1.CreateRoleResponse, error) {
	return ordinarySpaceRPC(ctx, s, req.GetSpaceId(), func(callCtx context.Context, scoped *RoleGRPC) (*rolev1.CreateRoleResponse, error) {
		return scoped.createRole(callCtx, req)
	})
}

func (s *RoleGRPC) AssignRole(ctx context.Context, req *rolev1.AssignRoleRequest) (*rolev1.AssignRoleResponse, error) {
	return ordinarySpaceRPC(ctx, s, req.GetSpaceId(), func(callCtx context.Context, scoped *RoleGRPC) (*rolev1.AssignRoleResponse, error) {
		return scoped.assignRole(callCtx, req)
	}, req.GetProfileId())
}

func (s *RoleGRPC) RevokeRole(ctx context.Context, req *rolev1.RevokeRoleRequest) (*rolev1.RevokeRoleResponse, error) {
	return ordinarySpaceRPC(ctx, s, req.GetSpaceId(), func(callCtx context.Context, scoped *RoleGRPC) (*rolev1.RevokeRoleResponse, error) {
		return scoped.revokeRole(callCtx, req)
	}, req.GetProfileId())
}

func (s *RoleGRPC) GetMemberRoles(ctx context.Context, req *rolev1.GetMemberRolesRequest) (*rolev1.GetMemberRolesResponse, error) {
	return ordinarySpaceRPC(ctx, s, req.GetSpaceId(), func(callCtx context.Context, scoped *RoleGRPC) (*rolev1.GetMemberRolesResponse, error) {
		return scoped.getMemberRoles(callCtx, req)
	}, req.GetProfileId())
}

func (s *RoleGRPC) CheckPermission(ctx context.Context, req *rolev1.CheckPermissionRequest) (*rolev1.CheckPermissionResponse, error) {
	if req.GetPermissionName() == permissions.MemberBan {
		if err := principalgrpc.RequireBanPermission(ctx, req); err != nil {
			return nil, err
		}
		response, err := s.checkQualifiedSpaceDecision(ctx, req, permissions.MemberBan)
		if err != nil {
			return nil, err
		}
		p, _ := principal.FromContext(ctx)
		if ctx.Err() != nil || !p.ExpiresAt.After(time.Now()) {
			return nil, status.Error(codes.Unavailable, "ban permission expired")
		}
		return response, nil
	}
	if p, verified := principal.FromContext(ctx); verified && (p.Issuer == "messaging" || p.Issuer == "voice") {
		if err := principalgrpc.RequireWorkloadDecision(ctx, req); err != nil {
			return nil, err
		}
		ctx = context.WithValue(ctx, nativeMemberDecisionReadKey{}, true)
		if req.Chat != nil {
			ctx = context.WithValue(ctx, nativeMemberChatScopeKey{}, nativeMemberChatScope{profile: req.ProfileId, chat: req.Chat.Id})
		}
		response, err := ordinarySpaceRPC(ctx, s, req.SpaceId, func(callCtx context.Context, scoped *RoleGRPC) (*rolev1.CheckPermissionResponse, error) {
			return scoped.checkPermission(callCtx, req)
		}, req.ProfileId)
		if err != nil {
			return nil, err
		}
		if ctx.Err() != nil || !p.ExpiresAt.After(time.Now()) {
			return nil, status.Error(codes.Unavailable, "workload decision expired")
		}
		return response, nil
	}
	if req.GetPermissionName() == "SPACE_MANAGE_INVITES" {
		return s.checkQualifiedInvites(ctx, req)
	}
	if req.GetPermissionName() == "SPACE_MANAGE_BOTS" {
		return s.checkQualifiedBotManagement(ctx, req)
	}
	if req.GetPermissionName() == "SPACE_APPROVE_JOIN_REQUESTS" {
		return s.checkQualifiedJoinApproval(ctx, req)
	}
	if req.GetPermissionName() == "SPACE_MANAGE_SETTINGS" {
		return s.checkQualifiedSettings(ctx, req)
	}
	if req.GetPermissionName() == "SPACE_MANAGE_MATCHMAKING" {
		if err := principalgrpc.RequireMatchmakingPermission(ctx, req); err != nil {
			return nil, err
		}
		response, err := s.checkQualifiedSpaceDecision(ctx, req, "SPACE_MANAGE_MATCHMAKING")
		if err != nil {
			return nil, err
		}
		p, _ := principal.FromContext(ctx)
		if ctx.Err() != nil || !p.ExpiresAt.After(time.Now()) {
			return nil, status.Error(codes.Unavailable, "matchmaking permission expired")
		}
		return response, nil
	}
	if p, verified := principal.FromContext(ctx); verified && (p.Issuer == "chat" || req.GetPermissionName() == "TEXT_CHAT_VIEW") {
		if err := principalgrpc.RequireChatCurrentACL(ctx, req); err != nil {
			return nil, err
		}
		ctx = context.WithValue(ctx, nativeMemberChatScopeKey{}, nativeMemberChatScope{profile: req.GetProfileId(), chat: req.GetChat().GetId(), voicePresent: req.VoiceRoomId != nil})
		response, err := ordinarySpaceRPC(ctx, s, req.GetSpaceId(), func(ctx context.Context, scoped *RoleGRPC) (*rolev1.CheckPermissionResponse, error) {
			return scoped.checkPermission(ctx, req)
		}, req.GetProfileId())
		if err != nil {
			return nil, err
		}
		// Read only: after the owning COMMIT, the original credential must still
		// be live. This never reinterprets a committed mutation or recovery fact.
		if ctx.Err() != nil || !p.ExpiresAt.After(time.Now()) {
			return nil, status.Error(codes.Unavailable, "Chat ACL decision expired")
		}
		return response, nil
	}
	if req.GetPermissionName() == "MEMBER_MANAGE_NICKNAMES" {
		if err := principalgrpc.RequireNicknamePermission(ctx, req); err != nil {
			return nil, err
		}
	}
	return ordinarySpaceRPC(ctx, s, req.GetSpaceId(), func(callCtx context.Context, scoped *RoleGRPC) (*rolev1.CheckPermissionResponse, error) {
		return scoped.checkPermission(callCtx, req)
	}, req.GetProfileId())
}

func (s *RoleGRPC) SetChatOverride(ctx context.Context, req *rolev1.SetChatOverrideRequest) (*rolev1.SetChatOverrideResponse, error) {
	spaceID, chatID, err := s.preflightOverrideTarget(ctx, req.GetSpaceId(), req.GetRoleId(), req.GetChat().GetId())
	if ctx.Err() != nil {
		return nil, status.FromContextError(ctx.Err()).Err()
	}
	if err != nil {
		return nil, err
	}
	if s.OverrideTargets == nil {
		return nil, status.Error(codes.Unavailable, "target validation unavailable")
	}
	if err := s.OverrideTargets.ValidateChat(ctx, spaceID.String(), chatID.String()); err != nil {
		return nil, targetValidationError(ctx, err)
	}
	if ctx.Err() != nil {
		return nil, status.FromContextError(ctx.Err()).Err()
	}
	response, err := ordinarySpaceRPC(ctx, s, req.GetSpaceId(), func(callCtx context.Context, scoped *RoleGRPC) (*rolev1.SetChatOverrideResponse, error) {
		return scoped.setChatOverride(callCtx, req)
	})
	if err != nil && ctx.Err() != nil {
		return nil, status.FromContextError(ctx.Err()).Err()
	}
	return response, err
}

func (s *RoleGRPC) GetEffectivePermissions(ctx context.Context, req *rolev1.GetEffectivePermissionsRequest) (*rolev1.GetEffectivePermissionsResponse, error) {
	if chat := req.GetChat(); chat != nil {
		ctx = context.WithValue(ctx, nativeMemberChatScopeKey{}, nativeMemberChatScope{profile: req.GetProfileId(), chat: chat.GetId(), voicePresent: req.VoiceRoomId != nil})
	}
	return ordinarySpaceRPC(ctx, s, req.GetSpaceId(), func(callCtx context.Context, scoped *RoleGRPC) (*rolev1.GetEffectivePermissionsResponse, error) {
		return scoped.getEffectivePermissions(callCtx, req)
	}, req.GetProfileId())
}

func (s *RoleGRPC) UpdateRole(ctx context.Context, req *rolev1.UpdateRoleRequest) (*rolev1.UpdateRoleResponse, error) {
	return ordinaryRoleRPC(ctx, s, req.GetRoleId(), func(callCtx context.Context, scoped *RoleGRPC) (*rolev1.UpdateRoleResponse, error) {
		return scoped.updateRole(callCtx, req)
	})
}

func (s *RoleGRPC) DeleteRole(ctx context.Context, req *rolev1.DeleteRoleRequest) (*rolev1.DeleteRoleResponse, error) {
	return ordinaryRoleRPC(ctx, s, req.GetRoleId(), func(callCtx context.Context, scoped *RoleGRPC) (*rolev1.DeleteRoleResponse, error) {
		return scoped.deleteRole(callCtx, req)
	})
}

func (s *RoleGRPC) ReorderRoles(ctx context.Context, req *rolev1.ReorderRolesRequest) (*rolev1.ReorderRolesResponse, error) {
	return ordinarySpaceRPC(ctx, s, req.GetSpaceId(), func(callCtx context.Context, scoped *RoleGRPC) (*rolev1.ReorderRolesResponse, error) {
		return scoped.reorderRoles(callCtx, req)
	})
}

func (s *RoleGRPC) RemoveChatOverride(ctx context.Context, req *rolev1.RemoveChatOverrideRequest) (*rolev1.RemoveChatOverrideResponse, error) {
	return ordinarySpaceRPC(ctx, s, req.GetSpaceId(), func(callCtx context.Context, scoped *RoleGRPC) (*rolev1.RemoveChatOverrideResponse, error) {
		return scoped.removeChatOverride(callCtx, req)
	})
}

func (s *RoleGRPC) GetChatOverrides(ctx context.Context, req *rolev1.GetChatOverridesRequest) (*rolev1.GetChatOverridesResponse, error) {
	return ordinarySpaceRPC(ctx, s, req.GetSpaceId(), func(callCtx context.Context, scoped *RoleGRPC) (*rolev1.GetChatOverridesResponse, error) {
		return scoped.getChatOverrides(callCtx, req)
	})
}

func (s *RoleGRPC) SetVoiceRoomOverride(ctx context.Context, req *rolev1.SetVoiceRoomOverrideRequest) (*rolev1.SetVoiceRoomOverrideResponse, error) {
	spaceID, roomID, err := s.preflightOverrideTarget(ctx, req.GetSpaceId(), req.GetRoleId(), req.GetVoiceRoomId())
	if ctx.Err() != nil {
		return nil, status.FromContextError(ctx.Err()).Err()
	}
	if err != nil {
		return nil, err
	}
	if s.OverrideTargets == nil {
		return nil, status.Error(codes.Unavailable, "target validation unavailable")
	}
	if err := s.OverrideTargets.ValidateVoice(ctx, spaceID.String(), roomID.String()); err != nil {
		return nil, targetValidationError(ctx, err)
	}
	if ctx.Err() != nil {
		return nil, status.FromContextError(ctx.Err()).Err()
	}
	response, err := ordinarySpaceRPC(ctx, s, req.GetSpaceId(), func(callCtx context.Context, scoped *RoleGRPC) (*rolev1.SetVoiceRoomOverrideResponse, error) {
		return scoped.setVoiceRoomOverride(callCtx, req)
	})
	if err != nil && ctx.Err() != nil {
		return nil, status.FromContextError(ctx.Err()).Err()
	}
	return response, err
}

func (s *RoleGRPC) RemoveVoiceRoomOverride(ctx context.Context, req *rolev1.RemoveVoiceRoomOverrideRequest) (*rolev1.RemoveVoiceRoomOverrideResponse, error) {
	return ordinarySpaceRPC(ctx, s, req.GetSpaceId(), func(callCtx context.Context, scoped *RoleGRPC) (*rolev1.RemoveVoiceRoomOverrideResponse, error) {
		return scoped.removeVoiceRoomOverride(callCtx, req)
	})
}

func (s *RoleGRPC) GetVoiceRoomOverrides(ctx context.Context, req *rolev1.GetVoiceRoomOverridesRequest) (*rolev1.GetVoiceRoomOverridesResponse, error) {
	return ordinarySpaceRPC(ctx, s, req.GetSpaceId(), func(callCtx context.Context, scoped *RoleGRPC) (*rolev1.GetVoiceRoomOverridesResponse, error) {
		return scoped.getVoiceRoomOverrides(callCtx, req)
	})
}

func (s *RoleGRPC) SetDefaultJoinRole(ctx context.Context, req *rolev1.SetDefaultJoinRoleRequest) (*rolev1.SetDefaultJoinRoleResponse, error) {
	return ordinarySpaceRPC(ctx, s, req.GetSpaceId(), func(callCtx context.Context, scoped *RoleGRPC) (*rolev1.SetDefaultJoinRoleResponse, error) {
		return scoped.setDefaultJoinRole(callCtx, req)
	})
}

func (s *RoleGRPC) GetDefaultJoinRole(ctx context.Context, req *rolev1.GetDefaultJoinRoleRequest) (*rolev1.GetDefaultJoinRoleResponse, error) {
	return ordinarySpaceRPC(ctx, s, req.GetSpaceId(), func(callCtx context.Context, scoped *RoleGRPC) (*rolev1.GetDefaultJoinRoleResponse, error) {
		return scoped.getDefaultJoinRole(callCtx, req)
	})
}

func (s *RoleGRPC) DeleteRolesCreatedByProfile(ctx context.Context, req *rolev1.DeleteRolesCreatedByProfileRequest) (*rolev1.DeleteRolesCreatedByProfileResponse, error) {
	return ordinarySpaceRPC(ctx, s, req.GetSpaceId(), func(callCtx context.Context, scoped *RoleGRPC) (*rolev1.DeleteRolesCreatedByProfileResponse, error) {
		return scoped.deleteRolesCreatedByProfile(callCtx, req)
	})
}
