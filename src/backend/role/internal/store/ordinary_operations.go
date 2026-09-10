package store

import (
	"context"
	"errors"
	"github.com/google/uuid"
)

func spaceValue[T any](ctx context.Context, s *RoleStore, spaceID uuid.UUID, fn func(*RoleStore) (T, error)) (T, error) {
	var value T
	err := s.WithinSpaces(ctx, []uuid.UUID{spaceID}, func(scoped *RoleStore) error { var err error; value, err = fn(scoped); return err })
	if err != nil {
		var zero T
		return zero, err
	}
	return value, nil
}

func roleValue[T any](ctx context.Context, s *RoleStore, roleID uuid.UUID, fn func(*RoleStore) (T, error)) (T, error) {
	var value T
	err := s.WithinRole(ctx, roleID, func(scoped *RoleStore) error { var err error; value, err = fn(scoped); return err })
	if err != nil {
		var zero T
		return zero, err
	}
	return value, nil
}

// BootstrapSystemRoles executes within its ordinary space transaction.
func (s *RoleStore) BootstrapSystemRoles(ctx context.Context, spaceID uuid.UUID) error {
	return s.WithinSpaces(ctx, []uuid.UUID{spaceID}, func(scoped *RoleStore) error { return scoped.unscopedBootstrapSystemRoles(ctx, spaceID) })
}

// BootstrapSpaceRoles executes within its ordinary space transaction.
func (s *RoleStore) BootstrapSpaceRoles(ctx context.Context, spaceID, ownerProfileID uuid.UUID) error {
	return s.WithinSpaces(ctx, []uuid.UUID{spaceID}, func(scoped *RoleStore) error { return scoped.unscopedBootstrapSpaceRoles(ctx, spaceID, ownerProfileID) })
}

// BootstrapSpaceRolesWithCreatedSystemRoles executes within its ordinary space transaction.
func (s *RoleStore) BootstrapSpaceRolesWithCreatedSystemRoles(ctx context.Context, spaceID, ownerProfileID uuid.UUID) ([]RoleRow, error) {
	return spaceValue(ctx, s, spaceID, func(scoped *RoleStore) ([]RoleRow, error) {
		return scoped.unscopedBootstrapSpaceRolesWithCreatedSystemRoles(ctx, spaceID, ownerProfileID)
	})
}

// ListRoles executes within its ordinary space transaction.
func (s *RoleStore) ListRoles(ctx context.Context, spaceID uuid.UUID) ([]RoleRow, error) {
	return spaceValue(ctx, s, spaceID, func(scoped *RoleStore) ([]RoleRow, error) { return scoped.unscopedListRoles(ctx, spaceID) })
}

// GetRoleByID executes within its ordinary space transaction.
func (s *RoleStore) GetRoleByID(ctx context.Context, roleID uuid.UUID) (*RoleRow, error) {
	value, err := roleValue(ctx, s, roleID, func(scoped *RoleStore) (*RoleRow, error) { return scoped.unscopedGetRoleByID(ctx, roleID) })
	if errors.Is(err, ErrRoleNotFound) {
		return nil, nil
	}
	return value, err
}

// GetRoleByIDInSpace resolves a request-supplied role without expanding the
// transaction beyond the already fenced request space.
func (s *RoleStore) GetRoleByIDInSpace(ctx context.Context, spaceID, roleID uuid.UUID) (*RoleRow, error) {
	return spaceValue(ctx, s, spaceID, func(scoped *RoleStore) (*RoleRow, error) {
		row, err := scoped.unscopedGetRoleByID(ctx, roleID)
		if err != nil || row == nil {
			return row, err
		}
		if row.SpaceID != spaceID {
			return nil, nil
		}
		return row, nil
	})
}

// AssignMemberRole executes within its ordinary space transaction.
func (s *RoleStore) AssignMemberRole(ctx context.Context, spaceID, profileID, roleID, assignedBy uuid.UUID) error {
	return s.WithinSpaces(ctx, []uuid.UUID{spaceID}, func(scoped *RoleStore) error {
		return scoped.unscopedAssignMemberRole(ctx, spaceID, profileID, roleID, assignedBy)
	})
}

// RevokeMemberRole executes within its ordinary space transaction.
func (s *RoleStore) RevokeMemberRole(ctx context.Context, spaceID, profileID, roleID uuid.UUID) error {
	return s.WithinSpaces(ctx, []uuid.UUID{spaceID}, func(scoped *RoleStore) error { return scoped.unscopedRevokeMemberRole(ctx, spaceID, profileID, roleID) })
}

// GetMemberRoles executes within its ordinary space transaction.
func (s *RoleStore) GetMemberRoles(ctx context.Context, spaceID, profileID uuid.UUID) ([]RoleRow, error) {
	return spaceValue(ctx, s, spaceID, func(scoped *RoleStore) ([]RoleRow, error) {
		return scoped.unscopedGetMemberRoles(ctx, spaceID, profileID)
	})
}

// GetEffectiveMask executes within its ordinary space transaction.
func (s *RoleStore) GetEffectiveMask(ctx context.Context, spaceID, profileID uuid.UUID, chatID, voiceRoomID *uuid.UUID) (uint64, error) {
	return spaceValue(ctx, s, spaceID, func(scoped *RoleStore) (uint64, error) {
		return scoped.unscopedGetEffectiveMask(ctx, spaceID, profileID, chatID, voiceRoomID)
	})
}

// CanManageRole executes within its ordinary space transaction.
func (s *RoleStore) CanManageRole(ctx context.Context, spaceID, actorProfileID, targetRoleID uuid.UUID) (bool, error) {
	return spaceValue(ctx, s, spaceID, func(scoped *RoleStore) (bool, error) {
		return scoped.unscopedCanManageRole(ctx, spaceID, actorProfileID, targetRoleID)
	})
}

// CanCreateRole executes within its ordinary space transaction.
func (s *RoleStore) CanCreateRole(ctx context.Context, spaceID, actorProfileID uuid.UUID, position int32) (bool, error) {
	return spaceValue(ctx, s, spaceID, func(scoped *RoleStore) (bool, error) {
		return scoped.unscopedCanCreateRole(ctx, spaceID, actorProfileID, position)
	})
}

// SetChatOverride executes within its ordinary space transaction.
func (s *RoleStore) SetChatOverride(ctx context.Context, chatID, roleID uuid.UUID, allow, deny uint64) error {
	return s.WithinRole(ctx, roleID, func(scoped *RoleStore) error { return scoped.unscopedSetChatOverride(ctx, chatID, roleID, allow, deny) })
}

// SetChatOverrideForMemberRoles executes within its ordinary space transaction.
func (s *RoleStore) SetChatOverrideForMemberRoles(ctx context.Context, spaceID uuid.UUID, chatID uuid.UUID, profileID uuid.UUID, allow, deny uint64) error {
	return s.WithinSpaces(ctx, []uuid.UUID{spaceID}, func(scoped *RoleStore) error {
		return scoped.unscopedSetChatOverrideForMemberRoles(ctx, spaceID, chatID, profileID, allow, deny)
	})
}

// SetVoiceRoomOverride executes within its ordinary space transaction.
func (s *RoleStore) SetVoiceRoomOverride(ctx context.Context, voiceRoomID, roleID uuid.UUID, allow, deny uint64) error {
	return s.WithinRole(ctx, roleID, func(scoped *RoleStore) error {
		return scoped.unscopedSetVoiceRoomOverride(ctx, voiceRoomID, roleID, allow, deny)
	})
}

// CreateCustomRole executes within its ordinary space transaction.
func (s *RoleStore) CreateCustomRole(ctx context.Context, spaceID uuid.UUID, name string, permissionsMask uint64, position int32, createdByProfileID *uuid.UUID) (*RoleRow, error) {
	return spaceValue(ctx, s, spaceID, func(scoped *RoleStore) (*RoleRow, error) {
		return scoped.unscopedCreateCustomRole(ctx, spaceID, name, permissionsMask, position, createdByProfileID)
	})
}

// UpdateRole executes within its ordinary space transaction.
func (s *RoleStore) UpdateRole(ctx context.Context, roleID uuid.UUID, name *string, permissionsMask *uint64, position *int32) (*RoleRow, error) {
	return roleValue(ctx, s, roleID, func(scoped *RoleStore) (*RoleRow, error) {
		return scoped.unscopedUpdateRole(ctx, roleID, name, permissionsMask, position)
	})
}

// DeleteRolesCreatedByProfile executes within its ordinary space transaction.
func (s *RoleStore) DeleteRolesCreatedByProfile(ctx context.Context, spaceID, profileID uuid.UUID) (int64, error) {
	return spaceValue(ctx, s, spaceID, func(scoped *RoleStore) (int64, error) {
		return scoped.unscopedDeleteRolesCreatedByProfile(ctx, spaceID, profileID)
	})
}

// DeleteRole executes within its ordinary space transaction.
func (s *RoleStore) DeleteRole(ctx context.Context, roleID uuid.UUID) error {
	return s.WithinRole(ctx, roleID, func(scoped *RoleStore) error { return scoped.unscopedDeleteRole(ctx, roleID) })
}

// ReorderRoles executes within its ordinary space transaction.
func (s *RoleStore) ReorderRoles(ctx context.Context, spaceID uuid.UUID, orderedRoleIDs []uuid.UUID) error {
	return s.WithinSpaces(ctx, []uuid.UUID{spaceID}, func(scoped *RoleStore) error { return scoped.unscopedReorderRoles(ctx, spaceID, orderedRoleIDs) })
}

// ListChatOverrides executes within its ordinary space transaction.
func (s *RoleStore) ListChatOverrides(ctx context.Context, spaceID uuid.UUID, chatID *uuid.UUID) ([]OverrideRow, error) {
	return spaceValue(ctx, s, spaceID, func(scoped *RoleStore) ([]OverrideRow, error) {
		return scoped.unscopedListChatOverrides(ctx, spaceID, chatID)
	})
}

// ListVoiceRoomOverrides executes within its ordinary space transaction.
func (s *RoleStore) ListVoiceRoomOverrides(ctx context.Context, spaceID uuid.UUID, voiceRoomID *uuid.UUID) ([]OverrideRow, error) {
	return spaceValue(ctx, s, spaceID, func(scoped *RoleStore) ([]OverrideRow, error) {
		return scoped.unscopedListVoiceRoomOverrides(ctx, spaceID, voiceRoomID)
	})
}

// RemoveChatOverride executes within its ordinary space transaction.
func (s *RoleStore) RemoveChatOverride(ctx context.Context, chatID, roleID uuid.UUID) error {
	return s.WithinRole(ctx, roleID, func(scoped *RoleStore) error { return scoped.unscopedRemoveChatOverride(ctx, chatID, roleID) })
}

// RemoveVoiceRoomOverride executes within its ordinary space transaction.
func (s *RoleStore) RemoveVoiceRoomOverride(ctx context.Context, voiceRoomID, roleID uuid.UUID) error {
	return s.WithinRole(ctx, roleID, func(scoped *RoleStore) error { return scoped.unscopedRemoveVoiceRoomOverride(ctx, voiceRoomID, roleID) })
}

// SetDefaultJoinRole executes within its ordinary space transaction.
func (s *RoleStore) SetDefaultJoinRole(ctx context.Context, spaceID, roleID uuid.UUID) error {
	return s.WithinSpaces(ctx, []uuid.UUID{spaceID}, func(scoped *RoleStore) error { return scoped.unscopedSetDefaultJoinRole(ctx, spaceID, roleID) })
}

// GetDefaultJoinRole executes within its ordinary space transaction.
func (s *RoleStore) GetDefaultJoinRole(ctx context.Context, spaceID uuid.UUID) (*RoleRow, error) {
	return spaceValue(ctx, s, spaceID, func(scoped *RoleStore) (*RoleRow, error) { return scoped.unscopedGetDefaultJoinRole(ctx, spaceID) })
}

// RoleIDByNameRow executes within its ordinary space transaction.
func (s *RoleStore) RoleIDByNameRow(ctx context.Context, spaceID uuid.UUID, name string) (*RoleRow, error) {
	return spaceValue(ctx, s, spaceID, func(scoped *RoleStore) (*RoleRow, error) { return scoped.unscopedRoleIDByNameRow(ctx, spaceID, name) })
}

// CanEditRole executes within its ordinary space transaction.
func (s *RoleStore) CanEditRole(ctx context.Context, spaceID, actorProfileID, targetRoleID uuid.UUID) (bool, error) {
	return spaceValue(ctx, s, spaceID, func(scoped *RoleStore) (bool, error) {
		return scoped.unscopedCanEditRole(ctx, spaceID, actorProfileID, targetRoleID)
	})
}

// RoleIDByName executes within its ordinary space transaction.
func (s *RoleStore) RoleIDByName(ctx context.Context, spaceID uuid.UUID, name string) (uuid.UUID, error) {
	return spaceValue(ctx, s, spaceID, func(scoped *RoleStore) (uuid.UUID, error) { return scoped.unscopedRoleIDByName(ctx, spaceID, name) })
}
