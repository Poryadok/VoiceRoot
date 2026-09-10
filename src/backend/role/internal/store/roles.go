package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"voice/backend/role/permissions"
)

// ErrRoleNotFound identifies a missing role without hiding scope failures.
var ErrRoleNotFound = errors.New("role not found")

// Keep the legacy lifecycle's existing sentinel identity.
var errRoleNotFound = ErrRoleNotFound

// BootstrapSystemRoles seeds Owner, Admin, Moderator, Member, Guest for a space.
func (s *RoleStore) unscopedBootstrapSystemRoles(ctx context.Context, spaceID uuid.UUID) error {
	if s == nil || s.Pool == nil {
		return errors.New("role store: pool not configured")
	}
	_, err := s.bootstrapSystemRoles(ctx, spaceID)
	return err
}

// bootstrapSystemRoles atomically seeds system roles and returns only rows it created.
func (s *RoleStore) bootstrapSystemRoles(ctx context.Context, spaceID uuid.UUID) ([]RoleRow, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("role store: pool not configured")
	}
	tx := s.tx
	created, err := s.bootstrapSystemRolesTx(ctx, tx, spaceID)
	if err != nil {
		return nil, err
	}
	return created, nil
}

func (s *RoleStore) bootstrapSystemRolesTx(ctx context.Context, tx pgx.Tx, spaceID uuid.UUID) ([]RoleRow, error) {
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, spaceID.String()); err != nil {
		return nil, err
	}
	existing, err := listRoles(ctx, tx, spaceID)
	if err != nil {
		return nil, err
	}
	if len(existing) > 0 {
		return nil, nil
	}
	specs, err := permissions.SystemRoles()
	if err != nil {
		return nil, err
	}
	created := make([]RoleRow, 0, len(specs))
	for _, spec := range specs {
		defaultJoin := spec.Name == permissions.RoleMember
		var id uuid.UUID
		var createdAt time.Time
		err := tx.QueryRow(ctx, `
INSERT INTO roles (space_id, name, is_system, position, permissions, is_default_join)
VALUES ($1, $2, true, $3, $4, $5)

RETURNING id, created_at
`, spaceID, spec.Name, spec.Position, int64(spec.Mask), defaultJoin).Scan(&id, &createdAt)
		if err != nil {
			return nil, fmt.Errorf("insert system role %q: %w", spec.Name, err)
		}
		created = append(created, RoleRow{
			ID:              id,
			SpaceID:         spaceID,
			Name:            spec.Name,
			PermissionsMask: spec.Mask,
			Position:        spec.Position,
			Managed:         true,
			CreatedAt:       createdAt,
		})
	}
	return created, nil
}

// BootstrapSpaceRoles seeds roles and assigns Owner to ownerProfileID.
func (s *RoleStore) unscopedBootstrapSpaceRoles(ctx context.Context, spaceID, ownerProfileID uuid.UUID) error {
	_, err := s.BootstrapSpaceRolesWithCreatedSystemRoles(ctx, spaceID, ownerProfileID)
	return err
}

// BootstrapSpaceRolesWithCreatedSystemRoles seeds roles, assigns Owner, and returns only newly created system roles.
func (s *RoleStore) unscopedBootstrapSpaceRolesWithCreatedSystemRoles(ctx context.Context, spaceID, ownerProfileID uuid.UUID) ([]RoleRow, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("role store: pool not configured")
	}
	tx := s.tx
	created, err := s.bootstrapSystemRolesTx(ctx, tx, spaceID)
	if err != nil {
		return nil, err
	}
	roles, err := listRoles(ctx, tx, spaceID)
	if err != nil {
		return nil, err
	}
	for _, r := range roles {
		if r.Name == permissions.RoleOwner {
			rows, err := tx.Query(ctx, "SELECT space_id, profile_id FROM member_roles WHERE role_id=$1 FOR UPDATE", r.ID)
			if err != nil {
				return nil, err
			}
			ownerCount := 0
			matchingOwner := false
			for rows.Next() {
				var assignedSpace, assignedOwner uuid.UUID
				if err := rows.Scan(&assignedSpace, &assignedOwner); err != nil {
					rows.Close()
					return nil, err
				}
				ownerCount++
				matchingOwner = assignedSpace == spaceID && assignedOwner == ownerProfileID
			}
			rows.Close()
			if err := rows.Err(); err != nil {
				return nil, err
			}
			if ownerCount > 1 || (ownerCount == 1 && !matchingOwner) {
				return nil, errors.New("bootstrap cannot change existing Owner")
			}
			if ownerCount == 1 {
				return created, nil
			}
			if _, err := tx.Exec(ctx, `
INSERT INTO member_roles (space_id, profile_id, role_id, assigned_by)
VALUES ($1, $2, $3, $4)
ON CONFLICT (space_id, profile_id, role_id) DO NOTHING
`, spaceID, ownerProfileID, r.ID, ownerProfileID); err != nil {
				return nil, err
			}
			return created, nil
		}
	}
	return nil, ErrRoleNotFound
}

func scanRoleRow(row pgx.Row) (RoleRow, error) {
	var r RoleRow
	var perms int64
	var isSystem bool
	var createdBy *uuid.UUID
	err := row.Scan(&r.ID, &r.SpaceID, &r.Name, &isSystem, &r.Position, &perms, &createdBy, &r.CreatedAt)
	if err != nil {
		return RoleRow{}, err
	}
	r.PermissionsMask = uint64(perms)
	r.Managed = isSystem
	r.CreatedByProfileID = createdBy
	return r, nil
}

type roleQueryer interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func listRoles(ctx context.Context, queryer roleQueryer, spaceID uuid.UUID) ([]RoleRow, error) {
	rows, err := queryer.Query(ctx, `
SELECT id, space_id, name, is_system, position, permissions, created_by_profile_id, created_at
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

// ListRoles returns roles for a space ordered by position descending.
func (s *RoleStore) unscopedListRoles(ctx context.Context, spaceID uuid.UUID) ([]RoleRow, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("role store: pool not configured")
	}
	return listRoles(ctx, s.db(), spaceID)
}

// GetRoleByID loads a single role.
func (s *RoleStore) unscopedGetRoleByID(ctx context.Context, roleID uuid.UUID) (*RoleRow, error) {
	row := s.db().QueryRow(ctx, `
SELECT id, space_id, name, is_system, position, permissions, created_by_profile_id, created_at
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
func (s *RoleStore) unscopedAssignMemberRole(ctx context.Context, spaceID, profileID, roleID, assignedBy uuid.UUID) error {
	if s == nil || s.Pool == nil {
		return errors.New("role store: pool not configured")
	}
	role, err := s.GetRoleByID(ctx, roleID)
	if err != nil {
		return err
	}
	if role == nil || role.SpaceID != spaceID {
		return ErrRoleNotFound
	}
	_, err = s.db().Exec(ctx, `
INSERT INTO member_roles (space_id, profile_id, role_id, assigned_by)
VALUES ($1, $2, $3, $4)
ON CONFLICT (space_id, profile_id, role_id) DO NOTHING
`, spaceID, profileID, roleID, assignedBy)
	return err
}

// RevokeMemberRole removes a role assignment.
func (s *RoleStore) unscopedRevokeMemberRole(ctx context.Context, spaceID, profileID, roleID uuid.UUID) error {
	role, err := s.GetRoleByID(ctx, roleID)
	if err != nil {
		return err
	}
	if role == nil || role.SpaceID != spaceID {
		return ErrRoleNotFound
	}
	_, err = s.db().Exec(ctx, `
DELETE FROM member_roles WHERE space_id = $1 AND profile_id = $2 AND role_id = $3
`, spaceID, profileID, roleID)
	return err
}

// GetMemberRoles returns roles assigned to a member.
func (s *RoleStore) unscopedGetMemberRoles(ctx context.Context, spaceID, profileID uuid.UUID) ([]RoleRow, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("role store: pool not configured")
	}
	rows, err := s.db().Query(ctx, `
SELECT r.id, r.space_id, r.name, r.is_system, r.position, r.permissions, r.created_by_profile_id, r.created_at
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
		if r.SpaceID != spaceID {
			return nil, fmt.Errorf("%w: member role belongs to another space", ErrScopeUnavailable)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// GetEffectiveMask computes effective permissions per role-service.md algorithm.
func (s *RoleStore) unscopedGetEffectiveMask(ctx context.Context, spaceID, profileID uuid.UUID, chatID, voiceRoomID *uuid.UUID) (uint64, error) {
	roles, err := s.GetMemberRoles(ctx, spaceID, profileID)
	if err != nil {
		return 0, err
	}
	if len(roles) == 0 {
		return 0, nil
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
		err := s.db().QueryRow(ctx, `
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
		err := s.db().QueryRow(ctx, `
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
func (s *RoleStore) unscopedCanManageRole(ctx context.Context, spaceID, actorProfileID, targetRoleID uuid.UUID) (bool, error) {
	actorRoles, err := s.GetMemberRoles(ctx, spaceID, actorProfileID)
	if err != nil {
		return false, err
	}
	target, err := s.GetRoleByID(ctx, targetRoleID)
	if err != nil || target == nil || target.SpaceID != spaceID {
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

// CanCreateRole reports whether actor may create a role at position under their hierarchy.
// Permission-bit authorization is performed by the gRPC caller.
func (s *RoleStore) unscopedCanCreateRole(ctx context.Context, spaceID, actorProfileID uuid.UUID, position int32) (bool, error) {
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
	return actorTop > position, nil
}

// SetChatOverride upserts chat_overrides for role_id (all member roles if roleID is Nil — not used).
func (s *RoleStore) unscopedSetChatOverride(ctx context.Context, chatID, roleID uuid.UUID, allow, deny uint64) error {
	_, err := s.db().Exec(ctx, `
INSERT INTO chat_overrides (chat_id, role_id, allow, deny)
VALUES ($1, $2, $3, $4)
ON CONFLICT (chat_id, role_id) DO UPDATE SET allow = EXCLUDED.allow, deny = EXCLUDED.deny
`, chatID, roleID, int64(allow), int64(deny))
	return err
}

// SetChatOverrideForMemberRoles sets deny/allow for each role the profile holds in the chat scope.
func (s *RoleStore) unscopedSetChatOverrideForMemberRoles(ctx context.Context, spaceID uuid.UUID, chatID uuid.UUID, profileID uuid.UUID, allow, deny uint64) error {
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
func (s *RoleStore) unscopedSetVoiceRoomOverride(ctx context.Context, voiceRoomID, roleID uuid.UUID, allow, deny uint64) error {
	_, err := s.db().Exec(ctx, `
INSERT INTO voice_room_overrides (voice_room_id, role_id, allow, deny)
VALUES ($1, $2, $3, $4)
ON CONFLICT (voice_room_id, role_id) DO UPDATE SET allow = EXCLUDED.allow, deny = EXCLUDED.deny
`, voiceRoomID, roleID, int64(allow), int64(deny))
	return err
}

// CreateCustomRole inserts a non-system role.
func (s *RoleStore) unscopedCreateCustomRole(ctx context.Context, spaceID uuid.UUID, name string, permissionsMask uint64, position int32, createdByProfileID *uuid.UUID) (*RoleRow, error) {
	var id uuid.UUID
	var err error
	if createdByProfileID != nil && *createdByProfileID != uuid.Nil {
		err = s.db().QueryRow(ctx, `
INSERT INTO roles (space_id, name, is_system, position, permissions, created_by_profile_id)
VALUES ($1, $2, false, $3, $4, $5)
RETURNING id
`, spaceID, name, position, int64(permissionsMask), *createdByProfileID).Scan(&id)
	} else {
		err = s.db().QueryRow(ctx, `
INSERT INTO roles (space_id, name, is_system, position, permissions)
VALUES ($1, $2, false, $3, $4)
RETURNING id
`, spaceID, name, position, int64(permissionsMask)).Scan(&id)
	}
	if err != nil {
		return nil, err
	}
	return s.GetRoleByID(ctx, id)
}

// UpdateRole updates name, mask, or position.
func (s *RoleStore) unscopedUpdateRole(ctx context.Context, roleID uuid.UUID, name *string, permissionsMask *uint64, position *int32) (*RoleRow, error) {
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
	_, err = s.db().Exec(ctx, `
UPDATE roles SET name = $2, permissions = $3, position = $4, updated_at = now()
WHERE id = $1
`, roleID, n, int64(mask), pos)
	if err != nil {
		return nil, err
	}
	return s.GetRoleByID(ctx, roleID)
}

// DeleteRolesCreatedByProfile removes non-system roles created by profileID in spaceID.
func (s *RoleStore) unscopedDeleteRolesCreatedByProfile(ctx context.Context, spaceID, profileID uuid.UUID) (int64, error) {
	if s == nil || s.Pool == nil {
		return 0, errors.New("role store: pool not configured")
	}
	tag, err := s.db().Exec(ctx, `
DELETE FROM roles
WHERE space_id = $1 AND created_by_profile_id = $2 AND is_system = false
`, spaceID, profileID)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// DeleteRole removes a non-system role.
func (s *RoleStore) unscopedDeleteRole(ctx context.Context, roleID uuid.UUID) error {
	row, err := s.GetRoleByID(ctx, roleID)
	if err != nil {
		return err
	}
	if row == nil {
		return ErrRoleNotFound
	}
	if row.Managed {
		return errors.New("cannot delete managed system role")
	}
	_, err = s.db().Exec(ctx, `DELETE FROM roles WHERE id = $1`, roleID)
	return err
}

// ReorderRoles updates role positions from ordered_role_ids (highest position first).
func (s *RoleStore) unscopedReorderRoles(ctx context.Context, spaceID uuid.UUID, orderedRoleIDs []uuid.UUID) error {
	if s == nil || s.Pool == nil {
		return errors.New("role store: pool not configured")
	}
	tx := s.tx
	pos := int32(len(orderedRoleIDs))
	for _, id := range orderedRoleIDs {
		role, err := s.GetRoleByID(ctx, id)
		if err != nil {
			return err
		}
		if role == nil || role.SpaceID != spaceID {
			return ErrRoleNotFound
		}
		pos--
		_, err = tx.Exec(ctx, `
UPDATE roles SET position = $3, updated_at = now()
WHERE id = $1 AND space_id = $2
`, id, spaceID, pos)
		if err != nil {
			return err
		}
	}
	return nil
}

// ListChatOverrides returns overrides for a space, optionally filtered by chat_id.
func (s *RoleStore) unscopedListChatOverrides(ctx context.Context, spaceID uuid.UUID, chatID *uuid.UUID) ([]OverrideRow, error) {
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
	rows, err := s.db().Query(ctx, query, args...)
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
func (s *RoleStore) unscopedListVoiceRoomOverrides(ctx context.Context, spaceID uuid.UUID, voiceRoomID *uuid.UUID) ([]OverrideRow, error) {
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
	rows, err := s.db().Query(ctx, query, args...)
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
func (s *RoleStore) unscopedRemoveChatOverride(ctx context.Context, chatID, roleID uuid.UUID) error {
	_, err := s.db().Exec(ctx, `
DELETE FROM chat_overrides WHERE chat_id = $1 AND role_id = $2
`, chatID, roleID)
	return err
}

// RemoveVoiceRoomOverride deletes a voice room override row.
func (s *RoleStore) unscopedRemoveVoiceRoomOverride(ctx context.Context, voiceRoomID, roleID uuid.UUID) error {
	_, err := s.db().Exec(ctx, `
DELETE FROM voice_room_overrides WHERE voice_room_id = $1 AND role_id = $2
`, voiceRoomID, roleID)
	return err
}

// SetDefaultJoinRole marks role_id as the default join role for space_id.
func (s *RoleStore) unscopedSetDefaultJoinRole(ctx context.Context, spaceID, roleID uuid.UUID) error {
	row, err := s.GetRoleByID(ctx, roleID)
	if err != nil {
		return err
	}
	if row == nil || row.SpaceID != spaceID {
		return ErrRoleNotFound
	}
	tx := s.tx
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
	return nil
}

// GetDefaultJoinRole returns the configured default join role for a space.
func (s *RoleStore) unscopedGetDefaultJoinRole(ctx context.Context, spaceID uuid.UUID) (*RoleRow, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("role store: pool not configured")
	}
	row := s.db().QueryRow(ctx, `
SELECT id, space_id, name, is_system, position, permissions, created_by_profile_id, created_at
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
func (s *RoleStore) unscopedRoleIDByNameRow(ctx context.Context, spaceID uuid.UUID, name string) (*RoleRow, error) {
	id, err := s.RoleIDByName(ctx, spaceID, name)
	if err != nil {
		return nil, err
	}
	return s.GetRoleByID(ctx, id)
}

// CanEditRole reports whether actor may update/delete/reorder target role definition.
func (s *RoleStore) unscopedCanEditRole(ctx context.Context, spaceID, actorProfileID, targetRoleID uuid.UUID) (bool, error) {
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
func (s *RoleStore) unscopedRoleIDByName(ctx context.Context, spaceID uuid.UUID, name string) (uuid.UUID, error) {
	var id uuid.UUID
	err := s.db().QueryRow(ctx, `
SELECT id FROM roles WHERE space_id = $1 AND name = $2
`, spaceID, name).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, ErrRoleNotFound
	}
	return id, err
}
