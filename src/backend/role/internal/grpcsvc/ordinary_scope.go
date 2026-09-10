package grpcsvc

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	rolev1 "voice.app/voice/role/v1"
	"voice/backend/role/internal/roleevents"
	"voice/backend/role/internal/store"
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

func (b *ordinaryEventBuffer) PublishChatOverrideSet(ctx context.Context, chatID, roleID string) error {
	return b.add(func() error { return b.target.PublishChatOverrideSet(ctx, chatID, roleID) })
}

func (b *ordinaryEventBuffer) PublishVoiceOverrideSet(ctx context.Context, voiceRoomID, roleID string) error {
	return b.add(func() error { return b.target.PublishVoiceOverrideSet(ctx, voiceRoomID, roleID) })
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

func ordinarySpaceRPC[T any](ctx context.Context, s *RoleGRPC, rawSpaceID string, call func(*RoleGRPC) (T, error)) (T, error) {
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
	err = s.Store.WithinSpaces(ctx, []uuid.UUID{spaceID}, func(scopedStore *store.RoleStore) error {
		scoped := &RoleGRPC{Store: scopedStore, Events: buffer}
		var callErr error
		response, callErr = call(scoped)
		return callErr
	})
	if err != nil {
		return zero, ordinaryScopeError(err)
	}
	buffer.flush()
	return response, nil
}

func ordinaryRoleRPC[T any](ctx context.Context, s *RoleGRPC, rawRoleID string, call func(*RoleGRPC) (T, error)) (T, error) {
	var zero T
	if s == nil || s.Store == nil {
		return zero, status.Error(codes.FailedPrecondition, "role persistence not configured")
	}
	if s.Store.Pool == nil {
		return zero, status.Error(codes.Internal, "role persistence not configured")
	}
	roleID, err := parseUUIDField("role_id", rawRoleID)
	if err != nil {
		return zero, err
	}
	buffer := &ordinaryEventBuffer{target: s.Events}
	var response T
	err = s.Store.WithinRole(ctx, roleID, func(scopedStore *store.RoleStore) error {
		scoped := &RoleGRPC{Store: scopedStore, Events: buffer}
		var callErr error
		response, callErr = call(scoped)
		return callErr
	})
	if err != nil {
		return zero, ordinaryScopeError(err)
	}
	buffer.flush()
	return response, nil
}

func (s *RoleGRPC) BootstrapSpaceRoles(ctx context.Context, req *rolev1.BootstrapSpaceRolesRequest) (*rolev1.BootstrapSpaceRolesResponse, error) {
	return ordinarySpaceRPC(ctx, s, req.GetSpaceId(), func(scoped *RoleGRPC) (*rolev1.BootstrapSpaceRolesResponse, error) {
		return scoped.bootstrapSpaceRoles(ctx, req)
	})
}

func (s *RoleGRPC) ListRoles(ctx context.Context, req *rolev1.ListRolesRequest) (*rolev1.ListRolesResponse, error) {
	return ordinarySpaceRPC(ctx, s, req.GetSpaceId(), func(scoped *RoleGRPC) (*rolev1.ListRolesResponse, error) {
		return scoped.listRoles(ctx, req)
	})
}

func (s *RoleGRPC) CreateRole(ctx context.Context, req *rolev1.CreateRoleRequest) (*rolev1.CreateRoleResponse, error) {
	return ordinarySpaceRPC(ctx, s, req.GetSpaceId(), func(scoped *RoleGRPC) (*rolev1.CreateRoleResponse, error) {
		return scoped.createRole(ctx, req)
	})
}

func (s *RoleGRPC) AssignRole(ctx context.Context, req *rolev1.AssignRoleRequest) (*rolev1.AssignRoleResponse, error) {
	return ordinarySpaceRPC(ctx, s, req.GetSpaceId(), func(scoped *RoleGRPC) (*rolev1.AssignRoleResponse, error) {
		return scoped.assignRole(ctx, req)
	})
}

func (s *RoleGRPC) RevokeRole(ctx context.Context, req *rolev1.RevokeRoleRequest) (*rolev1.RevokeRoleResponse, error) {
	return ordinarySpaceRPC(ctx, s, req.GetSpaceId(), func(scoped *RoleGRPC) (*rolev1.RevokeRoleResponse, error) {
		return scoped.revokeRole(ctx, req)
	})
}

func (s *RoleGRPC) GetMemberRoles(ctx context.Context, req *rolev1.GetMemberRolesRequest) (*rolev1.GetMemberRolesResponse, error) {
	return ordinarySpaceRPC(ctx, s, req.GetSpaceId(), func(scoped *RoleGRPC) (*rolev1.GetMemberRolesResponse, error) {
		return scoped.getMemberRoles(ctx, req)
	})
}

func (s *RoleGRPC) CheckPermission(ctx context.Context, req *rolev1.CheckPermissionRequest) (*rolev1.CheckPermissionResponse, error) {
	return ordinarySpaceRPC(ctx, s, req.GetSpaceId(), func(scoped *RoleGRPC) (*rolev1.CheckPermissionResponse, error) {
		return scoped.checkPermission(ctx, req)
	})
}

func (s *RoleGRPC) SetChatOverride(ctx context.Context, req *rolev1.SetChatOverrideRequest) (*rolev1.SetChatOverrideResponse, error) {
	return ordinarySpaceRPC(ctx, s, req.GetSpaceId(), func(scoped *RoleGRPC) (*rolev1.SetChatOverrideResponse, error) {
		return scoped.setChatOverride(ctx, req)
	})
}

func (s *RoleGRPC) GetEffectivePermissions(ctx context.Context, req *rolev1.GetEffectivePermissionsRequest) (*rolev1.GetEffectivePermissionsResponse, error) {
	return ordinarySpaceRPC(ctx, s, req.GetSpaceId(), func(scoped *RoleGRPC) (*rolev1.GetEffectivePermissionsResponse, error) {
		return scoped.getEffectivePermissions(ctx, req)
	})
}

func (s *RoleGRPC) UpdateRole(ctx context.Context, req *rolev1.UpdateRoleRequest) (*rolev1.UpdateRoleResponse, error) {
	return ordinaryRoleRPC(ctx, s, req.GetRoleId(), func(scoped *RoleGRPC) (*rolev1.UpdateRoleResponse, error) {
		return scoped.updateRole(ctx, req)
	})
}

func (s *RoleGRPC) DeleteRole(ctx context.Context, req *rolev1.DeleteRoleRequest) (*rolev1.DeleteRoleResponse, error) {
	return ordinaryRoleRPC(ctx, s, req.GetRoleId(), func(scoped *RoleGRPC) (*rolev1.DeleteRoleResponse, error) {
		return scoped.deleteRole(ctx, req)
	})
}

func (s *RoleGRPC) ReorderRoles(ctx context.Context, req *rolev1.ReorderRolesRequest) (*rolev1.ReorderRolesResponse, error) {
	return ordinarySpaceRPC(ctx, s, req.GetSpaceId(), func(scoped *RoleGRPC) (*rolev1.ReorderRolesResponse, error) {
		return scoped.reorderRoles(ctx, req)
	})
}

func (s *RoleGRPC) RemoveChatOverride(ctx context.Context, req *rolev1.RemoveChatOverrideRequest) (*rolev1.RemoveChatOverrideResponse, error) {
	return ordinarySpaceRPC(ctx, s, req.GetSpaceId(), func(scoped *RoleGRPC) (*rolev1.RemoveChatOverrideResponse, error) {
		return scoped.removeChatOverride(ctx, req)
	})
}

func (s *RoleGRPC) GetChatOverrides(ctx context.Context, req *rolev1.GetChatOverridesRequest) (*rolev1.GetChatOverridesResponse, error) {
	return ordinarySpaceRPC(ctx, s, req.GetSpaceId(), func(scoped *RoleGRPC) (*rolev1.GetChatOverridesResponse, error) {
		return scoped.getChatOverrides(ctx, req)
	})
}

func (s *RoleGRPC) SetVoiceRoomOverride(ctx context.Context, req *rolev1.SetVoiceRoomOverrideRequest) (*rolev1.SetVoiceRoomOverrideResponse, error) {
	return ordinarySpaceRPC(ctx, s, req.GetSpaceId(), func(scoped *RoleGRPC) (*rolev1.SetVoiceRoomOverrideResponse, error) {
		return scoped.setVoiceRoomOverride(ctx, req)
	})
}

func (s *RoleGRPC) RemoveVoiceRoomOverride(ctx context.Context, req *rolev1.RemoveVoiceRoomOverrideRequest) (*rolev1.RemoveVoiceRoomOverrideResponse, error) {
	return ordinarySpaceRPC(ctx, s, req.GetSpaceId(), func(scoped *RoleGRPC) (*rolev1.RemoveVoiceRoomOverrideResponse, error) {
		return scoped.removeVoiceRoomOverride(ctx, req)
	})
}

func (s *RoleGRPC) GetVoiceRoomOverrides(ctx context.Context, req *rolev1.GetVoiceRoomOverridesRequest) (*rolev1.GetVoiceRoomOverridesResponse, error) {
	return ordinarySpaceRPC(ctx, s, req.GetSpaceId(), func(scoped *RoleGRPC) (*rolev1.GetVoiceRoomOverridesResponse, error) {
		return scoped.getVoiceRoomOverrides(ctx, req)
	})
}

func (s *RoleGRPC) SetDefaultJoinRole(ctx context.Context, req *rolev1.SetDefaultJoinRoleRequest) (*rolev1.SetDefaultJoinRoleResponse, error) {
	return ordinarySpaceRPC(ctx, s, req.GetSpaceId(), func(scoped *RoleGRPC) (*rolev1.SetDefaultJoinRoleResponse, error) {
		return scoped.setDefaultJoinRole(ctx, req)
	})
}

func (s *RoleGRPC) GetDefaultJoinRole(ctx context.Context, req *rolev1.GetDefaultJoinRoleRequest) (*rolev1.GetDefaultJoinRoleResponse, error) {
	return ordinarySpaceRPC(ctx, s, req.GetSpaceId(), func(scoped *RoleGRPC) (*rolev1.GetDefaultJoinRoleResponse, error) {
		return scoped.getDefaultJoinRole(ctx, req)
	})
}

func (s *RoleGRPC) DeleteRolesCreatedByProfile(ctx context.Context, req *rolev1.DeleteRolesCreatedByProfileRequest) (*rolev1.DeleteRolesCreatedByProfileResponse, error) {
	return ordinarySpaceRPC(ctx, s, req.GetSpaceId(), func(scoped *RoleGRPC) (*rolev1.DeleteRolesCreatedByProfileResponse, error) {
		return scoped.deleteRolesCreatedByProfile(ctx, req)
	})
}
