package store

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"voice/backend/pkg/authoritysource"
)

// NativeMembershipRequired is an owner-schema decision, never a request flag.
// Historical catalogs retain their separate behavior; future/dirty schemas do
// not fall back to historical authorization.
func (s *RoleStore) NativeMembershipRequired(ctx context.Context) (bool, error) {
	if s == nil || (s.tx == nil && s.Pool == nil) {
		return false, ErrScopeUnavailable
	}
	var db scopeExecutor = s.Pool
	if s.tx != nil {
		db = s.db()
	}
	var version int64
	var dirty bool
	if err := db.QueryRow(ctx, `SELECT version,dirty FROM schema_migrations`).Scan(&version, &dirty); err != nil {
		return false, fmt.Errorf("%w: native schema: %w", ErrScopeUnavailable, err)
	}
	if dirty || version < 1 || version > 21 {
		return false, ErrScopeUnavailable
	}
	return version == 21, nil
}

// QualifiedRevision is local-only and must run inside the caller's Space fence.
// It binds the later mutation CAS to both independent Role-owned clocks.
func (s *RoleStore) QualifiedRevision(ctx context.Context, space uuid.UUID) (uint64, error) {
	if s == nil || s.tx == nil {
		return 0, ErrScopeUnavailable
	}
	if err := authoritysource.CheckCatalog(ctx, s.tx, roleAuthorityCatalog21); err != nil {
		return 0, fmt.Errorf("%w: %w", ErrScopeUnavailable, err)
	}
	role, sdk, err := roleSourceRevisions(ctx, s.tx, space.String())
	if err != nil {
		return 0, fmt.Errorf("%w: %w", ErrScopeUnavailable, err)
	}
	return role + sdk, nil
}

// GetRawMemberRoleInventory is explicitly non-authoritative migration/cleanup
// inventory. Authorization and exported membership readers use GetMemberRoles.
func (s *RoleStore) GetRawMemberRoleInventory(ctx context.Context, space, profile uuid.UUID) ([]RoleRow, error) {
	return spaceValue(ctx, s, space, func(scoped *RoleStore) ([]RoleRow, error) { return scoped.rawMemberRoleInventory(ctx, space, profile) })
}
