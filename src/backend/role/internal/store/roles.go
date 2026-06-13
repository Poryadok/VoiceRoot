package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"voice/backend/role/permissions"
)

var errRoleNotFound = errors.New("role not found")

// BootstrapSystemRoles seeds Owner, Admin, Moderator, Member, Guest for a space.
func (s *RoleStore) BootstrapSystemRoles(ctx context.Context, spaceID uuid.UUID) error {
	if s == nil || s.Pool == nil {
		return errors.New("role store: pool not configured")
	}
	existing, err := s.ListRoles(ctx, spaceID)
	if err != nil {
		return err
	}
	if len(existing) > 0 {
		return nil
	}
	specs, err := permissions.SystemRoles()
	if err != nil {
		return err
	}
	for _, spec := range specs {
		defaultJoin := spec.Name == permissions.RoleMember
		_, err := s.Pool.Exec(ctx, `
INSERT INTO roles (space_id, name, is_system, position, permissions, is_default_join)
VALUES ($1, $2, true, $3, $4, $5)
`, spaceID, spec.Name, spec.Position, int64(spec.Mask), defaultJoin)
		if err != nil {
			return fmt.Errorf("insert system role %q: %w", spec.Name, err)
		}
	}
	return nil
}

// BootstrapSpaceRoles seeds roles and assigns Owner to ownerProfileID.
func (s *RoleStore) BootstrapSpaceRoles(ctx context.Context, spaceID, ownerProfileID uuid.UUID) error {
	if err := s.BootstrapSystemRoles(ctx, spaceID); err != nil {
		return err
	}
	roles, err := s.ListRoles(ctx, spaceID)
	if err != nil {
		return err
	}
	for _, r := range roles {
		if r.Name == permissions.RoleOwner {
			return s.AssignMemberRole(ctx, spaceID, ownerProfileID, r.ID, ownerProfileID)
		}
	}
	return errRoleNotFound
}

func scanRoleRow(row pgx.Row) (RoleRow, error) {
	var r RoleRow
	var perms int64
	var isSystem bool
	err := row.Scan(&r.ID, &r.SpaceID, &r.Name, &isSystem, &r.Position, &perms)
	if err != nil {
		return RoleRow{}, err
	}
	r.PermissionsMask = uint64(perms)
	r.Managed = isSystem
	return r, nil
}

// ListRoles returns roles for a space ordered by position descending.
func (s *RoleStore) ListRoles(ctx context.Context, spaceID uuid.UUID) ([]RoleRow, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("role store: pool not configured")
	}
	rows, err := s.Pool.Query(ctx, `
SELECT id, space_id, name, is_system, position, permissions
FROM roles
WHERE space_id = $1
ORDER BY position DESC, name ASC
`, spaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RoleRow
	for rows.Next() {
		r, err := scanRoleRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// GetRoleByID loads a single role.
func (s *RoleStore) GetRoleByID(ctx context.Context, roleID uuid.UUID) (*RoleRow, error) {
	row := s.Pool.QueryRow(ctx, `
SELECT id, space_id, name, is_system, position, permissions
FROM roles WHERE id = $1
`, roleID)
	r, err := scanRoleRow(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// AssignMemberRole assigns role_id to profile_id within space_id.
func (s *RoleStore) AssignMemberRole(ctx context.Context, spaceID, profileID, roleID, assignedBy uuid.UUID) error {
	if s == nil || s.Pool == nil {
		return errors.New("role store: pool not configured")
	}
	_, err := s.Pool.Exec(ctx, `
INSERT INTO member_roles (space_id, profile_id, role_id, assigned_by)
VALUES ($1, $2, $3, $4)
ON CONFLICT (space_id, profile_id, role_id) DO NOTHING
`, spaceID, profileID, roleID, assignedBy)
	return err
}

// RevokeMemberRole removes a role assignment.
func (s *RoleStore) RevokeMemberRole(ctx context.Context, spaceID, profileID, roleID uuid.UUID) error {
	_, err := s.Pool.Exec(ctx, `
DELETE FROM member_roles WHERE space_id = $1 AND profile_id = $2 AND role_id = $3
`, spaceID, profileID, roleID)
	return err
}

// GetMemberRoles returns roles assigned to a member.
func (s *RoleStore) GetMemberRoles(ctx context.Context, spaceID, profileID uuid.UUID) ([]RoleRow, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("role store: pool not configured")
	}
	rows, err := s.Pool.Query(ctx, `
SELECT r.id, r.space_id, r.name, r.is_system, r.position, r.permissions
FROM member_roles mr
JOIN roles r ON r.id = mr.role_id
WHERE mr.space_id = $1 AND mr.profile_id = $2
ORDER BY r.position DESC
`, spaceID, profileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RoleRow
	for rows.Next() {
		r, err := scanRoleRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// GetEffectiveMask computes effective permissions per role-service.md algorithm.
func (s *RoleStore) GetEffectiveMask(ctx context.Context, spaceID, profileID uuid.UUID, chatID, voiceRoomID *uuid.UUID) (uint64, error) {
	roles, err := s.GetMemberRoles(ctx, spaceID, profileID)
	if err != nil {
		return 0, err
	}
	all, err := permissions.AllMask()
	if err != nil {
		return 0, err
	}
	for _, r := range roles {
		if r.Name == permissions.RoleOwner {
			return all, nil
		}
	}
	for _, r := range roles {
		if r.Name == permissions.RoleAdmin {
			return all, nil
		}
	}
	var mask uint64
	if defaultRole, err := s.GetDefaultJoinRole(ctx, spaceID); err != nil {
		return 0, err
	} else if defaultRole != nil {
		mask = defaultRole.PermissionsMask
	}
	for _, r := range roles {
		mask |= r.PermissionsMask
	}
	if chatID != nil {
		override, err := s.getChatOverrideMask(ctx, *chatID, roles)
		if err != nil {
			return 0, err
		}
		mask |= override.allow
		mask &^= override.deny
	}
	if voiceRoomID != nil {
		override, err := s.getVoiceOverrideMask(ctx, *voiceRoomID, roles)
		if err != nil {
			return 0, err
		}
		mask |= override.allow
		mask &^= override.deny
	}
	return mask, nil
}

type overrideMask struct {
	allow uint64
	deny  uint64
}

func (s *RoleStore) getChatOverrideMask(ctx context.Context, chatID uuid.UUID, roles []RoleRow) (overrideMask, error) {
	var out overrideMask
	for _, r := range roles {
		var allow, deny int64
		err := s.Pool.QueryRow(ctx, `
SELECT allow, deny FROM chat_overrides WHERE chat_id = $1 AND role_id = $2
`, chatID, r.ID).Scan(&allow, &deny)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return out, err
		}
		out.allow |= uint64(allow)
		out.deny |= uint64(deny)
	}
	return out, nil
}

func (s *RoleStore) getVoiceOverrideMask(ctx context.Context, voiceRoomID uuid.UUID, roles []RoleRow) (overrideMask, error) {
	var out overrideMask
	for _, r := range roles {
		var allow, deny int64
		err := s.Pool.QueryRow(ctx, `
SELECT allow, deny FROM voice_room_overrides WHERE voice_room_id = $1 AND role_id = $2
`, voiceRoomID, r.ID).Scan(&allow, &deny)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return out, err
		}
		out.allow |= uint64(allow)
		out.deny |= uint64(deny)
	}
	return out, nil
}

// CanManageRole reports whether actor may assign/revoke target role (hierarchy).
func (s *RoleStore) CanManageRole(ctx context.Context, spaceID, actorProfileID, targetRoleID uuid.UUID) (bool, error) {
	actorRoles, err := s.GetMemberRoles(ctx, spaceID, actorProfileID)
	if err != nil {
		return false, err
	}
	target, err := s.GetRoleByID(ctx, targetRoleID)
	if err != nil || target == nil {
		return false, err
	}
	assignMask, err := permissions.MaskFor(permissions.MemberAssignRoles)
	if err != nil {
		return false, err
	}
	var actorTop int32 = -1
	for _, ar := range actorRoles {
		if ar.Name == permissions.RoleOwner {
			return true, nil
		}
		if ar.Position > actorTop {
			actorTop = ar.Position
		}
	}
	if actorTop < 0 {
		return false, nil
	}
	eff, err := s.GetEffectiveMask(ctx, spaceID, actorProfileID, nil, nil)
	if err != nil {
		return false, err
	}
	if eff&assignMask == 0 {
		return false, nil
	}
	return actorTop > target.Position, nil
}

// SetChatOverride upserts chat_overrides for role_id (all member roles if roleID is Nil — not used).
func (s *RoleStore) SetChatOverride(ctx context.Context, chatID, roleID uuid.UUID, allow, deny uint64) error {
	_, err := s.Pool.Exec(ctx, `
INSERT INTO chat_overrides (chat_id, role_id, allow, deny)
VALUES ($1, $2, $3, $4)
ON CONFLICT (chat_id, role_id) DO UPDATE SET allow = EXCLUDED.allow, deny = EXCLUDED.deny
`, chatID, roleID, int64(allow), int64(deny))
	return err
}

// SetChatOverrideForMemberRoles sets deny/allow for each role the profile holds in the chat scope.
func (s *RoleStore) SetChatOverrideForMemberRoles(ctx context.Context, spaceID uuid.UUID, chatID uuid.UUID, profileID uuid.UUID, allow, deny uint64) error {
	roles, err := s.GetMemberRoles(ctx, spaceID, profileID)
	if err != nil {
		return err
	}
	for _, r := range roles {
		if err := s.SetChatOverride(ctx, chatID, r.ID, allow, deny); err != nil {
			return err
		}
	}
	return nil
}

// SetVoiceRoomOverride upserts voice_room_overrides.
func (s *RoleStore) SetVoiceRoomOverride(ctx context.Context, voiceRoomID, roleID uuid.UUID, allow, deny uint64) error {
	_, err := s.Pool.Exec(ctx, `
INSERT INTO voice_room_overrides (voice_room_id, role_id, allow, deny)
VALUES ($1, $2, $3, $4)
ON CONFLICT (voice_room_id, role_id) DO UPDATE SET allow = EXCLUDED.allow, deny = EXCLUDED.deny
`, voiceRoomID, roleID, int64(allow), int64(deny))
	return err
}

// CreateCustomRole inserts a non-system role.
func (s *RoleStore) CreateCustomRole(ctx context.Context, spaceID uuid.UUID, name string, permissionsMask uint64, position int32) (*RoleRow, error) {
	var id uuid.UUID
	err := s.Pool.QueryRow(ctx, `
INSERT INTO roles (space_id, name, is_system, position, permissions)
VALUES ($1, $2, false, $3, $4)
RETURNING id
`, spaceID, name, position, int64(permissionsMask)).Scan(&id)
	if err != nil {
		return nil, err
	}
	return s.GetRoleByID(ctx, id)
}

// UpdateRole updates name, mask, or position.
func (s *RoleStore) UpdateRole(ctx context.Context, roleID uuid.UUID, name *string, permissionsMask *uint64, position *int32) (*RoleRow, error) {
	row, err := s.GetRoleByID(ctx, roleID)
	if err != nil || row == nil {
		return nil, err
	}
	if row.Managed {
		return nil, errors.New("cannot update managed system role")
	}
	n := row.Name
	mask := row.PermissionsMask
	pos := row.Position
	if name != nil {
		n = *name
	}
	if permissionsMask != nil {
		mask = *permissionsMask
	}
	if position != nil {
		pos = *position
	}
	_, err = s.Pool.Exec(ctx, `
UPDATE roles SET name = $2, permissions = $3, position = $4, updated_at = now()
WHERE id = $1
`, roleID, n, int64(mask), pos)
	if err != nil {
		return nil, err
	}
	return s.GetRoleByID(ctx, roleID)
}

// DeleteRole removes a non-system role.
func (s *RoleStore) DeleteRole(ctx context.Context, roleID uuid.UUID) error {
	row, err := s.GetRoleByID(ctx, roleID)
	if err != nil || row == nil {
		return errRoleNotFound
	}
	if row.Managed {
		return errors.New("cannot delete managed system role")
	}
	_, err = s.Pool.Exec(ctx, `DELETE FROM roles WHERE id = $1`, roleID)
	return err
}

// ReorderRoles updates role positions from ordered_role_ids (highest position first).
func (s *RoleStore) ReorderRoles(ctx context.Context, spaceID uuid.UUID, orderedRoleIDs []uuid.UUID) error {
	if s == nil || s.Pool == nil {
		return errors.New("role store: pool not configured")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	pos := int32(len(orderedRoleIDs))
	for _, id := range orderedRoleIDs {
		pos--
		_, err := tx.Exec(ctx, `
UPDATE roles SET position = $3, updated_at = now()
WHERE id = $1 AND space_id = $2
`, id, spaceID, pos)
		if err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// ListChatOverrides returns overrides for a space, optionally filtered by chat_id.
func (s *RoleStore) ListChatOverrides(ctx context.Context, spaceID uuid.UUID, chatID *uuid.UUID) ([]OverrideRow, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("role store: pool not configured")
	}
	query := `
SELECT co.chat_id, co.role_id, r.name, co.allow, co.deny
FROM chat_overrides co
JOIN roles r ON r.id = co.role_id
WHERE r.space_id = $1
`
	args := []any{spaceID}
	if chatID != nil {
		query += ` AND co.chat_id = $2`
		args = append(args, *chatID)
	}
	rows, err := s.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []OverrideRow
	for rows.Next() {
		var row OverrideRow
		var allow, deny int64
		if err := rows.Scan(&row.ChatID, &row.RoleID, &row.RoleName, &allow, &deny); err != nil {
			return nil, err
		}
		row.Allow = uint64(allow)
		row.Deny = uint64(deny)
		out = append(out, row)
	}
	return out, rows.Err()
}

// ListVoiceRoomOverrides returns overrides for a space, optionally filtered by voice_room_id.
func (s *RoleStore) ListVoiceRoomOverrides(ctx context.Context, spaceID uuid.UUID, voiceRoomID *uuid.UUID) ([]OverrideRow, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("role store: pool not configured")
	}
	query := `
SELECT vo.voice_room_id, vo.role_id, r.name, vo.allow, vo.deny
FROM voice_room_overrides vo
JOIN roles r ON r.id = vo.role_id
WHERE r.space_id = $1
`
	args := []any{spaceID}
	if voiceRoomID != nil {
		query += ` AND vo.voice_room_id = $2`
		args = append(args, *voiceRoomID)
	}
	rows, err := s.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []OverrideRow
	for rows.Next() {
		var row OverrideRow
		var allow, deny int64
		if err := rows.Scan(&row.VoiceRoomID, &row.RoleID, &row.RoleName, &allow, &deny); err != nil {
			return nil, err
		}
		row.Allow = uint64(allow)
		row.Deny = uint64(deny)
		out = append(out, row)
	}
	return out, rows.Err()
}

// RemoveChatOverride deletes a chat override row.
func (s *RoleStore) RemoveChatOverride(ctx context.Context, chatID, roleID uuid.UUID) error {
	_, err := s.Pool.Exec(ctx, `
DELETE FROM chat_overrides WHERE chat_id = $1 AND role_id = $2
`, chatID, roleID)
	return err
}

// RemoveVoiceRoomOverride deletes a voice room override row.
func (s *RoleStore) RemoveVoiceRoomOverride(ctx context.Context, voiceRoomID, roleID uuid.UUID) error {
	_, err := s.Pool.Exec(ctx, `
DELETE FROM voice_room_overrides WHERE voice_room_id = $1 AND role_id = $2
`, voiceRoomID, roleID)
	return err
}

// SetDefaultJoinRole marks role_id as the default join role for space_id.
func (s *RoleStore) SetDefaultJoinRole(ctx context.Context, spaceID, roleID uuid.UUID) error {
	row, err := s.GetRoleByID(ctx, roleID)
	if err != nil || row == nil || row.SpaceID != spaceID {
		return errRoleNotFound
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `
UPDATE roles SET is_default_join = false, updated_at = now() WHERE space_id = $1
`, spaceID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
UPDATE roles SET is_default_join = true, updated_at = now() WHERE id = $1 AND space_id = $2
`, roleID, spaceID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// GetDefaultJoinRole returns the configured default join role for a space.
func (s *RoleStore) GetDefaultJoinRole(ctx context.Context, spaceID uuid.UUID) (*RoleRow, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("role store: pool not configured")
	}
	row := s.Pool.QueryRow(ctx, `
SELECT id, space_id, name, is_system, position, permissions
FROM roles WHERE space_id = $1 AND is_default_join = true
LIMIT 1
`, spaceID)
	r, err := scanRoleRow(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return s.RoleIDByNameRow(ctx, spaceID, permissions.RoleMember)
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// RoleIDByNameRow loads a role row by name in a space.
func (s *RoleStore) RoleIDByNameRow(ctx context.Context, spaceID uuid.UUID, name string) (*RoleRow, error) {
	id, err := s.RoleIDByName(ctx, spaceID, name)
	if err != nil {
		return nil, err
	}
	return s.GetRoleByID(ctx, id)
}

// CanEditRole reports whether actor may update/delete/reorder target role definition.
func (s *RoleStore) CanEditRole(ctx context.Context, spaceID, actorProfileID, targetRoleID uuid.UUID) (bool, error) {
	target, err := s.GetRoleByID(ctx, targetRoleID)
	if err != nil || target == nil || target.SpaceID != spaceID {
		return false, err
	}
	if target.Managed {
		return false, nil
	}
	manageMask, err := permissions.MaskFor(permissions.SpaceManageRoles)
	if err != nil {
		return false, err
	}
	eff, err := s.GetEffectiveMask(ctx, spaceID, actorProfileID, nil, nil)
	if err != nil {
		return false, err
	}
	if eff&manageMask == 0 {
		return false, nil
	}
	actorRoles, err := s.GetMemberRoles(ctx, spaceID, actorProfileID)
	if err != nil {
		return false, err
	}
	var actorTop int32 = -1
	for _, ar := range actorRoles {
		if ar.Name == permissions.RoleOwner {
			return true, nil
		}
		if ar.Position > actorTop {
			actorTop = ar.Position
		}
	}
	return actorTop > target.Position, nil
}

// RoleIDByName finds a system role id by name in a space.
func (s *RoleStore) RoleIDByName(ctx context.Context, spaceID uuid.UUID, name string) (uuid.UUID, error) {
	var id uuid.UUID
	err := s.Pool.QueryRow(ctx, `
SELECT id FROM roles WHERE space_id = $1 AND name = $2
`, spaceID, name).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, errRoleNotFound
	}
	return id, err
}
